// The Worker's entry (wrangler.jsonc's main). charter build writes build/: go.mjs runs the Go, keeping its runtimes
// between requests; hub.mjs is the hub's Durable Object class, which has to be JavaScript.
import { goWorker } from "./build/go.mjs";

export { Hub } from "./build/hub.mjs";

const go = goWorker();
await go.warm({ paths: ["/api/openapi.json"] });

export default { fetch: go.fetch };
