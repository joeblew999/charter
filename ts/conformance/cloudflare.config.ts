import { bindings, defineConfig, exports } from "cf/config";
import * as entrypoint from "./src/index.ts" with { type: "cf-worker" };
import spec from "./fern/openapi.json" with { type: "json" };
import { optionalSecrets } from "./secrets.ts";

// Who else may call (src/conformance.ts: trusted): Cloudflare Access (ACCESS_TEAM_DOMAIN, ACCESS_AUD)
// and an OpenID Connect issuer (OIDC_ISSUER, OIDC_AUDIENCE), declared where the environment that
// deploys or runs the Worker has them. Unset, the Worker trusts no one that way.
declare const process: { env: Record<string, string | undefined> };
const optional = Object.fromEntries(optionalSecrets.filter(name => process.env[name]).map(name => [name, bindings.secret()]));

// How often a caller may call (charter-lib/ratelimit): each limit the contract declares (limited() in
// src/contract.ts; x-rate-limit in fern/openapi.json) is a Workers Rate Limiting binding with the same
// limit and period. Its namespace is the counter's id, unique in the Cloudflare account (the notes
// Workers have 1001 and 1002); both of this project's Workers count in it.
const namespaces: Record<string, string> = { WRITE_LIMIT: "1004" };
type Limit = { binding: string; limit: number; period: 10 | 60 };
const limits = Object.fromEntries((Object.values(spec.paths) as Record<string, { "x-rate-limit"?: Limit }>[])
	.flatMap(item => Object.values(item)).flatMap(op => op["x-rate-limit"] ?? [])
	.map(({ binding, limit, period }) => {
		const namespace = namespaces[binding];
		if (!namespace) throw new Error(`cloudflare.config.ts: the rate limit ${binding} has no namespace`);
		return [binding, bindings.rateLimit({ namespace, simple: { limit, period } })];
	}));

// The SDK test Worker. The same code also deploys as a second Worker, charter-conformance-ts-api (`--mode api`):
// on Cloudflare the WebSocket test connects from charter-conformance-ts to charter-conformance-ts-api, which needs the
// global_fetch_strictly_public flag (without it: error 1042, Worker-to-Worker on the same zone).
// Each keeps its notes in its own D1 (migrations/), and ConformanceHub, a hibernating Durable
// Object, wakes the WebSocket's feed on every new one, as in the notes example (docs/guides/streaming.md).
export default defineConfig(ctx => {
	const name = ctx.mode === "api" ? "charter-conformance-ts-api" : "charter-conformance-ts";
	return {
		worker: {
			name,
			compatibilityDate: "2026-09-25",
			// Lets this Worker call another Worker on the same workers.dev zone (else Cloudflare error 1042).
			compatibilityFlags: ["global_fetch_strictly_public"],
			entrypoint,
			assets: {
				notFoundHandling: "single-page-application",
				// The API, and what a client reads to find out how to get in (/.well-known/).
				runWorkerFirst: ["/api/*", "/.well-known/*"],
			},
			exports: {
				ConformanceHub: exports.durableObject({ storage: "sqlite" }),
			},
			// Workers Logs is off unless enabled.
			observability: { enabled: true },
			env: {
				APP_NAME: bindings.text("charter-conformance-ts"),
				// Where the noteCreated webhook is sent; empty: nowhere. WEBHOOK_SECRET (a secret, set where
				// the environment has it) signs it, "whsec" by default for trying it out.
				WEBHOOK_URL: bindings.text(process.env.CONFORMANCE_WEBHOOK_URL ?? ""),
				ASSETS: bindings.assets(),
				DB: bindings.d1(),
				HUB: bindings.durableObject({ worker: name, exportName: "ConformanceHub" }),
				// The API's tokens (src/conformance.ts): set by every npm run deploy, from the environment under cf dev.
				READ_TOKEN: bindings.secret(),
				WRITE_TOKEN: bindings.secret(),
				...limits,
				...optional,
			},
		},
	};
});
