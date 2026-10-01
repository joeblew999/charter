import { bindings, defineConfig, exports } from "cf/config";
import * as entrypoint from "./worker/index.mjs" with { type: "cf-worker" };

// The Go Worker: the same notes API as api/ (oRPC), written in Go on workers-go and built with TinyGo.
// D1 is the log; NotesHub (worker/hub.mjs) is the hibernating live fan-out.
export default defineConfig({
	worker: {
		name: "orpc-api-go",
		compatibilityDate: "2026-09-25",
		entrypoint,
		exports: {
			NotesHub: exports.durableObject({ storage: "sqlite" }),
		},
		observability: { enabled: true },
		env: {
			APP_NAME: bindings.text("orpc-api-go"),
			DB: bindings.d1(),
			HUB: bindings.durableObject({ worker: "orpc-api-go", exportName: "NotesHub" }),
		},
	},
});
