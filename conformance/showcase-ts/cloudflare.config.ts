import { bindings, defineConfig } from "cf/config";
import * as entrypoint from "./src/index.ts" with { type: "cf-worker" };

// The SDK test Worker. The same code also deploys as a second Worker, charter-showcase-ts-api (`--mode api`):
// on Cloudflare the WebSocket test connects from charter-showcase-ts to charter-showcase-ts-api, which needs the
// global_fetch_strictly_public flag (without it: error 1042, Worker-to-Worker on the same zone).
export default defineConfig(ctx => ({
	worker: {
		name: ctx.mode === "api" ? "charter-showcase-ts-api" : "charter-showcase-ts",
		compatibilityDate: "2026-09-25",
		// Lets this Worker call another Worker on the same workers.dev zone (else Cloudflare error 1042).
		compatibilityFlags: ["global_fetch_strictly_public"],
		entrypoint,
		assets: {
			notFoundHandling: "single-page-application",
			runWorkerFirst: ["/api/*"],
		},
		// Workers Logs is off unless enabled; mise run logs / errors need it.
		observability: { enabled: true },
		env: {
			APP_NAME: bindings.text("charter-showcase-ts"),
			ASSETS: bindings.assets(),
		},
	},
}));
