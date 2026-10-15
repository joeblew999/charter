import { bindings, defineConfig, exports } from "cf/config";
import * as entrypoint from "./worker.mjs" with { type: "cf-worker" };
import spec from "./fern/openapi.json" with { type: "json" };
import { optionalSecrets } from "./secrets.ts";

// The Go Worker: the notes API, written in Go on workers-go and built with TinyGo.
// D1 is the log; Hub is the hibernating live fan-out: the library's Hub class (build/hub.mjs),
// which worker.mjs exports.
// A mode that starts with "perf-" deploys a scratch Worker of that name, with a
// database and a hub of its own.

// Who else may call (go/auth, charter-lib/auth): Cloudflare Access (ACCESS_TEAM_DOMAIN, ACCESS_AUD)
// and an OpenID Connect issuer (OIDC_ISSUER, OIDC_AUDIENCE): the Worker's optional secrets
// (secrets.ts), declared where the environment that deploys or runs the Worker has them; npm run
// deploy sets them, cf dev takes them from the environment. Unset, the Worker trusts no one that way.
declare const process: { env: Record<string, string | undefined> };
const optional = Object.fromEntries(optionalSecrets.filter(name => process.env[name]).map(name => [name, bindings.secret()]));

// How often a caller may call (go/ratelimit, @charter/ts/ratelimit): each limit the contract declares
// (ratelimit.On in api/contract.go; x-rate-limit in fern/openapi.json) is a Workers Rate Limiting
// binding with the same limit and period. Its namespace is the counter's id, unique in the
// Cloudflare account.
const namespaces: Record<string, string> = { WRITE_LIMIT: "1001" };
type Limit = { binding: string; limit: number; period: 10 | 60 };
const limits = Object.fromEntries((Object.values(spec.paths) as Record<string, { "x-rate-limit"?: Limit }>[])
	.flatMap(item => Object.values(item)).flatMap(op => op["x-rate-limit"] ?? [])
	.map(({ binding, limit, period }) => {
		const namespace = namespaces[binding];
		if (!namespace) throw new Error(`cloudflare.config.ts: the rate limit ${binding} has no namespace`);
		return [binding, bindings.rateLimit({ namespace, simple: { limit, period } })];
	}));

export default defineConfig(ctx => {
	const name = ctx.mode?.startsWith("perf-") ? `charter-notes-go-${ctx.mode}` : "charter-notes-go";
	return {
		worker: {
			name,
			compatibilityDate: "2026-09-25",
			entrypoint,
			exports: {
				Hub: exports.durableObject({ storage: "sqlite" }),
			},
			observability: { enabled: true },
			env: {
				APP_NAME: bindings.text(name),
				DB: bindings.d1(),
				HUB: bindings.durableObject({ worker: name, exportName: "Hub" }),
				// The tokens (api/handlers.go): set by every npm run deploy, from the environment under cf dev.
				READ_TOKEN: bindings.secret(),
				WRITE_TOKEN: bindings.secret(),
				...limits,
				...optional,
			},
		},
	};
});
