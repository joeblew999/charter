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
//   - A runtime is reused when its handler has returned (app.Serve says so). After an error it is
//     dropped.
//   - A runtime is dropped before Go's collector would run in it: Go says when its heap has no
//     room for another request (app.Serve sets the binding's "full"). So a runtime serves
//     some dozens of requests, not thousands, and what must be shared between requests still goes
//     through a binding.
//
// The Go side must stay alive after a response: app.Run instead of workers.Serve.
//
// A project's entry (worker.mjs) is then:
//
//	import { goWorker } from "./build/go.mjs";
//	const go = goWorker();
//	await go.warm({ paths: ["/api/hello"] });
//	export default { fetch: go.fetch, scheduled: go.scheduled, queue: go.queue, email: go.email };
import "./wasm_exec.js"; // the compiler's (workers-go writes it), for its global Go class
import * as build from "./runtime.mjs";
import { EmailMessage } from "cloudflare:email"; // for cloud.SendEmail: what a send_email binding takes
import { webSocket } from "./websocket.mjs";

// request.cf's fields app.CF has (cf.go), under the header name it reads for a WebSocket's messages (lower
// case, as Headers gives names). A client's own header of that name is dropped.
const CF_HEADER = "x-charter-cf";
const CF_FIELDS = ["country", "region", "regionCode", "city", "postalCode", "continent", "timezone", "latitude", "longitude", "colo", "asn", "isEUCountry"];
function cfFields(cf) {
	const picked = {};
	for (const name of CF_FIELDS) if (cf[name] !== undefined) picked[name] = cf[name];
	return picked;
}

// workers-go v0.36.0 answers with a ReadableStream that Go fills (internal/jsutil/stream.go). When the reader goes
// away while Go waits for the next bytes (every closed event stream), the stream is cancelled, Go's read ends, and
// workers-go closes the stream's controller, which throws "This ReadableStream is closed": in Go that is a panic,
// and the whole Go program exits. Closing or filling a stream that is already closed is nothing to end a program
// over, so here it does nothing. Upstream: syumai/workers-go, readerToReadableStream.Pull.
new ReadableStream({
	start(controller) {
		const proto = Object.getPrototypeOf(controller);
		for (const name of ["close", "enqueue"]) {
			const original = proto[name];
			proto[name] = function (...args) {
				try {
					return original.apply(this, args);
				} catch (error) {
					if (error instanceof TypeError && this.desiredSize === null) return undefined; // errored, or closed and read to its end
					if (error instanceof TypeError && /closed/i.test(error.message)) return undefined;
					throw error;
				}
			};
		}
	},
});

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
	let warmFailure = ""; // why no runtime was started while the module loaded, for the x-go-runtime header

	// begin starts a runtime. Go's start-up runs inside go.run, up to workers.Ready(): started says
	// whether it got there before go.run returned, isReady resolves when it does.
	function begin(module, env, ctx, warm) {
		const go = new Go();
		const binding = warm ? { warm } : {};
		binding.EmailMessage = EmailMessage;
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

	return { fetch, scheduled, queue, email, warm };

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
		return run(env, ctx, "runScheduler", event, "a Cron Trigger (cron.ScheduleTaskNonBlock, before app.Run)");
	}

	// email is the Worker's handler for a message Email Routing hands it: cloud.ConsumeEmail gets it.
	function email(message, env, ctx) {
		return run(env, ctx, "handleEmail", message, "an email (cloud.ConsumeEmail, before the program serves)");
	}

	// queue is the Worker's handler for a batch from a queue: workers-go's queues.ConsumeNonBlock gets it.
	function queue(batch, env, ctx) {
		return run(env, ctx, "handleQueueMessageBatch", batch, "a batch from a queue (queues.ConsumeNonBlock, before app.Run)");
	}

	// fetch is the Worker's: a WebSocket upgrade is carried by websocket.mjs (which gives it to Go
	// as plain HTTP), everything else goes to Go as it is.
	function fetch(request, env, ctx) {
		// Only websocket.mjs may say where a request came from this way: a client's own header of that name goes.
		if (request.headers.has(CF_HEADER)) {
			request = new Request(request);
			request.headers.delete(CF_HEADER);
		}
		if (request.headers.get("upgrade") === "websocket") {
			// Each message of the socket reaches Go as a request the adapter makes, which Cloudflare knows nothing
			// of: the upgrade's request.cf goes along with them, as a header.
			const upgrade = new Request(request);
			if (request.cf) upgrade.headers.set(CF_HEADER, JSON.stringify(cfFields(request.cf)));
			return webSocket({ fetch: serve }, upgrade, env, ctx);
		}
		return serve(request, env, ctx);
	}

	// warm starts runtimes before any request, while the Worker's module loads: await it at the top
	// of the entry. A request that starts its own runtime in a new isolate costs about 100 ms of
	// CPU; one that finds a warm runtime costs 4 to 19.
	//
	//   - runtimes: how many (default 1). Each is start-up time for
	//     the isolate (Cloudflare allows about a second) and its heap held. Requests that arrive together beyond
	//     that number start their own.
	//   - paths: each runtime answers a GET of these during start-up, into nothing (app.Run),
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

	// serve gives a request to a Go runtime through workers-go's own path (the handleRequest that workers.Serve,
	// workers.ServeNonBlock and so charter's app.Run register): the request itself goes to Go, so everything
	// of workers-go that reads it works as its docs say. What charter adds is that the runtime is kept for the next
	// request instead of started again for each. (charter had a path of its own here, two strings in and one call
	// back; measured on 2026-11-25 with the standard compiler it saved 0.2 to 0.3 ms a request, and cost workers-go's
	// request properties and Cache API. It went.)
	async function serve(request, env, ctx) {
		let runtime = take();
		const kind = runtime ? (runtime.served ? "reused" : runtime.warm ? "warm" : "new") : "new";
		if (runtime) {
			runtime.context.env = env;
			runtime.context.ctx = ctx;
		} else {
			runtime = await start(env, ctx);
		}
		runtime.served++;
		// workers-go answers when the handler has written its headers: Go is still writing the body. The runtime
		// serves one request at a time, so it waits for the next only when the handler has returned, which
		// app.Serve says by calling the binding's "ended" (by then every byte it wrote is in the answer's
		// stream). A program that serves without app.Run never says so: its runtimes are not offered again,
		// as workers-go itself starts one for each request.
		const ended = new Promise(resolve => (runtime.binding.ended = resolve)).then(() => done(runtime));
		ctx?.waitUntil?.(ended);
		// If the reader goes away first (a closed event stream), Cloudflare says so on request.signal, and Go is told:
		// app.Serve ends the request's context, so a handler that waits returns. Cloudflare gives that signal
		// from compatibility date 2026-11-17, or with the flag enable_request_signal before it; without it the
		// handler is left waiting until Cloudflare ends the invocation, some 30 seconds later.
		const left = () => runtime.binding.cancel?.();
		if (request.signal?.aborted) left();
		else request.signal?.addEventListener("abort", left, { once: true });
		// If this throws, the runtime is not offered again.
		const response = await runtime.binding.handleRequest(request);
		if (!request.headers.has("x-go-runtime")) return response;
		const told = new Response(response.body, response);
		told.headers.set("x-go-runtime", warmFailure ? `${kind}; ${warmFailure}` : kind);
		return told;
	}
}
