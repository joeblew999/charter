// What charter-ts needs to know about this project (npm run <step> runs charter-ts <step>), typed, in one place.
import { defineConfig, type TestContext } from "charter-ts/config";
import { sdk } from "charter-ts/scripts";
import { optionalSecrets, secrets } from "./secrets.ts";

const issuer = ({ extra }: { extra: number }) => ({
	ACCESS_TEAM_DOMAIN: `http://localhost:${extra}`,
	ACCESS_AUD: "test-aud",
	OIDC_ISSUER: `http://localhost:${extra}`,
	OIDC_AUDIENCE: "https://notes.test",
});
// The notes API is the same in Go and in TypeScript (test/surface.test.ts holds them to it), so the
// typed notes tests are ts/notes's; the MCP endpoint is Go's own.
const notes = "../../ts/notes/test";
const all = ({ origin, extra, node }: TestContext) => {
	node(`${notes}/live-test.ts`, origin);
	node("test/mcp-test.mjs", origin);
	node(`${notes}/auth-test.ts`, origin, String(extra));
};

export default defineConfig({
	/** The Worker, as in cloudflare.config.ts. */
	worker: "charter-notes-go",
	/** Where it is deployed (cf deploy prints it). On another account: set API_URL, then npm run spec. */
	url: process.env.API_URL ?? "https://charter-notes-go.gedw99.workers.dev",
	/** npm start's and npm run dev's port. */
	port: Number(process.env.API_PORT ?? 5174),
	/** The Worker's secrets (secrets.ts, which cloudflare.config.ts reads too): fnox exec -- npm run deploy. */
	secrets,
	optionalSecrets,
	/** Throwaway tokens for the local API and its tests. */
	localSecrets: { READ_TOKEN: "local-read-token", WRITE_TOKEN: "local-write-token" },
	/** Both specs, from the contract (api/contract.go). */
	specs: { go: "./cmd/spec" },
	go: { lint: ["api", "cmd", "test"], sdk: true },
	/** npm run test:native and test:workerd: the real-time live test, the MCP client test, and the auth
	 * test, which serves a test issuer of tokens on the extra port: the Worker trusts it as Cloudflare
	 * Access and as an OpenID Connect issuer. */
	native: { env: issuer, run: all },
	test: { env: issuer, run: all },
	/** npm run live-test (fnox exec -- npm run live-test): against the deployed Worker, through the
	 * generated TypeScript SDK too (made first if the specs are newer: Docker). It writes test notes. */
	live: ({ url, node }) => {
		sdk("typescript-dist", { ifStale: true });
		node(`${notes}/live-test.ts`, url);
		node(`${notes}/sdk-live-test.ts`, url);
		node("test/mcp-test.mjs", url);
	},
});
