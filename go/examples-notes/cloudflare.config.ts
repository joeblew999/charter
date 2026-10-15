import { bindings, defineConfig, exports } from "cf/config";
import * as entrypoint from "./worker.mjs" with { type: "cf-worker" };
import spec from "./fern/openapi.json" with { type: "json" };
import { rateLimits } from "charter-ts/cloudflare";
import { optionalSecrets } from "./secrets.ts";

// The Go Worker: the notes API, written in Go on workers-go and built with TinyGo.
// D1 is the log; Hub is the hibernating live fan-out: the library's Hub class (build/hub.mjs),
// which worker.mjs exports.

// Who else may call (go/lib/auth, charter-lib/auth): Cloudflare Access (ACCESS_TEAM_DOMAIN, ACCESS_AUD)
// and an OpenID Connect issuer (OIDC_ISSUER, OIDC_AUDIENCE): the Worker's optional secrets
// (secrets.ts), declared where the environment that deploys or runs the Worker has them; npm run
// deploy sets them, cf dev takes them from the environment. Unset, the Worker trusts no one that way.
declare const process: { env: Record<string, string | undefined> };
const optional = Object.fromEntries(optionalSecrets.filter(name => process.env[name]).map(name => [name, bindings.secret()]));

// How often a caller may call (go/lib/ratelimit, charter-lib/ratelimit): each limit the contract
// declares (x-rate-limit in fern/openapi.json) is a Workers Rate Limiting binding with the same limit
// and period, counting in a namespace named from the Worker's name (charter-ts/cloudflare).

export default defineConfig(() => {
	const name = "charter-notes-go";
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
				...rateLimits(spec, name, bindings.rateLimit),
				...optional,
			},
		},
	};
});
