import { bindings, defineConfig, exports } from "cf/config";
import * as entrypoint from "./worker.ts" with { type: "cf-worker" };
import spec from "./fern/openapi.json" with { type: "json" };
import { compatibilityDate, rateLimits, secretsFromEnv } from "charter-ts/cloudflare";
import { optionalSecrets } from "./secrets.ts";

// Who else may call (api/handlers.ts: trusted): Cloudflare Access (ACCESS_TEAM_DOMAIN, ACCESS_AUD)
// and an OpenID Connect issuer (OIDC_ISSUER, OIDC_AUDIENCE), declared where the environment that
// deploys or runs the Worker has them. Unset, the Worker trusts no one that way.
declare const process: { env: Record<string, string | undefined> };

// How often a caller may call (charter/ratelimit, charter-lib/ratelimit): each limit the contract
// declares (x-rate-limit in fern/openapi.json) is a Workers Rate Limiting binding with the same limit
// and period, counting in a namespace named from the Worker's name (charter-ts/cloudflare).

// The SDK test Worker. The same code also deploys as a second Worker, charter-conformance-ts-api (`--mode api`):
// on Cloudflare the WebSocket test connects from charter-conformance-ts to charter-conformance-ts-api, which needs the
// global_fetch_strictly_public flag (without it: error 1042, Worker-to-Worker on the same zone).
// Each keeps its notes in its own D1 (migrations/), and Hub, a hibernating Durable
// Object, wakes the WebSocket's feed on every new one, as in the notes example (docs/self/streaming.md).
export default defineConfig(ctx => {
	const name = ctx.mode === "api" ? "charter-conformance-ts-api" : "charter-conformance-ts";
	return {
		worker: {
			name,
			compatibilityDate,
			// Lets this Worker call another Worker on the same workers.dev zone (else Cloudflare error 1042).
			compatibilityFlags: ["global_fetch_strictly_public"],
			entrypoint,
			exports: {
				Hub: exports.durableObject({ storage: "sqlite" }),
			},
			// Workers Logs is off unless enabled.
			observability: { enabled: true },
			env: {
				APP_NAME: bindings.text("charter-conformance-ts"),
				// Where the noteCreated webhook is sent; empty: nowhere. WEBHOOK_SECRET (a secret, set where
				// the environment has it) signs it, "whsec" by default for trying it out.
				WEBHOOK_URL: bindings.text(process.env.WEBHOOK_URL ?? ""),
				DB: bindings.d1(),
				HUB: bindings.durableObject({ worker: name, exportName: "Hub" }),
				// The API's tokens (api/handlers.ts): set by every npm run deploy, from the environment under cf dev.
				READ_TOKEN: bindings.secret(),
				WRITE_TOKEN: bindings.secret(),
				...rateLimits(spec, name, bindings.rateLimit),
				...secretsFromEnv(optionalSecrets, bindings.secret),
			},
		},
	};
});
