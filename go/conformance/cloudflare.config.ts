import { bindings, defineConfig, exports } from "cf/config";
import * as entrypoint from "./worker.mjs" with { type: "cf-worker" };
import spec from "./fern/openapi.json" with { type: "json" };
import { optionalSecrets } from "./secrets.ts";

// Who else may call (api/handlers.go: trusted): Cloudflare Access (ACCESS_TEAM_DOMAIN, ACCESS_AUD) and
// an OpenID Connect issuer (OIDC_ISSUER, OIDC_AUDIENCE), declared where the environment that deploys or
// runs the Worker has them. Unset, the Worker trusts no one that way.
const optional = Object.fromEntries(optionalSecrets.filter(name => process.env[name]).map(name => [name, bindings.secret()]));

// How often a caller may call (go/lib/ratelimit): each limit the contract declares (x-rate-limit in
// fern/openapi.json) is a Workers Rate Limiting binding with the same limit and period. Its namespace
// is the counter's id, unique in the Cloudflare account (the notes Workers have 1001 and 1002).
const namespaces: Record<string, string> = { WRITE_LIMIT: "1003" };
type Limit = { binding: string; limit: number; period: 10 | 60 };
const limits = Object.fromEntries((Object.values(spec.paths) as Record<string, { "x-rate-limit"?: Limit }>[])
	.flatMap(item => Object.values(item)).flatMap(op => op["x-rate-limit"] ?? [])
	.map(({ binding, limit, period }) => {
		const namespace = namespaces[binding];
		if (!namespace) throw new Error(`cloudflare.config.ts: the rate limit ${binding} has no namespace`);
		return [binding, bindings.rateLimit({ namespace, simple: { limit, period } })];
	}));

// The conformance Worker: every Fern feature we use, served by Go (api/) on workers-go, built
// with TinyGo. The notes are in D1 (migrations/), and Hub, a hibernating Durable Object, wakes the
// WebSocket's feed on every new one, as in the notes example (docs/guides/streaming.md).
export default defineConfig({
	worker: {
		name: "charter-conformance-go",
		compatibilityDate: "2026-09-25",
		entrypoint,
		exports: {
			Hub: exports.durableObject({ storage: "sqlite" }),
		},
		observability: { enabled: true },
		env: {
			DB: bindings.d1(),
			HUB: bindings.durableObject({ worker: "charter-conformance-go", exportName: "Hub" }),
			// The API's tokens (api/handlers.go): set by every npm run deploy, from the environment under cf dev.
			READ_TOKEN: bindings.secret(),
			WRITE_TOKEN: bindings.secret(),
			...limits,
			...optional,
			// Where the noteCreated webhook is sent; empty: nowhere. The other settings (CLIENT_ID,
			// CLIENT_SECRET, TOKEN_SECRET, WEBHOOK_SECRET) have defaults for trying it out
			// (conformance/handlers.go): a real deployment sets them as secrets.
			WEBHOOK_URL: bindings.text(process.env.CONFORMANCE_WEBHOOK_URL ?? ""),
		},
	},
});
