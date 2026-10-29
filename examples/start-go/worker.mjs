// The Worker's entry: everything goes to the Go Worker (TinyGo Wasm in build/, made by
// charter dev, check and deploy). The build also writes the Go library's glue into build/, from the
// version of github.com/joeblew999/charter that go.mod requires (its worker/ folder): go.mjs
// runs Go, keeping its runtimes alive between requests, and carries WebSockets.
import { goWorker } from "./build/go.mjs";

const go = goWorker();
await go.warm({ paths: ["/api/openapi.json", "/api/hello"] });

export default { fetch: go.fetch };
