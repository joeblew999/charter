// What charter-ts needs to know about this project (npm run <name> runs charter-ts <name>), typed, in one place.
import { defineConfig, type TestContext } from "charter-ts/config";

// The start API is the same in Go and in TypeScript, so its test is the TypeScript project's, typed.
const both = ({ origin, node }: Pick<TestContext, "origin" | "node">) => node("../../examples/start-ts/test/live.ts", origin);

export default defineConfig({
	/** The Worker, as in cloudflare.config.ts. */
	worker: "charter-start-go",
	/** Where it is deployed (cf deploy prints it). On another account: set API_URL, then npm run spec. */
	url: process.env.API_URL ?? "https://charter-start-go.gedw99.workers.dev",
	/** npm run dev's port. */
	port: Number(process.env.API_PORT ?? 5177),
	/** The SDKs Fern makes (npm run spec writes fern/generators.yml from this and the spec). */
	sdks: { name: "Api" },
	go: {},
	/** npm run check, natively and under workerd: the live test. */
	native: { run: both },
	test: { run: both },
	/** npm run live-test: the same against the deployed Worker. */
	live: ({ url, node }) => both({ origin: url, node }),
});
