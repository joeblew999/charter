// How often a caller may call an oRPC API's operations, the counterpart of go/ratelimit, beside
// auth: the contract declares each limit, and one check enforces it, so the specs, the SDKs and the
// server agree.
//
// A limit is a Cloudflare Workers Rate Limiting binding (cloudflare.config.ts declares it, with the
// same limit and period, from the spec), counted per caller: the key is who `authorize` let in (the
// service token, the subject, the email or the token's secret), or the client's IP
// (CF-Connecting-IP) for an operation that is public. An operation has a limit of its own
// (`limited`, in its `spec`), or the limit of a scope it needs (openapiSpec's `rateLimits`, and the
// same list to `limitOf`). Over it: 429, with Retry-After the binding's period, and the specs say so.
//
// The binding is a fixed window per key, local to each Cloudflare location and eventually
// consistent: good for stopping abuse, not for accounting
// (https://developers.cloudflare.com/workers/runtime-apis/bindings/rate-limit/). A Worker without
// the binding lets every call through and logs it.
import { getOpenAPIMeta, type OpenAPIV3_2 } from "@orpc/openapi";
import type { Caller, Security } from "./auth.ts";

/** Where an operation's limit is in the specs. */
export const EXTENSION = "x-rate-limit";

/** One Rate Limiting binding: at most `limit` calls per caller in each `period`. */
export interface Limit {
	binding: string; // the Worker's binding that counts, e.g. WRITE_LIMIT
	limit: number; // calls per caller in a period
	period: 10 | 60; // seconds, all the binding takes
	scope?: string; // openapiSpec's rateLimits: every operation that needs this scope
}

type Operation = OpenAPIV3_2.OperationObject;

/** An operation's `spec`: its own limit. Wraps another spec, e.g. Fern's names, and goes inside `needs`. */
export const limited = (limit: Limit, spec: (op: Operation) => Operation = op => op) =>
	(op: Operation): Operation => withLimit(spec(op), limit);

/** An operation with its limit and its 429 (openapiSpec applies the scopes' limits with it). */
export const withLimit = (op: Operation, limit: Limit): Operation => ({
	...op,
	[EXTENSION]: limit,
	responses: {
		...op.responses,
		"429": {
			description: `Too Many Requests: more than ${limit.limit} calls in ${limit.period} s by this caller`,
			headers: { "Retry-After": { description: "Seconds to wait before calling again", schema: { type: "integer" } } },
			content: { "application/json": { schema: { type: "object", properties: { code: { type: "string" }, status: { type: "integer" }, message: { type: "string" } } } } },
		},
	},
} as Operation);

/** Whether every alternative of a security list needs the scope. */
export const needsScope = (security: Security | undefined, scope: string | undefined) =>
	!!scope && !!security?.length && security.every(alternative => Object.values(alternative).some(scopes => scopes.includes(scope)));

/** The limit a procedure's contract declares, or else that of the first of perScope whose scope its security needs. */
export function limitOf(procedure: unknown, perScope: Limit[] = [], defaults?: Security): Limit | undefined {
	const spec = getOpenAPIMeta(procedure as never)?.spec;
	const op = (typeof spec === "function" ? spec({} as Operation) : spec) as (Operation & { [EXTENSION]?: Limit }) | undefined;
	if (op?.[EXTENSION]) return op[EXTENSION];
	const security = (op?.security as Security | undefined) ?? defaults;
	return perScope.find(l => needsScope(security, l.scope));
}

/** Whom a call counts against: the caller, by the most specific thing that names it, or else the client's IP. */
export function keyOf(caller: Caller | undefined, headers: Headers): string {
	if (caller?.machine) return `machine:${caller.machine}`;
	if (caller?.subject) return `${caller.scheme}:${caller.subject}`;
	if (caller?.email) return `email:${caller.email}`;
	if (caller?.token) return `token:${caller.token}`;
	if (caller) return `${caller.scheme}:${caller.scopes.join(" ")}`;
	return `ip:${headers.get("cf-connecting-ip") ?? "unknown"}`;
}

/** The binding's API (the generated Env types it as RateLimit). */
export interface RateLimiter {
	limit(options: { key: string }): Promise<{ success: boolean }>;
}

export type Verdict = { status: 200 } | { status: 429; retryAfter: number; message: string };

const warned = new Set<string>();
const warn = (binding: string, why: string) => {
	if (!warned.has(binding + why)) { warned.add(binding + why); console.warn(`ratelimit: ${binding}: ${why}; calls are not limited`); }
};

/**
 * Whether key may call once more under limit, counted by the binding of that name in bindings (the
 * Worker's env): 200, or 429 with the seconds to wait. No limit, no binding, or a binding that
 * fails: 200 (a missing or failing binding never takes the API down; it is logged once).
 */
export async function rateLimit(limit: Limit | undefined, key: string, bindings: object): Promise<Verdict> {
	if (!limit) return { status: 200 };
	const binding = (bindings as Record<string, RateLimiter | undefined>)[limit.binding];
	if (typeof binding?.limit !== "function") {
		warn(limit.binding, "the Worker has no such Rate Limiting binding");
		return { status: 200 };
	}
	try {
		if ((await binding.limit({ key })).success) return { status: 200 };
	} catch (error) {
		warn(limit.binding, String(error));
		return { status: 200 };
	}
	return { status: 429, retryAfter: limit.period, message: `too many calls: at most ${limit.limit} in ${limit.period} s; retry after ${limit.period} s` };
}
