import { bindings, defineConfig } from "cf/config";
import * as entrypoint from "./worker.mjs" with { type: "cf-worker" };

// The showcase Worker: every Fern feature we use, served by Go (api/) on workers-go, built
// with TinyGo. It has no storage.
export default defineConfig({
	worker: {
		name: "charter-showcase-go",
		compatibilityDate: "2026-09-25",
		entrypoint,
		observability: { enabled: true },
		env: {
			// Where the noteCreated webhook is sent; empty: nowhere. The other settings (CLIENT_ID,
			// CLIENT_SECRET, TOKEN_SECRET, WEBHOOK_SECRET) have defaults for trying it out
			// (showcase/handlers.go): a real deployment sets them as secrets.
			WEBHOOK_URL: bindings.text(process.env.SHOWCASE_WEBHOOK_URL ?? ""),
		},
	},
});
