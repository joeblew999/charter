// Runs a Go Worker (the Wasm in build/), keeping Go runtimes alive between
// requests.
//
// This file and the others in this folder ship with the Go library and are not copied into a
// project: charter build writes them into the project's build/ folder, beside the Wasm and
// workers-go's wasm_exec.js and runtime.mjs, from the version of the library the project builds
// against. So the JavaScript here and the Go it talks to (transport, hub) always match, and the
// imports below are of files in build/.
//
// workers-go's own entry (build/worker.mjs) starts a Go runtime for every request and drops it
// afterwards: every request pays for Go's start-up, and the engine then has the runtime's whole
// memory to collect. Here a runtime that has finished a request waits for the next one.
//
//   - A runtime serves one request at a time. A request that finds none waiting starts its own,
//     exactly as workers-go does, so nothing waits for another request.
//   - A runtime is reused only when its response ended normally: Go wrote it to its end. After an
//     error, or when the client went away, it is dropped.
//   - A runtime is dropped before Go's collector would run in it: Go says when its heap has no
//     room for another request (transport.Serve sets the binding's "full"). So a runtime serves
//     some dozens of requests, not thousands, and what must be shared between requests still goes
//     through a binding.
//
// The Go side must stay alive after a response: transport.Run instead of workers.Serve.
//
// A project's entry (worker.mjs) is then:
//
//	import { goWorker } from "./build/go.mjs";
//	const go = goWorker();
//	await go.warm({ paths: ["/api/hello"] });
//	export default { fetch: go.fetch, scheduled: go.scheduled, queue: go.queue };
import "./wasm_exec.js"; // the compiler's (workers-go writes it), for its global Go class
import * as build from "./runtime.mjs";
import { webSocket } from "./websocket.mjs";

// request.cf's fields transport.CF has (cf.go), under the header name it reads (lower case, as
// Headers gives names). A client's own header of that name is dropped.
const CF_HEADER = "x-charter-cf";
const CF_FIELDS = ["country", "region", "regionCode", "city", "postalCode", "continent", "timezone", "latitude", "longitude", "colo", "asn", "isEUCountry"];
function cfFields(cf) {
	const picked = {};
	for (const name of CF_FIELDS) if (cf[name] !== undefined) picked[name] = cf[name];
	return picked;
}

const MAX_WAITING = 4; // runtimes kept for the next request; each holds its Go heap (8 MB)

// workers-go's Go code calls this.
globalThis.tryCatch = fn => {
	try {
		return { result: fn() };
	} catch (error) {
		return { error };
	}
};

// goWorker is a Worker (its fetch) over the workers-go build beside this file (runtime.mjs).
export function goWorker({ createRuntimeContext, loadModule } = build) {
	const waiting = [];
	let workersGo; // whether the Go program serves with workers.Serve (true) or charter's transport.Run (false)
	let warmFailure = ""; // why no runtime was started while the module loaded, for the x-go-runtime header

	// begin starts a runtime. Go's start-up runs inside go.run, up to workers.Ready(): started says
	// whether it got there before go.run returned, isReady resolves when it does.
	function begin(module, env, ctx, warm) {
		const go = new Go();
		const binding = warm ? { warm } : {};
		const context = createRuntimeContext({ env, ctx, binding });
		const runtime = { go, binding, context, started: false, served: 0, warm: Boolean(warm) };
		runtime.isReady = new Promise(resolve => {
			// The runtime's seed is the first random bytes it takes, before any of the program's code runs (the scheduler
			// starts before package initialisers), through the same import as crypto/rand. While the module loads,
			// Cloudflare gives no crypto randomness: that first call gets Math.random's; a second call there is the
			// program wanting real random bytes, which fails the warm start.
			const gojs = go.importObject.gojs;
			if (gojs && warm) {
				const random = gojs["runtime.getRandomData"];
				let seeded = false;
				gojs["runtime.getRandomData"] = sp => {
					if (runtime.started) return random(sp);
					if (seeded) throw new Error("random bytes wanted during start-up, other than the runtime's seed");
					seeded = true;
					sp >>>= 0;
					const view = new DataView(go._inst.exports.mem.buffer);
					const int64 = at => view.getUint32(at, true) + view.getInt32(at + 4, true) * 4294967296;
					new Uint8Array(go._inst.exports.mem.buffer, int64(sp + 8), int64(sp + 16)).forEach((_, i, bytes) => (bytes[i] = Math.random() * 256));
				};
				// Its clock: while the module loads Cloudflare's reads zero, and the runtime stops at a clock that does
				// ("nanotime returning zero"). It gets one nanosecond there; the real time comes with the first request.
				const nanotime = gojs["runtime.nanotime1"];
				gojs["runtime.nanotime1"] = sp => {
					nanotime(sp);
					if (runtime.started) return;
					sp >>>= 0;
					const view = new DataView(go._inst.exports.mem.buffer);
					if (view.getUint32(sp + 8, true) === 0 && view.getUint32(sp + 12, true) === 0) view.setUint32(sp + 8, 1, true);
				};
			}
			const instance = new WebAssembly.Instance(module, {
				...go.importObject,
				workers: {
					ready: () => {
						runtime.started = true;
						resolve();
					},
				},
			});
			go.run(instance, context).catch(error => {
				if (warm) warmFailure = `start-up failed while the module loaded: ${String((error && error.stack) || error).replace(/\s+/g, " ").slice(0, 900)}`;
			});
		});
		return runtime;
	}

	async function start(env, ctx) {
		const runtime = begin(await loadModule(), env, ctx);
		await runtime.isReady;
		return runtime;
	}

	// take is a waiting runtime whose Go program is still running. One that served with workers-go's blocking
	// workers.Serve ends itself when its answer's body closes, as workers-go means it to (a runtime per request): it
	// is passed over, and the request starts a new one.
	function take() {
		for (let runtime = waiting.pop(); runtime; runtime = waiting.pop()) if (!runtime.go.exited) return runtime;
		return undefined;
	}

	function done(runtime) {
		if (!runtime.go.exited && !runtime.binding.full && waiting.length < MAX_WAITING) waiting.push(runtime);
	}

	return { fetch, scheduled, queue, warm };

	// run takes a runtime for one event that is no request (a cron trigger, a batch from a queue): one that is
	// waiting, or a new one. The bindings and waitUntil are this event's. It gives the event to what the Go program
	// registered for it, as workers-go's own entry does, and the runtime is free again when that settles.
	async function run(env, ctx, handler, event, what) {
		let runtime = take();
		if (runtime) {
			runtime.context.env = env;
			runtime.context.ctx = ctx;
		} else {
			runtime = await start(env, ctx);
		}
		runtime.served++;
		const handle = runtime.binding[handler];
		if (typeof handle !== "function") {
			done(runtime);
			throw new Error(`the Worker got ${what}, and its Go registers nothing for it`);
		}
		try {
			return await handle(event);
		} finally {
			done(runtime);
		}
	}

	// scheduled is the Worker's handler for a Cron Trigger: workers-go's cron.ScheduleTaskNonBlock gets the event.
	function scheduled(event, env, ctx) {
		return run(env, ctx, "runScheduler", event, "a Cron Trigger (cron.ScheduleTaskNonBlock, before transport.Run)");
	}

	// queue is the Worker's handler for a batch from a queue: workers-go's queues.ConsumeNonBlock gets it.
	function queue(batch, env, ctx) {
		return run(env, ctx, "handleQueueMessageBatch", batch, "a batch from a queue (queues.ConsumeNonBlock, before transport.Run)");
	}

	// fetch is the Worker's: a WebSocket upgrade is carried by websocket.mjs (which gives it to Go
	// as plain HTTP), everything else goes to Go as it is.
	function fetch(request, env, ctx) {
		if (request.headers.get("upgrade") === "websocket") return webSocket({ fetch: serve }, request, env, ctx);
		return serve(request, env, ctx);
	}

	// warm starts runtimes before any request, while the Worker's module loads: await it at the top
	// of the entry. A request that starts its own runtime in a new isolate costs about 100 ms of
	// CPU; one that finds a warm runtime costs 4 to 19.
	//
	//   - runtimes: how many (default 1). Each is start-up time for
	//     the isolate (Cloudflare allows about a second) and its heap held. Requests that arrive together beyond
	//     that number start their own.
	//   - paths: each runtime answers a GET of these during start-up, into nothing (transport.Run),
	//     so the code is compiled and the operations registered. Only paths whose handlers touch
	//     no binding: a runtime has none until a request gives it its own.
	//
	// If Go's start-up can't finish there, requests start runtimes as before, and a request with
	// the header x-go-runtime is told why in the answer's x-go-runtime.
	async function warm({ runtimes, paths = [] } = {}) {
		// A runtime takes about 150 ms of CPU to start, of the second Cloudflare allows a module to load in.
		runtimes ??= 1;
		try {
			const module = await loadModule();
			for (let i = 0; i < runtimes && waiting.length < MAX_WAITING; i++) {
				const runtime = begin(module, {}, undefined, paths);
				if (!runtime.started) {
					warmFailure ||= "start-up did not reach workers.Ready() while the module loaded";
					return;
				}
				waiting.push(runtime);
			}
		} catch (error) {
			warmFailure = `start-up failed while the module loaded: ${String(error)}`;
		}
	}

	// serveWorkersGo gives a request to a Go program that serves the workers-go way, workers.Serve(handler): the
	// request itself goes to workers-go's handleRequest, so everything of workers-go that reads it works as its docs
	// say (fetch.NewIncomingProperties, the Cache API). What charter adds is only that the runtime is kept for the next
	// request instead of started again.
	async function serveWorkersGo(request, env, ctx) {
		let runtime = take();
		const kind = runtime ? (runtime.served ? "reused" : runtime.warm ? "warm" : "new") : "new";
		if (runtime) {
			runtime.context.env = env;
			runtime.context.ctx = ctx;
		} else {
			runtime = await start(env, ctx);
		}
		runtime.served++;
		try {
			const response = await runtime.binding.handleRequest(request);
			if (!request.headers.has("x-go-runtime")) return response;
			const told = new Response(response.body, response);
			told.headers.set("x-go-runtime", warmFailure ? `${kind}; ${warmFailure}` : kind);
			return told;
		} finally {
			done(runtime);
		}
	}

	// serve gives a request to a Go runtime as two values, and gets the answer in one call when the
	// handler did not stream (answer in transport_js.go has the protocol): every value that crosses
	// between JavaScript and Go costs, and so does every call into Go.
	async function serve(request, env, ctx) {
		// Which way the Go program serves is known once a runtime has started: charter's transport.Run registers
		// serve; workers-go's own workers.Serve registers handleRequest.
		if (workersGo === undefined) {
			const first = take() ?? (await start(env, ctx));
			workersGo = typeof first.binding.serve !== "function";
			waiting.push(first);
		}
		if (workersGo) return serveWorkersGo(request, env, ctx);
		let head = `${request.method}\n${request.url}`;
		for (const [name, value] of request.headers) if (name !== CF_HEADER) head += `\n${name}\n${value}`;
		// request.cf, for transport.CFOf: the fields it reads, as one JSON line.
		if (request.cf) head += `\n${CF_HEADER}\n${JSON.stringify(cfFields(request.cf))}`;
		const body = request.body ? new Uint8Array(await request.arrayBuffer()) : null;
		let runtime = take();
		// Which kind of runtime a request got, in the answer's x-go-runtime: for looking into a slow one.
		const kind = runtime ? (runtime.served ? "reused" : runtime.warm ? "warm" : "new") : "new";
		if (runtime) {
			// The bindings and waitUntil are this request's.
			runtime.context.env = env;
			runtime.context.ctx = ctx;
		} else {
			runtime = await start(env, ctx);
		}
		runtime.served++;
		return new Promise(resolve => {
			const { binding } = runtime;
			let stream; // the controller of a streamed body, until the client goes away
			binding.respond = (status, head, body, more) => {
				const headers = new Headers();
				const lines = head.split("\n");
				for (let i = 0; i + 1 < lines.length; i += 2) headers.append(lines[i], lines[i + 1]);
				if (request.headers.has("x-go-runtime")) headers.set("x-go-runtime", warmFailure ? `${kind}; ${warmFailure}` : kind);
				// The whole answer: the runtime is free for the next request.
				if (!more) {
					const response = new Response(body, { status, headers });
					done(runtime);
					return resolve(response);
				}
				// A stream: the runtime is free when Go has written the last of it. If the client goes
				// away first, Go is told (transport.Serve cancels the request's context) and the
				// runtime is dropped.
				let readable = new ReadableStream({
					start(controller) {
						stream = controller;
						if (body) controller.enqueue(body);
					},
					cancel() {
						stream = null;
						binding.cancel?.();
					},
				});
				const length = headers.get("content-length");
				if (length !== null) readable = readable.pipeThrough(new FixedLengthStream(Number(length)));
				resolve(new Response(readable, { status, headers }));
			};
			binding.write = (body, more) => {
				if (!stream) return false;
				if (body) stream.enqueue(body);
				if (!more) {
					stream.close();
					done(runtime);
				}
				return true;
			};
			binding.serve(head, body);
		});
	}
}
