// The Vite config every project's vite.config.ts is: export { default } from "charter-ts/vite".
// cf dev and cf deploy run the Worker through Vite with Cloudflare's plugin, on the port charter-ts
// gives (PORT). One copy of oRPC and Zod in the Worker, wherever charter-ts is installed from.
import { cloudflare } from "@cloudflare/vite-plugin";
import { defineConfig } from "vite";

export default defineConfig({
	resolve: { dedupe: ["@orpc/contract", "@orpc/json-schema", "@orpc/openapi", "@orpc/server", "@orpc/zod", "zod"] },
	server: { port: Number(process.env.PORT) || 5173, strictPort: true },
	// CHARTER_LOCAL_STATE: local bindings (D1, Durable Objects) kept there, not in the project's own
	// state; a run that must start from nothing (charter-ts twins) gives it a folder of its own.
	plugins: [cloudflare(process.env.CHARTER_LOCAL_STATE ? { persistState: { path: process.env.CHARTER_LOCAL_STATE } } : {})],
});
