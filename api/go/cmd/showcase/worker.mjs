// The showcase Worker's entry: everything goes to the Go server (TinyGo Wasm in build/, made by
// `mise run showcase:go:build`). The build also writes the Go library's glue into build/ (go/worker/
// in this repo): go.mjs runs Go with its runtimes kept between requests, carries a WebSocket, here
// both ways, and makes Go's timers fire on Cloudflare.
import { goWorker } from "./build/go.mjs";

const go = goWorker();
await go.warm({ paths: ["/openapi.json"] });

export default { fetch: go.fetch };
