// The Worker's entry: everything goes to the Go Worker (TinyGo Wasm in build/, made by
// `mise run api-go:build`). Three things are JavaScript because Go can't do them here: the hub
// Durable Object class (hub.mjs), carrying a stream over a WebSocket (websocket.mjs), and making
// Go's timers fire on Cloudflare (tinygo-clock.mjs).
import goWorker from "../build/worker.mjs";
import "./tinygo-clock.mjs";
import { webSocket } from "./websocket.mjs";

export { NotesHub } from "./hub.mjs";

export default {
	fetch(request, env, ctx) {
		if (request.headers.get("upgrade") === "websocket") return webSocket(goWorker, request, env, ctx);
		return goWorker.fetch(request, env, ctx);
	},
};
