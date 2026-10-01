// The showcase Worker's entry: everything goes to the Go server (TinyGo Wasm in build/, made by
// `mise run showcase-go:build`). The glue is the notes Worker's (../../worker): carrying a
// WebSocket, here both ways, and making Go's timers fire on Cloudflare.
import goWorker from "./build/worker.mjs";
import "../../worker/tinygo-clock.mjs";
import { webSocket } from "../../worker/websocket.mjs";

export default {
	fetch(request, env, ctx) {
		if (request.headers.get("upgrade") === "websocket") return webSocket(goWorker, request, env, ctx);
		return goWorker.fetch(request, env, ctx);
	},
};
