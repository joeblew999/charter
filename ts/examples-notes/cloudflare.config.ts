import { bindings, defineConfig, exports } from "cf/config";
import * as entrypoint from "./src/index.ts" with { type: "cf-worker" };
import spec from "./fern/openapi.json" with { type: "json" };
import { rateLimits } from "charter-ts/cloudflare";
import { optionalSecrets } from "./secrets.ts";

// The oRPC API (src/contract.ts) on D1, plus Hub: a hibernating Durable Object that fans every
// new note out to the Worker's follow() loops. The Worker, not the hub, holds the clients' SSE
// streams and WebSockets (docs/guides/streaming.md).

// Who else may call (charter-lib/auth): Cloudflare Access (ACCESS_TEAM_DOMAIN, ACCESS_AUD) and an
// OpenID Connect issuer (OIDC_ISSUER, OIDC_AUDIENCE): the Worker's optional secrets (secrets.ts),
// declared where the environment that deploys or runs the Worker has them; scripts/deploy.ts sets
// them, cf dev takes them from the environment. Unset, the Worker trusts no one that way.
declare const process: { env: Record<string, string | undefined> };
const optional = Object.fromEntries(optionalSecrets.filter(name => process.env[name]).map(name => [name, bindings.secret()]));

// How often a caller may call (go/lib/ratelimit, charter-lib/ratelimit): each limit the contract
// declares (x-rate-limit in fern/openapi.json) is a Workers Rate Limiting binding with the same limit
// and period, counting in a namespace named from the Worker's name (charter-ts/cloudflare).

export default defineConfig({
	worker: {
		name: "charter-notes-ts",
		compatibilityDate: "2026-09-25",
		entrypoint,
		exports: {
			Hub: exports.durableObject({ storage: "sqlite" }),
		},
		// Workers Logs is off unless enabled.
		observability: { enabled: true },
		env: {
			APP_NAME: bindings.text("charter-notes-ts"),
			DB: bindings.d1(),
			HUB: bindings.durableObject({ worker: "charter-notes-ts", exportName: "Hub" }),
			// The tokens (src/index.ts): set by every npm run deploy, from the environment under cf dev.
			READ_TOKEN: bindings.secret(),
			WRITE_TOKEN: bindings.secret(),
			...rateLimits(spec, "charter-notes-ts", bindings.rateLimit),
			...optional,
		},
	},
});
