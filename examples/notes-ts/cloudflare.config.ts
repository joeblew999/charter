import { bindings, defineConfig, exports } from "cf/config";
import * as entrypoint from "./src/index.ts" with { type: "cf-worker" };

// The oRPC API (src/contract.ts) on D1, plus NotesHub: a hibernating Durable Object that fans every
// new note out to the Worker's follow() loops. The Worker, not the hub, holds the clients' SSE
// streams and WebSockets (docs/guides/streaming.md).
export default defineConfig({
	worker: {
		name: "charter-notes-ts",
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
			APP_NAME: bindings.text("charter-notes-ts"),
			ASSETS: bindings.assets(),
			DB: bindings.d1(),
			HUB: bindings.durableObject({ worker: "charter-notes-ts", exportName: "NotesHub" }),
			// The tokens (src/index.ts): set by every mise run deploy, from the environment under cf dev.
			READ_TOKEN: bindings.secret(),
			WRITE_TOKEN: bindings.secret(),
		},
	},
});
