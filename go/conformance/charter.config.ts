// What charter-ts needs to know about this project (npm run <name> runs charter-ts <name>), typed, in one place.
import { defineConfig, type Extras, type TestContext, type Tests } from "charter-ts/config";
import { sdk } from "charter-ts/sdk";
import { optionalSecrets, secrets } from "./secrets.ts";

/** The tests ts/conformance has, against this server: the same API, so the same tests. The SDK test
 * runs the TypeScript SDKs made from this contract's specs (made first if fern/ changed: Docker); then
 * MCP and every way in. The tests serve the auth test's issuer of tokens (extras[0]) and the webhook's
 * receiver (extras[1]), each on a port of its own. */
const tests = "../../ts/conformance/test";
const sdks = () => {
	sdk("typescript-dist", { ifStale: true });
	sdk("typescript-public", { ifStale: true });
};
const conformance = ({ origin, extras: [issuer, webhook], node }: TestContext) => {
	sdks();
	node(`${tests}/sdk.ts`, origin, "--webhooks", `http://localhost:${webhook}`);
	node(`${tests}/mcp.ts`, origin);
	node(`${tests}/auth.ts`, origin, String(issuer));
};
/** The webhook's receiver, for the server's whole run: every note the tests create is delivered. */
const receiver = ({ extras: [, webhook] }: { extras: Extras }) => [{ program: "node", args: [`${tests}/webhook-receiver.ts`, String(webhook)], ready: `http://localhost:${webhook}/` }];

/** Whom the server trusts besides its OAuth clients and its tokens: the auth test's issuer, as
 * Cloudflare Access and as an OpenID Connect issuer. */
const trusting = (issuer: number) => ({
	ACCESS_TEAM_DOMAIN: `http://localhost:${issuer}`,
	ACCESS_AUD: "test-aud",
	OIDC_ISSUER: `http://localhost:${issuer}`,
	OIDC_AUDIENCE: "https://conformance.test",
});

/** The tests and what they serve, natively and under workerd alike. */
const served: Tests = { env: ({ extras: [issuer, webhook] }) => ({ WEBHOOK_URL: `http://localhost:${webhook}/webhook`, ...trusting(issuer) }), serve: receiver, run: conformance };

export default defineConfig({
	/** The Worker, as in cloudflare.config.ts. */
	worker: "charter-conformance-go",
	/** Where it is deployed (cf deploy prints it). On another account: set API_URL, then npm run spec. */
	url: process.env.API_URL ?? "https://charter-conformance-go.gedw99.workers.dev",
	/** npm run dev's port. */
	port: Number(process.env.API_PORT ?? 5175),
	/** The Worker's secrets (secrets.ts, which cloudflare.config.ts reads too): fnox exec -- npm run deploy. */
	secrets,
	optionalSecrets,
	/** Throwaway tokens for the local server and its tests. */
	localSecrets: { READ_TOKEN: "local-read-token", WRITE_TOKEN: "local-write-token" },
	/** Both specs, from the contract (api/contract.go). */
	specs: { go: "./cmd/spec" },
	go: {},
	/** npm run check, natively and under workerd: the generated SDKs against the API, with a webhook
	 * receiver the test serves on the extra port. */
	native: served,
	test: served,
	/** npm run live-test (fnox exec -- npm run live-test): the SDK and MCP against the deployed Worker
	 * (never the auth test: its issuer is local). It writes test notes. */
	live: ({ url, node }) => {
		sdks();
		node(`${tests}/sdk.ts`, url);
		node(`${tests}/mcp.ts`, url);
		node(`${tests}/ratelimit.ts`, url);
	},
});
