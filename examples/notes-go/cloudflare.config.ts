import { bindings, defineConfig, exports } from "cf/config";
import * as entrypoint from "./worker.mjs" with { type: "cf-worker" };

// The Go Worker: the notes API, written in Go on workers-go and built with TinyGo.
// D1 is the log; Hub is the hibernating live fan-out: the library's Hub class (build/hub.mjs),
// which worker.mjs exports.
// A mode that starts with "perf-" (charter perf) deploys a scratch Worker of that name, with a
// database and a hub of its own.
export default defineConfig(ctx => {
	const name = ctx.mode?.startsWith("perf-") ? `charter-notes-go-${ctx.mode}` : "charter-notes-go";
	return {
		worker: {
			name,
			compatibilityDate: "2026-09-25",
			entrypoint,
			exports: {
				Hub: exports.durableObject({ storage: "sqlite" }),
			},
			observability: { enabled: true },
			env: {
				APP_NAME: bindings.text(name),
				DB: bindings.d1(),
				HUB: bindings.durableObject({ worker: name, exportName: "Hub" }),
			},
		},
	};
});
