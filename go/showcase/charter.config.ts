// What charter-ts needs to know about this project (npm run <step> runs charter-ts <step>), typed, in one place.
import { defineConfig, type TestContext } from "charter-ts/config";

const showcase = ({ origin, extra, node }: TestContext) => node("test/showcase-test.mjs", origin, "--webhook-port", String(extra));

export default defineConfig({
	/** The Worker, as in cloudflare.config.ts. */
	worker: "charter-showcase-go",
	/** Where it is deployed (cf deploy prints it). On another account: set API_URL, then npm run spec. */
	url: process.env.API_URL ?? "https://charter-showcase-go.gedw99.workers.dev",
	/** npm start's and npm run dev's port. */
	port: Number(process.env.API_PORT ?? 5175),
	/** Both specs, from the contract (api/contract.go). */
	specs: { go: "./cmd/spec" },
	go: { lint: ["api", "cmd"] },
	/** npm run test:native and test:workerd: the generated SDKs against the API, with a webhook
	 * receiver the test serves on the extra port. */
	native: { ready: "/openapi.json", env: ({ extra }) => ({ WEBHOOK_URL: `http://localhost:${extra}/webhook` }), run: showcase },
	test: { ready: "/openapi.json", env: ({ extra }) => ({ SHOWCASE_WEBHOOK_URL: `http://localhost:${extra}/webhook` }), run: showcase },
});
