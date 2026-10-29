// The conformance Worker's entry: everything goes to the Go server (TinyGo Wasm in build/, made by
// npm run dev, check and deploy). The build also writes the Go library's glue into build/ (worker/
// in this repo): go.mjs runs Go with its runtimes kept between requests, carries a WebSocket, here
// both ways, and makes Go's timers fire on Cloudflare; hub.mjs is the hub Durable Object class,
// which has to be JavaScript.
import { goWorker } from "./build/go.mjs";

// The hub's class is the library's, under the name cloudflare.config.ts declares.
export { Hub } from "./build/hub.mjs";

const go = goWorker();
await go.warm({ paths: ["/api/openapi.json"] });

export default { fetch: go.fetch };
