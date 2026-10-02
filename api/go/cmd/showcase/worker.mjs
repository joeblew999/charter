// The showcase Worker's entry: everything goes to the Go server (TinyGo Wasm in build/, made by
// `mise run showcase-go:build`). The glue is the notes Worker's (../../worker): running Go with
// its runtimes kept between requests, carrying a WebSocket, here both ways, and making Go's timers
// fire on Cloudflare.
import "./build/wasm_exec.js";
import * as build from "./build/runtime.mjs";
import { goWorker } from "../../worker/go.mjs";
import "../../worker/tinygo-clock.mjs";
import { webSocket } from "../../worker/websocket.mjs";

const go = goWorker(build);
await go.warm({ paths: ["/openapi.json"] });

export default {
	fetch(request, env, ctx) {
		if (request.headers.get("upgrade") === "websocket") return webSocket(go, request, env, ctx);
		return go.fetch(request, env, ctx);
	},
};
