import { bindings, defineConfig, exports } from "cf/config";
import * as entrypoint from "./worker.mjs" with { type: "cf-worker" };

// The Go Worker: the same notes API as api/ts/ (oRPC), written in Go on workers-go and built with TinyGo.
// D1 is the log; NotesHub is the hibernating live fan-out: the library's Hub class (build/hub.mjs), exported under
// the name this Worker was deployed with (worker.mjs).
// A mode that starts with "perf-" (dev perf) deploys a scratch Worker of that name, with a database
// and a hub of its own.
export default defineConfig(ctx => {
	const name = ctx.mode?.startsWith("perf-") ? `orpc-api-go-${ctx.mode}` : "orpc-api-go";
	return {
		worker: {
			name,
			compatibilityDate: "2026-09-25",
			entrypoint,
			exports: {
				NotesHub: exports.durableObject({ storage: "sqlite" }),
			},
			observability: { enabled: true },
			env: {
				APP_NAME: bindings.text(name),
				DB: bindings.d1(),
				HUB: bindings.durableObject({ worker: name, exportName: "NotesHub" }),
			},
		},
	};
});
