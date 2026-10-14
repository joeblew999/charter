// What charter-ts needs to know about this project (npm run <step> runs charter-ts <step>), typed, in one place.
import { accessScheme, oidcScheme, scheme } from "charter-lib/auth";
import { defineConfig } from "charter-ts/config";
import { sdk } from "charter-ts/scripts";
import { asyncapiSpec, openapiSpec } from "charter-lib/specs";
import { optionalSecrets, secrets } from "./secrets.ts";
import { asyncInfo, contract, info } from "./src/contract.ts";

// Who may call: bearer tokens, Cloudflare Access and an OpenID Connect issuer (src/index.ts).
const base = oidcScheme(accessScheme(info.title, scheme()));

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
	/** Both specs, with the same generators the Worker serves /api/openapi.json and /api/asyncapi.json with. */
	specs: {
		openapi: server => openapiSpec(contract, { info, server, base }),
		asyncapi: server => asyncapiSpec(contract, { info: asyncInfo, server }),
	},
	/** npm run test:workerd: the live test, then the auth test, which serves a test issuer of tokens on
	 * the extra port: the Worker trusts it as Cloudflare Access and as an OpenID Connect issuer. */
	test: {
		env: ({ extra }) => ({
			ACCESS_TEAM_DOMAIN: `http://localhost:${extra}`,
			ACCESS_AUD: "test-aud",
			OIDC_ISSUER: `http://localhost:${extra}`,
			OIDC_AUDIENCE: "https://notes.test",
		}),
		run: ({ origin, extra, node }) => {
			node("test/live-test.ts", origin);
			node("test/auth-test.ts", origin, String(extra));
		},
	},
	/** npm run live-test (fnox exec -- npm run live-test): the live test and the generated TypeScript SDK
	 * (made first if the specs are newer: Docker) against the deployed Worker. It writes test notes. */
	live: ({ url, node }) => {
		sdk("typescript-dist", { ifStale: true });
		node("test/live-test.ts", url);
		node("test/sdk-live-test.ts", url);
	},
});
