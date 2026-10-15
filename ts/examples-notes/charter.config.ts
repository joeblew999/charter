// What charter-ts needs to know about this project (npm run <name> runs charter-ts <name>), typed, in one place.
import { defineConfig } from "charter-ts/config";
import { sdk } from "charter-ts/sdk";
import { optionalSecrets, secrets } from "./secrets.ts";
import { specs } from "./api/spec.ts";

export default defineConfig({
	/** The Worker, as in cloudflare.config.ts. */
	worker: "charter-notes-ts",
	/** Where it is deployed (cf deploy prints it). On another account: set API_URL, then npm run spec. */
	url: process.env.API_URL ?? "https://charter-notes-ts.gedw99.workers.dev",
	/** npm run dev's port. */
	port: Number(process.env.API_PORT ?? 5173),
	/** The Worker's secrets (secrets.ts, which cloudflare.config.ts reads too): fnox exec -- npm run deploy. */
	secrets,
	optionalSecrets,
	/** Throwaway tokens for the local Worker and its tests. */
	localSecrets: { READ_TOKEN: "local-read-token", WRITE_TOKEN: "local-write-token" },
	/** The SDKs Fern makes (npm run spec writes fern/generators.yml from this and the spec). */
	sdks: { name: "Notes", more: ["python", "cli"] },
	/** Both specs, with the same generators the Worker serves /api/openapi.json and /api/asyncapi.json with. */
	specs,
	/** npm run check, under workerd: the live test. Auth, rate limits and MCP are charter-lib/serve's, which the
	 * conformance projects test every way. */
	test: { run: ({ origin, node }) => node("test/live.ts", origin) },
	/** npm run live-test (fnox exec -- npm run live-test): the live test and the generated TypeScript SDK
	 * (made first if the specs are newer: Docker) against the deployed Worker. It writes test notes. */
	live: ({ url, node }) => {
		sdk("typescript-dist", { ifStale: true });
		node("test/live.ts", url);
		node("test/sdk.ts", url);
	},
});
