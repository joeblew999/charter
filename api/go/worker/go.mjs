// Runs the Go Worker (TinyGo Wasm in build/), keeping Go runtimes alive between requests.
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
// The entry gives it the build (TinyGo's wasm_exec.js is imported there, for its global Go class):
//
//	import "../build/wasm_exec.js";
//	import * as build from "../build/runtime.mjs";
//	const go = goWorker(build);

const MAX_WAITING = 4; // runtimes kept for the next request; each holds its Go heap (8 MB)

// workers-go's Go code calls this.
globalThis.tryCatch = fn => {
	try {
		return { result: fn() };
	} catch (error) {
		return { error };
	}
};

// seedOnly says whether a call for random bytes comes from TinyGo's runtime taking its seed as the
// program starts: the only Wasm functions on the stack are arc4random, runtime.hardwareRand (when
// it is not inlined) and _start. Package initialisers and main run in a goroutine, further down.
function seedOnly(stack) {
	const frames = stack.split("\n").filter(line => line.includes("wasm://"));
	return frames.length > 0 && frames.every(line => /\.(arc4random|runtime\.hardwareRand|_start(\.command_export)?) \(/.test(line));
}

// goWorker is a Worker (its fetch) over a workers-go build: build/runtime.mjs, as a module.
export function goWorker({ createRuntimeContext, loadModule }) {
	const waiting = [];
	let warmFailure = ""; // why no runtime was started while the module loaded, for the x-go-runtime header

	// begin starts a runtime. Go's start-up runs inside go.run, up to workers.Ready(): started says
	// whether it got there before go.run returned, isReady resolves when it does.
	function begin(module, env, ctx, warm) {
		const go = new Go();
		const binding = warm ? { warm } : {};
		const context = createRuntimeContext({ env, ctx, binding });
		const runtime = { go, binding, context, started: false, served: 0, warm: Boolean(warm) };
		runtime.isReady = new Promise(resolve => {
			const wasi = go.importObject.wasi_snapshot_preview1;
			const random = wasi.random_get;
			wasi.random_get = (pointer, length) => {
				if (runtime.started || !warm) return random(pointer, length);
				// While the module loads, Cloudflare gives no crypto randomness. TinyGo's runtime asks for
				// its seed there (hash maps, fastrand): it gets Math.random's. Anything else that wants
				// random bytes during start-up fails, and with it the warm start.
				if (!seedOnly(new Error().stack)) throw new Error("random bytes wanted during start-up, other than the runtime's seed");
				new Uint8Array(go._inst.exports.memory.buffer, pointer >>> 0, length >>> 0).forEach((_, i, bytes) => (bytes[i] = Math.random() * 256));
				return 0;
			};
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

	function done(runtime) {
		if (!runtime.go.exited && !runtime.binding.full && waiting.length < MAX_WAITING) waiting.push(runtime);
	}

	return { fetch, warm };

	// warm starts runtimes before any request, while the Worker's module loads: await it at the top
	// of the entry. A request that starts its own runtime in a new isolate costs about 100 ms of
	// CPU; one that finds a warm runtime costs 4 to 19 (docs/benchmarks.md).
	//
	//   - runtimes: how many. Each is start-up time for the isolate (Cloudflare allows about a
	//     second) and 8 MB held. Requests that arrive together beyond that number start their own.
	//   - paths: each runtime answers a GET of these during start-up, into nothing (transport.Run),
	//     so the code is compiled and the operations registered. Only paths whose handlers touch
	//     no binding: a runtime has none until a request gives it its own.
	//
	// If Go's start-up can't finish there, requests start runtimes as before, and a request with
	// the header x-go-runtime is told why in the answer's x-go-runtime.
	async function warm({ runtimes = 2, paths = [] } = {}) {
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

	// fetch gives a request to a Go runtime as two values, and gets the answer in one call when the
	// handler did not stream (answer in transport_js.go has the protocol): every value that crosses
	// between JavaScript and Go costs, and so does every call into Go.
	async function fetch(request, env, ctx) {
		let head = `${request.method}\n${request.url}`;
		for (const [name, value] of request.headers) head += `\n${name}\n${value}`;
		const body = request.body ? new Uint8Array(await request.arrayBuffer()) : null;
		let runtime = waiting.pop();
		// For measuring (dev bench -each): which kind of runtime a request got.
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
