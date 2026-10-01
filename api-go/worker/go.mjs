// Runs the Go Worker (TinyGo Wasm in build/), keeping Go runtimes alive between requests.
//
// workers-go's own entry (build/worker.mjs) starts a Go runtime for every request and drops it
// afterwards: every request pays for Go's start-up, and the engine then has the runtime's whole
// memory to collect. Here a runtime that has finished a request waits for the next one.
//
//   - A runtime serves one request at a time. A request that finds none waiting starts its own,
//     exactly as workers-go does, so nothing waits for another request.
//   - A runtime is reused only when its response ended normally: the body was read to its end.
//     After an error, or when the client went away, it is dropped.
//   - A runtime is dropped before Go's collector would run in it: Go says when its heap has no
//     room for another request (transport.Serve sets the binding's "full"). So a runtime serves a
//     few requests, not thousands, and what must be shared between requests still goes through a
//     binding.
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

// goWorker is a Worker (its fetch) over a workers-go build: build/runtime.mjs, as a module.
export function goWorker({ createRuntimeContext, loadModule }) {
	const waiting = [];

	async function start(env, ctx) {
		const go = new Go();
		const binding = {};
		const context = createRuntimeContext({ env, ctx, binding });
		let ready;
		const isReady = new Promise(resolve => (ready = resolve));
		const instance = new WebAssembly.Instance(await loadModule(), { ...go.importObject, workers: { ready: () => ready() } });
		go.run(instance, context);
		await isReady;
		return { go, binding, context };
	}

	function done(runtime) {
		if (!runtime.go.exited && !runtime.binding.full && waiting.length < MAX_WAITING) waiting.push(runtime);
	}

	return { fetch };

	async function fetch(request, env, ctx) {
		let runtime = waiting.pop();
		if (runtime) {
			// The bindings and waitUntil are this request's.
			runtime.context.env = env;
			runtime.context.ctx = ctx;
		} else {
			runtime = await start(env, ctx);
		}
		const response = await runtime.binding.handleRequest(request);
		if (!response.body) {
			done(runtime);
			return response;
		}
		// The runtime is free when the body has been read to its end. If the client goes away
		// first, Go is told (transport.Serve cancels the request's context) and the runtime is dropped.
		const reader = response.body.getReader();
		let body = new ReadableStream({
			async pull(controller) {
				const { value, done: ended } = await reader.read();
				if (!ended) return controller.enqueue(value);
				controller.close();
				done(runtime);
			},
			cancel(reason) {
				runtime.binding.cancel?.();
				return reader.cancel(reason);
			},
		});
		const length = response.headers.get("content-length");
		if (length !== null) body = body.pipeThrough(new FixedLengthStream(Number(length)));
		return new Response(body, response);
	}
}
