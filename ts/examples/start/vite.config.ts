import { cloudflare } from "@cloudflare/vite-plugin";
import { defineConfig } from "vite";

// One copy of oRPC and Zod in the Worker, the project's, wherever charter-ts is installed from (a
// workspace link in the charter repo).
export default defineConfig({
	resolve: { dedupe: ["@orpc/contract", "@orpc/json-schema", "@orpc/openapi", "@orpc/server", "@orpc/zod", "zod"] },
	server: { port: Number(process.env.PORT) || 5173, strictPort: true },
	plugins: [cloudflare()],
});
