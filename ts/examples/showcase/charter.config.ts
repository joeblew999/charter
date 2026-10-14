// What charter-ts needs to know about this project (npm run <step> runs charter-ts <step>), typed, in one place.
import { defineConfig } from "charter-ts/config";
import { specs } from "./src/specs.ts";

/** The second Worker, the same code deployed with --mode api, which serves the API the specs describe:
 * on Cloudflare a Worker can't open a WebSocket to itself. On another account: set API_MOCK_URL. */
const mockUrl = process.env.API_MOCK_URL ?? "https://charter-showcase-ts-api.gedw99.workers.dev";

export default defineConfig({
	/** The Worker, as in cloudflare.config.ts. */
	worker: "charter-showcase-ts",
	/** Where it is deployed (cf deploy prints it). On another account: set API_URL. */
	url: process.env.API_URL ?? "https://charter-showcase-ts.gedw99.workers.dev",
	/** npm run dev's port. */
	port: Number(process.env.API_PORT ?? 5176),
	specs,
	specServer: `${mockUrl}/api/mock`,
	/** The Worker runs the generated SDK itself: it is copied into src/client first. */
	sdkInWorker: "src/client",
	deployModes: ["api"],
	/** npm run test:workerd: the SDK test inside the Worker, then the SDK's WebSocket client from Node. */
	test: {
		ready: "/",
		run: ({ origin, node }) => {
			node("test/harness-test.ts", origin);
			node("test/ws-client.ts", origin);
		},
	},
	/** npm run live-test: the same inside the deployed Worker, against the second one. */
	live: ({ url, node }) => {
		node("test/harness-test.ts", url, mockUrl);
		node("test/ws-client.ts", url);
	},
});
