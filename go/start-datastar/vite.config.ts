import { cloudflare } from "@cloudflare/vite-plugin";
import { defineConfig } from "vite";

export default defineConfig({
	server: { port: Number(process.env.PORT) || 5173, strictPort: true },
	plugins: [cloudflare()],
});
