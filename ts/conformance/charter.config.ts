// What charter-ts needs to know about this project (npm run <name> runs charter-ts <name>), typed, in one place.
import { defineConfig } from "charter-ts/config";
import { sdk } from "charter-ts/sdk";
import { optionalSecrets, secrets } from "./secrets.ts";
import { specs } from "./api/spec.ts";

/** The second Worker, the same code deployed with --mode api, which serves the API the specs describe:
 * on Cloudflare a Worker can't open a WebSocket to itself. On another account: set API_SERVER_URL, then npm run spec. */
const apiUrl = process.env.API_SERVER_URL ?? "https://charter-conformance-ts-api.gedw99.workers.dev";

export default defineConfig({
	/** The Worker, as in cloudflare.config.ts. */
	worker: "charter-conformance-ts",
	/** Where it is deployed (cf deploy prints it). On another account: set API_URL. */
	url: process.env.API_URL ?? "https://charter-conformance-ts.gedw99.workers.dev",
	/** npm run dev's port. */
	port: Number(process.env.API_PORT ?? 5176),
	/** The Worker's secrets (secrets.ts, which cloudflare.config.ts reads too): fnox exec -- npm run deploy. */
	secrets,
	optionalSecrets,
	/** Throwaway tokens for the local Worker and its tests. */
	localSecrets: { READ_TOKEN: "local-read-token", WRITE_TOKEN: "local-write-token" },
	/** The SDKs Fern makes (npm run spec writes fern/generators.yml from this and the spec). */
	sdks: { name: "Conformance", more: ["python", "cli"], audiences: ["public"] },
	/** Both specs, from the contract (api/contract.ts), naming the API's Worker. */
	specs,
	/** The tables of migrations/ as Drizzle has them (api/tables.ts). */
	db: "drizzle",
	specServer: apiUrl,
	/** The Worker runs the generated SDK itself: it is copied into client/ first. */
	sdkInWorker: "client",
	deployModes: ["api"],
	/** npm run check, under workerd: the SDK test inside the Worker, then from Node the SDK, MCP and every way in,
	 * the tests go/conformance runs too (the public SDK made first if fern/ changed: Docker). The tests
	 * serve the auth test's issuer of tokens (extras[0]), which the Worker trusts as Cloudflare Access
	 * and as an OpenID Connect issuer, and the webhook's receiver (extras[1]), each on a port of its own. */
	test: {
		env: ({ extras: [issuer, webhook] }) => ({
			WEBHOOK_URL: `http://localhost:${webhook}/webhook`,
			ACCESS_TEAM_DOMAIN: `http://localhost:${issuer}`,
			ACCESS_AUD: "test-aud",
			OIDC_ISSUER: `http://localhost:${issuer}`,
			OIDC_AUDIENCE: "https://conformance.test",
		}),
		/** The webhook's receiver, for the Worker's whole run: every note the tests create is delivered. */
		serve: ({ extras: [, webhook] }) => [{ program: "node", args: ["test/webhook-receiver.ts", String(webhook)], ready: `http://localhost:${webhook}/` }],
		run: ({ origin, extras: [issuer, webhook], node }) => {
			node("test/sdk-in-worker.ts", origin);
			sdk("typescript-public", { ifStale: true });
			node("test/sdk.ts", origin, "--webhooks", `http://localhost:${webhook}`);
			node("test/mcp.ts", origin);
			node("test/auth.ts", origin, String(issuer));
		},
	},
	/** npm run live-test (fnox exec -- npm run live-test): the SDK test inside the deployed Worker, against
	 * the second one, then the SDK and MCP from here against that one (never the auth test: its issuer
	 * is local). It writes test notes. */
	live: ({ url, node }) => {
		node("test/sdk-in-worker.ts", url, apiUrl);
		sdk("typescript-public", { ifStale: true });
		node("test/sdk.ts", apiUrl);
		node("test/mcp.ts", apiUrl);
		node("test/ratelimit.ts", apiUrl);
	},
});
