import { bindings, defineConfig, exports } from "cf/config";
import * as entrypoint from "./src/index.ts" with { type: "cf-worker" };
import spec from "./fern/openapi.json" with { type: "json" };

// The oRPC API (src/contract.ts) on D1, plus NotesHub: a hibernating Durable Object that fans every
// new note out to the Worker's follow() loops. The Worker, not the hub, holds the clients' SSE
// streams and WebSockets (docs/guides/streaming.md).

// Who else may call (go/auth, @charter/ts/auth): Cloudflare Access (ACCESS_TEAM_DOMAIN, ACCESS_AUD,
// which mise run access:setup keeps in fnox) and an OpenID Connect issuer (OIDC_ISSUER,
// OIDC_AUDIENCE). They are the Worker's optional secrets (WORKER_OPTIONAL_SECRETS in mise.toml):
// declared when the environment that deploys or runs the Worker has them, and then set by charter
// deploy, or taken from the environment by cf dev. Unset, the Worker trusts no one that way.
declare const process: { env: Record<string, string | undefined> };
const optional = Object.fromEntries((process.env.WORKER_OPTIONAL_SECRETS ?? "").split(" ").filter(name => process.env[name]).map(name => [name, bindings.secret()]));

// How often a caller may call (go/ratelimit, @charter/ts/ratelimit): each limit the contract declares
// (limited() in src/contract.ts; x-rate-limit in fern/openapi.json) is a Workers Rate Limiting
// binding with the same limit and period. Its namespace is the counter's id, unique in the
// Cloudflare account.
const namespaces: Record<string, string> = { WRITE_LIMIT: "1002" };
type Limit = { binding: string; limit: number; period: 10 | 60 };
const limits = Object.fromEntries((Object.values(spec.paths) as Record<string, { "x-rate-limit"?: Limit }>[])
	.flatMap(item => Object.values(item)).flatMap(op => op["x-rate-limit"] ?? [])
	.map(({ binding, limit, period }) => {
		const namespace = namespaces[binding];
		if (!namespace) throw new Error(`cloudflare.config.ts: the rate limit ${binding} has no namespace`);
		return [binding, bindings.rateLimit({ namespace, simple: { limit, period } })];
	}));

export default defineConfig({
	worker: {
		name: "charter-notes-ts",
		compatibilityDate: "2026-09-25",
		entrypoint,
		assets: {
			notFoundHandling: "single-page-application",
			runWorkerFirst: ["/api/*", "/.well-known/*"],
		},
		exports: {
			NotesHub: exports.durableObject({ storage: "sqlite" }),
		},
		// Workers Logs is off unless enabled.
		observability: { enabled: true },
		env: {
			APP_NAME: bindings.text("charter-notes-ts"),
			ASSETS: bindings.assets(),
			DB: bindings.d1(),
			HUB: bindings.durableObject({ worker: "charter-notes-ts", exportName: "NotesHub" }),
			// The tokens (src/index.ts): set by every mise run deploy, from the environment under cf dev.
			READ_TOKEN: bindings.secret(),
			WRITE_TOKEN: bindings.secret(),
			...limits,
			...optional,
		},
	},
});
