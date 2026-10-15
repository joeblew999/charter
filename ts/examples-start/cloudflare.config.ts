import { bindings, defineConfig } from "cf/config";
import * as entrypoint from "./worker.ts" with { type: "cf-worker" };

// The oRPC API (api/contract.ts), with a D1 database (DB).
export default defineConfig({
	worker: {
		name: "charter-start-ts",
		compatibilityDate: "2026-09-25",
		entrypoint,
		// Workers Logs is off unless enabled.
		observability: { enabled: true },
		env: {
			APP_NAME: bindings.text("charter-start-ts"),
			DB: bindings.d1(),
		},
	},
});
