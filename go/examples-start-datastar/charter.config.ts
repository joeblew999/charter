// What charter-ts needs to know about this project (npm run <step> runs charter-ts <step>), typed, in one place.
import { defineConfig, type TestContext } from "charter-ts/config";

// The start API is the same in Go and in TypeScript, so its tests are ts/examples-start's, typed.
const both = ({ origin, node }: Pick<TestContext, "origin" | "node">) => {
	node("../../ts/examples-start/test/live-test.ts", origin);
	node("../../ts/examples-start/test/mcp-test.ts", origin);
};

export default defineConfig({
	/** The Worker, as in cloudflare.config.ts. */
	worker: "charter-start-datastar",
	/** Where it is deployed (cf deploy prints it). On another account: set API_URL, then npm run spec. */
	url: process.env.API_URL ?? "https://charter-start-datastar.gedw99.workers.dev",
	/** npm start's and npm run dev's port. */
	port: Number(process.env.API_PORT ?? 5180),
	/** Both specs, from the contract (api/contract.go). */
	specs: { go: "./cmd/spec" },
	go: { lint: ["api", "cmd", "pages"], gsx: true, sdk: true },
	/** npm run test:native and test:workerd: the live test and the MCP client test. */
	native: { run: both },
	test: { run: both },
	/** npm run live-test: the same against the deployed Worker. */
	live: ({ url, node }) => both({ origin: url, node }),
});
