// The Worker's entry: everything goes to the Go Worker (TinyGo Wasm in build/, made by
// `mise run build`). The build also writes the Go library's glue into build/, from the
// version of github.com/joeblew999/charter/go that go.mod requires (its worker/ folder): go.mjs
// runs Go, keeping its runtimes alive between requests; hub.mjs is the hub Durable Object class,
// which has to be JavaScript.
import { goWorker } from "./build/go.mjs";

// The hub's class is the library's, under the name cloudflare.config.ts declares.
export { Hub } from "./build/hub.mjs";

const go = goWorker();
await go.warm({ paths: ["/api/openapi.json", "/api/hello"] });

export default { fetch: go.fetch };
