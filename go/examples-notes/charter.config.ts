// What charter-ts needs to know about this project (npm run <name> runs charter-ts <name>), typed, in one place.
import { defineConfig, type TestContext } from "charter-ts/config";
import { sdk } from "charter-ts/sdk";
import { optionalSecrets, secrets } from "./secrets.ts";

// The notes API is the same in Go and in TypeScript, so its tests (notes, below) are the TypeScript
// project's, typed; charter's repo holds the two to the same answers (charter-ts twins). Auth, rate
// limits and MCP are go/lib/serve's, which charter's conformance projects test every way.
const notes = "../../ts/examples-notes/test";
const live = ({ origin, node }: Pick<TestContext, "origin" | "node">) => node(`${notes}/live.ts`, origin);

export default defineConfig({
	/** The Worker, as in cloudflare.config.ts. */
	worker: "charter-notes-go",
	/** Where it is deployed (cf deploy prints it). On another account: set API_URL, then npm run spec. */
	url: process.env.API_URL ?? "https://charter-notes-go.gedw99.workers.dev",
	/** npm run dev's port. */
	port: Number(process.env.API_PORT ?? 5174),
	/** The Worker's secrets (secrets.ts, which cloudflare.config.ts reads too): fnox exec -- npm run deploy. */
	secrets,
	optionalSecrets,
	/** Throwaway tokens for the local API and its tests. */
	localSecrets: { READ_TOKEN: "local-read-token", WRITE_TOKEN: "local-write-token" },
	/** The SDKs Fern makes (npm run spec writes fern/generators.yml from this and the spec). */
	sdks: { name: "Notes", more: ["python", "cli"] },
	/** Both specs, from the contract (api/contract.go). */
	specs: { go: "./cmd/spec" },
	go: { lint: ["api", "cmd", "test"], sdk: true },
	/** npm run check, natively and under workerd: the real-time live test. */
	native: { run: live },
	test: { run: live },
	/** npm run live-test (fnox exec -- npm run live-test): against the deployed Worker, through the
	 * generated TypeScript SDK too (made first if the specs are newer: Docker). It writes test notes. */
	live: ({ url, node }) => {
		sdk("typescript-dist", { ifStale: true });
		node(`${notes}/live.ts`, url);
		node(`${notes}/sdk.ts`, url);
	},
});
