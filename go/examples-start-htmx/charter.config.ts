// What charter-ts needs to know about this project (npm run <name> runs charter-ts <name>), typed, in one place.
import { defineConfig, type TestContext } from "charter-ts/config";

// Its test against a server is the TypeScript start project's (the hello route every start project has),
// typed; its messages and pages are tested by go test (api/api_test.go).
const both = ({ origin, node }: Pick<TestContext, "origin" | "node">) => node("../../ts/examples-start/test/live.ts", origin);

export default defineConfig({
	/** The Worker, as in cloudflare.config.ts. */
	worker: "charter-start-htmx",
	/** Where it is deployed (cf deploy prints it). On another account: set API_URL, then npm run spec. */
	url: process.env.API_URL ?? "https://charter-start-htmx.gedw99.workers.dev",
	/** npm run dev's port. */
	port: Number(process.env.API_PORT ?? 5179),
	/** Both specs, from the contract (api/contract.go). */
	specs: { go: "./cmd/spec" },
	go: { lint: ["api", "cmd", "pages"], gsx: true },
	/** npm run check, natively and under workerd: the live test. */
	native: { run: both },
	test: { run: both },
	/** npm run live-test: the same against the deployed Worker. */
	live: ({ url, node }) => both({ origin: url, node }),
});
