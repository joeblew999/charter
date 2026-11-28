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
//     exactly as workers-go does, so nothing waits for another request, while there is memory for one.
//   - The runtimes of an isolate share its memory, and Cloudflare ends a Worker that takes more than 128 MB,
//     with every request it was answering (they get a 503, and its analytics say exceededMemory). So what all
//     of them hold together is counted, from each one's own WebAssembly memory, and kept under a budget: a
//     runtime that would wait for the next request above it is let go instead, the largest first, and a request
//     that would have to start one above it waits a moment for one that is serving to finish.
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

// Where a page's stream is asked for: LIVE, then the Durable Object binding of the hub and the feed's name (the Go
// package live writes the address: Feed.Signal).
const LIVE = "/__charter/live/";
// The header of an answer whose stream the hub carries on: what to follow, as a query (live.Feed sets it).
const FOLLOW = "x-charter-follow";

const MAX_WAITING = 4; // runtimes kept for the next request

// Memory. A runtime of the standard compiler starts at some 13 MB of WebAssembly memory and grows as its heap does;
// that memory never shrinks. BUDGET is what all the runtimes of the isolate may hold together: the rest of the
// 128 MB is the module's compiled code and the JavaScript around it.
const MB = 1048576;
const BUDGET = 80 * MB;
const NEW = 14 * MB; // what a runtime that has just started holds
const WAIT = 3000; // how long a request waits for a runtime to finish before it starts one over the budget, in ms
const LOOK = 20; // how often it looks for one, in ms

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
	const alive = new Set(); // every runtime that holds memory: waiting, or serving
	let warmFailure = "";

	// held is the WebAssembly memory a runtime holds: what the isolate is charged for it.
	const held = runtime => runtime.go._inst?.exports?.mem?.buffer?.byteLength ?? NEW;
	const total = () => {
		let sum = 0;
		for (const runtime of alive) sum += held(runtime);
		return sum;
	};

	// retire lets a runtime go: it is given no more requests and is no longer counted, and with nothing left
	// that refers to it, its memory goes when the engine collects it. It is not stopped: it may still be
	// finishing what a request left it to do after its answer (cloudflare.WaitUntil), and a request whose work
	// is cut off is one the Workers runtime ends as hung.
	function retire(runtime) {
		alive.delete(runtime);
		const at = waiting.indexOf(runtime);
		if (at >= 0) waiting.splice(at, 1);
	}

	// begin starts a runtime. Go's start-up runs inside go.run, up to workers.Ready(): started says
	// whether it got there before go.run returned, isReady resolves when it does.
	function begin(module, env, ctx, warm) {
		const go = new Go();
		const binding = warm ? { warm } : {};
		binding.EmailMessage = EmailMessage;
		// What the isolate's runtimes hold between them, for Go to say (cloud.Runtimes): an admin's page shows it.
		binding.memory = () => ({ runtimes: alive.size, held: total(), budget: BUDGET });
		const context = createRuntimeContext({ env, ctx, binding });
		// asked: it has been given a request or an event, which is when Cloudflare's clock runs for it.
		const runtime = { go, binding, context, started: false, asked: !warm, served: 0, warm: Boolean(warm) };
		alive.add(runtime);
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
				// ("nanotime returning zero"; and its collector's "stopTheWorld: broken CPU time accounting", which
				// takes a zero for a time it was never told). It gets one nanosecond there; the real time comes with
				// the first request or event it is given (runtimeFor), and not before: a runtime that has started
				// still runs while the module loads, and a collection begun in its warm-up ends there.
				const nanotime = gojs["runtime.nanotime1"];
				gojs["runtime.nanotime1"] = sp => {
					nanotime(sp);
					if (runtime.asked) return;
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
			go.run(instance, context)
				.catch(error => {
					if (warm) warmFailure = `start-up failed while the module loaded: ${String((error && error.stack) || error).replace(/\s+/g, " ").slice(0, 900)}`;
				})
				// A Go program that has ended, however it did, holds nothing the isolate should count or keep.
				.finally(() => retire(runtime));
		});
		return runtime;
	}

	async function start(env, ctx) {
		const runtime = begin(await loadModule(), env, ctx);
		await runtime.isReady;
		return runtime;
	}

	// runtimeFor is a runtime for one request or event: one that is waiting, or a new one. When a new one would take
	// the isolate over its budget, it first waits for one that is serving to finish (WAIT at most): a request that is
	// a moment late is answered, and one that takes the Worker over its memory ends it, with everything in flight.
	async function runtimeFor(env, ctx) {
		let runtime = take();
		// It looks again every few milliseconds, itself: a request may not be woken by another request's work (the
		// Workers runtime ends one that waits for that, as hung).
		for (let waited = 0; !runtime && alive.size > 0 && total() + NEW > BUDGET && waited < WAIT; waited += LOOK) {
			await new Promise(resolve => setTimeout(resolve, LOOK));
			runtime = take();
		}
		if (runtime) {
			runtime.asked = true;
			runtime.context.env = env;
			runtime.context.ctx = ctx;
			return { runtime, kind: runtime.served ? "reused" : runtime.warm ? "warm" : "new" };
		}
		return { runtime: await start(env, ctx), kind: "new" };
	}

	// take is a waiting runtime whose Go program is still running. One that served with workers-go's blocking
	// workers.Serve ends itself when its answer's body closes, as workers-go means it to (a runtime per request): it
	// is passed over, and the request starts a new one.
	function take() {
		for (let runtime = waiting.pop(); runtime; runtime = waiting.pop()) if (!runtime.go.exited) return runtime;
		return undefined;
	}

	// done is a runtime that has finished what it was given. One that may serve again waits for the next request;
	// one that may not (its Go program ended, or says its heap is full) is ended.
	// Then what waits is cut back to the budget, the largest first: memory is for requests, before it is for the
	// next one's start.
	function done(runtime) {
		if (runtime.go.exited || runtime.binding.full) retire(runtime);
		else waiting.push(runtime);
		while (waiting.length > 0 && (waiting.length > MAX_WAITING || total() > BUDGET)) {
			retire(waiting.reduce((largest, other) => (held(other) > held(largest) ? other : largest)));
		}
	}

	return { fetch, scheduled, queue, email, warm };

	// run takes a runtime for one event that is no request (a cron trigger, a batch from a queue): one that is
	// waiting, or a new one. The bindings and waitUntil are this event's. It gives the event to what the Go program
	// registered for it, as workers-go's own entry does, and the runtime is free again when that settles.
	async function run(env, ctx, handler, event, what) {
		const { runtime } = await runtimeFor(env, ctx);
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
		// A page's stream is the hub's to hold, in JavaScript: no Go runtime waits on it (hub.mjs says why).
		const url = new URL(request.url);
		if (request.method === "GET" && url.pathname.startsWith(LIVE)) {
			const [binding, ...name] = url.pathname.slice(LIVE.length).split("/");
			const namespace = env[binding];
			if (typeof namespace?.idFromName !== "function" || name.length !== 1 || !name[0]) return new Response("no such feed", { status: 404 });
			return namespace.get(namespace.idFromName(decodeURIComponent(name[0]))).fetch(new Request("https://hub/signal" + url.search, { headers: request.headers, signal: request.signal }));
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

	// handOver joins the hub's stream to an answer of Go's that asks for it (the header FOLLOW, which live.Feed's
	// Events and Lines set): Go has written what the log had and is done, and the rest of the stream, which lasts,
	// is the hub's. So no Go runtime waits on a stream. An answer without the header is returned as it is.
	function handOver(response, request, env, ctx) {
		const follow = response.headers.get(FOLLOW);
		if (!follow) return response;
		const headers = new Headers(response.headers);
		headers.delete(FOLLOW);
		const ask = new URLSearchParams(follow);
		const namespace = env[ask.get("binding")];
		if (typeof namespace?.idFromName !== "function") return new Response(response.body, { status: response.status, headers });
		const hub = namespace.get(namespace.idFromName(ask.get("name")));
		const { readable, writable } = new TransformStream();
		const joined = (async () => {
			// Go's part first, to its end; then the hub's, until its reader goes or its time is up.
			await response.body.pipeTo(writable, { preventClose: true });
			const rest = await hub.fetch(new Request("https://hub/follow?" + follow, { signal: request.signal }));
			await rest.body.pipeTo(writable);
		})().catch(() => writable.abort().catch(() => {}));
		ctx?.waitUntil?.(joined);
		return new Response(readable, { status: response.status, headers });
	}

	// serve gives a request to a Go runtime through workers-go's own path (the handleRequest that workers.Serve,
	// workers.ServeNonBlock and so charter's app.Run register): the request itself goes to Go, so everything
	// of workers-go that reads it works as its docs say. What charter adds is that the runtime is kept for the next
	// request instead of started again for each. (charter had a path of its own here, two strings in and one call
	// back; measured on 2026-11-25 with the standard compiler it saved 0.2 to 0.3 ms a request, and cost workers-go's
	// request properties and Cache API. It went.)
	async function serve(request, env, ctx) {
		const { runtime, kind } = await runtimeFor(env, ctx);
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
		// If this throws, the runtime is not offered again, and what it held is not counted.
		let response;
		try {
			response = await runtime.binding.handleRequest(request);
		} catch (error) {
			retire(runtime);
			throw error;
		}
		response = handOver(response, request, env, ctx);
		if (!request.headers.has("x-go-runtime")) return response;
		const told = new Response(response.body, response);
		// Who asks is told which runtime answered, and what the isolate's runtimes hold between them.
		const memory = `${alive.size} runtimes, ${Math.round(total() / MB)} MB of ${BUDGET / MB}`;
		told.headers.set("x-go-runtime", warmFailure ? `${kind}; ${memory}; ${warmFailure}` : `${kind}; ${memory}`);
		return told;
	}
}
