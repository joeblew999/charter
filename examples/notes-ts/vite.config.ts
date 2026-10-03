import { cloudflare } from "@cloudflare/vite-plugin";
import { defineConfig } from "vite";

// Where @charter/ts is a linked folder (in the charter repo), its imports would resolve to its own
// node_modules and the Worker would bundle a second oRPC and Zod: take the project's, one copy.
export default defineConfig({
	resolve: { dedupe: ["@orpc/contract", "@orpc/json-schema", "@orpc/openapi", "@orpc/server", "@orpc/zod", "zod"] },
	server: { port: Number(process.env.PORT) || 5173, strictPort: true },
	plugins: [cloudflare()],
});
