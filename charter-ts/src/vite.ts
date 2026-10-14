// The Vite config every project's vite.config.ts is: export { default } from "charter-ts/vite".
// cf dev and cf deploy run the Worker through Vite with Cloudflare's plugin, on the port charter-ts
// gives (PORT). One copy of oRPC and Zod in the Worker, wherever charter-ts is installed from.
import { cloudflare } from "@cloudflare/vite-plugin";
import { defineConfig } from "vite";

export default defineConfig({
	resolve: { dedupe: ["@orpc/contract", "@orpc/json-schema", "@orpc/openapi", "@orpc/server", "@orpc/zod", "zod"] },
	server: { port: Number(process.env.PORT) || 5173, strictPort: true },
	plugins: [cloudflare()],
});
