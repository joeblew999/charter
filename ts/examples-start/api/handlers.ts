import { env } from "cloudflare:workers";
import { implement } from "@orpc/server";
import { guard, serve, type Served } from "charter-lib/serve";
import { contract } from "./contract.ts";
import { document } from "./spec.ts";

// The contract (api/contract.ts) implemented, and served the standard way (charter-lib/serve): the
// routes under /api, both specs, the routes as MCP tools at /api/mcp. Each call goes through the guard
// (auth and rate limits, once the API trusts anyone: serve's `trusted`). The D1 database is env.DB
// (cloudflare.config.ts). The router is built from `os`, without the guard: oRPC would otherwise run
// it twice per call.
const os = implement(contract).$context<Served>();
const api = os.use(guard);

export const router = os.router({
	hello: api.hello.handler(() => ({ message: `Hello from ${env.APP_NAME}` })),
});

/** The API, as a fetch function: what the Worker (worker.ts) answers with. */
export const handler = serve(router, { document });
