// The Worker's entry: everything goes to the Go Worker (TinyGo Wasm in build/, made by
// `mise run api:go:build`), run by go.mjs, which keeps Go runtimes alive between requests. Three
// more things are JavaScript because Go can't do them here: the hub Durable Object class (hub.mjs),
// carrying a stream over a WebSocket (websocket.mjs), and making Go's timers fire on Cloudflare
// (tinygo-clock.mjs).
import "../build/wasm_exec.js";
import * as build from "../build/runtime.mjs";
import { goWorker } from "./go.mjs";
import "./tinygo-clock.mjs";
import { webSocket } from "./websocket.mjs";

export { NotesHub } from "./hub.mjs";

const go = goWorker(build);
await go.warm({ paths: ["/api/openapi.json", "/api/hello"] });

export default {
	fetch(request, env, ctx) {
		if (request.headers.get("upgrade") === "websocket") return webSocket(go, request, env, ctx);
		return go.fetch(request, env, ctx);
	},
};
