import { bindings, defineConfig, exports } from "cf/config";
import * as entrypoint from "./src/index.ts" with { type: "cf-worker" };

// The oRPC API (src/contract.ts) on D1, plus NotesHub: a Durable Object that holds the live
// WebSocket connections and SSE streams, and broadcasts every new note to them.
export default defineConfig({
	worker: {
		name: "orpc-api",
		compatibilityDate: "2026-09-25",
		entrypoint,
		assets: {
			notFoundHandling: "single-page-application",
			runWorkerFirst: ["/api/*"],
		},
		exports: {
			NotesHub: exports.durableObject({ storage: "sqlite" }),
		},
		// Workers Logs is off unless enabled; mise run logs / errors need it.
		observability: { enabled: true },
		env: {
			APP_NAME: bindings.text("orpc-api"),
			ASSETS: bindings.assets(),
			DB: bindings.d1(),
			HUB: bindings.durableObject({ worker: "orpc-api", exportName: "NotesHub" }),
		},
	},
});
