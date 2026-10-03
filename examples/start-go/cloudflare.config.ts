import { bindings, defineConfig } from "cf/config";
import * as entrypoint from "./worker.mjs" with { type: "cf-worker" };

// The Go Worker, written in Go on workers-go and built with TinyGo, with a D1 database (DB).
// A mode that starts with "perf-" (charter perf) deploys a scratch Worker of that name, with a
// database of its own.
export default defineConfig(ctx => {
	const name = ctx.mode?.startsWith("perf-") ? `charter-start-go-${ctx.mode}` : "charter-start-go";
	return {
		worker: {
			name,
			compatibilityDate: "2026-09-25",
			entrypoint,
			observability: { enabled: true },
			env: {
				APP_NAME: bindings.text(name),
				DB: bindings.d1(),
			},
		},
	};
});
