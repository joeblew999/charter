// What charter-ts needs to know about this project (npm run <name> runs charter-ts <name>), typed, in one place.
import { defineConfig } from "charter-ts/config";
import { specs } from "./api/spec.ts";

export default defineConfig({
	/** The Worker, as in cloudflare.config.ts. */
	worker: "charter-start-ts",
	/** Where it is deployed (cf deploy prints it). On another account: set API_URL, then npm run spec. */
	url: process.env.API_URL ?? "https://charter-start-ts.gedw99.workers.dev",
	/** npm run dev's port. */
	port: Number(process.env.API_PORT ?? 5178),
	/** The SDKs Fern makes (npm run spec writes fern/generators.yml from this and the spec). */
	sdks: { name: "Api" },
	/** Both specs, with the same generators the Worker serves /api/openapi.json and /api/asyncapi.json with. */
	specs,
	/** npm run check: the live test against the Worker under workerd. */
	test: { run: ({ origin, node }) => node("test/live.ts", origin) },
	/** npm run live-test: the same against the deployed Worker. */
	live: ({ url, node }) => node("test/live.ts", url),
});
