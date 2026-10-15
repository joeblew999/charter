// What a project's cloudflare.config.ts takes from charter: its Workers Rate Limiting bindings, one for
// each limit its spec declares (x-rate-limit: go/lib/ratelimit, charter-lib/ratelimit), with the same
// limit and period, each counting in a namespace named from the Worker's name.
//   env: { ...rateLimits(spec, name, bindings.rateLimit) }

/** A limit as the spec declares it. */
type Limit = { binding: string; limit: number; period: 10 | 60 };

/** What cf/config's bindings.rateLimit takes. */
export interface RateLimitOptions {
	namespace: string;
	simple: { limit: number; period: 10 | 60 };
}

/**
 * The counter of a Worker's binding: a positive integer, as Cloudflare has it, which must be unique in
 * the account. It is named from the Worker and the binding (FNV-1a), so no project numbers its own:
 * two Workers never share a counter by mistake, and a Worker keeps its counter across deploys.
 */
export function namespaceOf(worker: string, binding: string): string {
	let hash = 0x811c9dc5;
	for (const byte of new TextEncoder().encode(`${worker}/${binding}`)) hash = Math.imul(hash ^ byte, 0x01000193) >>> 0;
	return String((hash % 1_000_000_000) + 1);
}

/** Each limit the spec declares, as a binding of its name: rateLimit is cf/config's bindings.rateLimit. */
export function rateLimits<B>(spec: { paths?: object }, worker: string, rateLimit: (options: RateLimitOptions) => B): Record<string, B> {
	const declared = Object.values(spec.paths ?? {})
		.flatMap(item => Object.values(item as Record<string, { "x-rate-limit"?: Limit }>))
		.flatMap(op => (op && typeof op === "object" && op["x-rate-limit"]) || []);
	return Object.fromEntries(declared.map(({ binding, limit, period }) => [binding, rateLimit({ namespace: namespaceOf(worker, binding), simple: { limit, period } })]));
}
