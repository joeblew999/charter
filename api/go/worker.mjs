// The Worker's entry: everything goes to the Go Worker (TinyGo Wasm in build/, made by
// `mise run api:go:build`). The build also writes the Go library's glue into build/, from the
// version of github.com/joeblew999/orpc-api/go that go.mod requires (its worker/ folder): go.mjs
// runs Go, keeping its runtimes alive between requests, and carries WebSockets; hub.mjs is the hub
// Durable Object class, which has to be JavaScript.
import { goWorker } from "./build/go.mjs";

// The class is the library's Hub. This Worker was deployed with it under the name NotesHub, and
// renaming a deployed Durable Object class is a migration (cloudflare.config.ts).
export { Hub as NotesHub } from "./build/hub.mjs";

const go = goWorker();
await go.warm({ paths: ["/api/openapi.json", "/api/hello"] });

export default { fetch: go.fetch };
