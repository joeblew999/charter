import { bindings, defineConfig, exports } from "cf/config";
import * as entrypoint from "./src/index.ts" with { type: "cf-worker" };

// The oRPC API (src/contract.ts) on D1, plus NotesHub: a hibernating Durable Object that fans every
// new note out to the Worker's follow() loops. The Worker, not the hub, holds the clients' SSE
// streams and WebSockets (docs/realtime.md).
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
		// Workers Logs is off unless enabled.
		observability: { enabled: true },
		env: {
			APP_NAME: bindings.text("orpc-api"),
			ASSETS: bindings.assets(),
			DB: bindings.d1(),
			HUB: bindings.durableObject({ worker: "orpc-api", exportName: "NotesHub" }),
		},
	},
});
