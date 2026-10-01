// The Worker's entry: everything goes to the Go Worker (TinyGo Wasm in build/, made by
// `mise run api-go:build`). Three things are JavaScript because Go can't do them here: the hub
// Durable Object class (hub.mjs), carrying a stream over a WebSocket (below), and making Go's
// timers fire on Cloudflare (tinygo-clock.mjs).
import goWorker from "../build/worker.mjs";
import "./tinygo-clock.mjs";

export { NotesHub } from "./hub.mjs";

// WebSocket transport. Go answers a WebSocket upgrade with a stream of lines (one JSON message per
// line; empty lines are padding), or with an error (400, 426) that is returned as it is. Each line
// is sent as one text frame. Which paths are WebSockets, their input and their feed are all Go's
// business. Go's stream only ends when its feed gave up (the hub is down), so the socket is then
// closed with 1011 and the client reconnects with `after`: it never stays open with nothing behind it.
async function webSocket(request, env, ctx) {
	const lines = await goWorker.fetch(request, env, ctx);
	if (!lines.ok || !lines.body) return lines;
	const [client, server] = Object.values(new WebSocketPair());
	server.accept();
	const reader = lines.body.pipeThrough(new TextDecoderStream()).getReader();
	const stop = () => void reader.cancel().catch(() => {});
	server.addEventListener("close", stop);
	server.addEventListener("error", stop);
	void (async () => {
		let buffer = "";
		try {
			for (;;) {
				const { value, done } = await reader.read();
				if (done) break;
				buffer += value;
				const complete = buffer.split("\n");
				buffer = complete.pop();
				for (const line of complete) if (line) server.send(line);
			}
		} catch {
			// The stream broke: close below.
		}
		try {
			server.close(1011, "hub unavailable; reconnect with after");
		} catch {
			// The client closed first.
		}
	})();
	return new Response(null, { status: 101, webSocket: client });
}

export default {
	fetch(request, env, ctx) {
		if (request.headers.get("upgrade") === "websocket") return webSocket(request, env, ctx);
		return goWorker.fetch(request, env, ctx);
	},
};
