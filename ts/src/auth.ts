// Bearer tokens with scopes for an oRPC contract, the counterpart of go/auth: the contract declares
// what each operation needs (its `spec`), and one middleware enforces that, so the specs, the SDKs
// and the server agree.
//
// `scheme()` goes into openapiSpec's `base`. An operation needs the scopes its security lists (the
// API's default when it lists none; nothing when its security is an empty list). Each token is a
// secret of the Worker (READ_TOKEN, WRITE_TOKEN) with the scopes it grants. A request without a token
// it knows gets 401; a known token without a scope the operation needs gets 403. An unset secret
// matches no token, so a Worker without its secrets refuses everything that is not public.
import { getOpenAPIMeta, type OpenAPIV3_2 } from "@orpc/openapi";

/** The security scheme's name in the specs. */
export const NAME = "bearer";

export type Security = Record<string, string[]>[];
type Operation = OpenAPIV3_2.OperationObject;

/** For openapiSpec's `base`: the scheme, and the scopes an operation needs when it says nothing (none: public). */
export const scheme = (...scopes: string[]) => ({
	components: { securitySchemes: { [NAME]: { type: "http" as const, scheme: "bearer" } } },
	...(scopes.length ? { security: [{ [NAME]: scopes }] } : {}),
});

/** An operation's `spec`: it needs a token with all of these scopes. Wraps another spec, e.g. Fern's names. */
export const needs = (scopes: string[], spec: (op: Operation) => Operation = op => op) =>
	(op: Operation): Operation => ({ ...spec(op), security: [{ [NAME]: scopes }] });

/** An operation's `spec`: it needs no token. */
export const open = (spec: (op: Operation) => Operation = op => op) =>
	(op: Operation): Operation => ({ ...spec(op), security: [] });

/** One secret's value (undefined when unset) and the scopes it grants. */
export interface Token {
	value: string | undefined;
	scopes: string[];
}

/** The security a procedure's contract declares, or the API's default. */
export function securityOf(procedure: unknown, defaults?: Security): Security | undefined {
	const spec = getOpenAPIMeta(procedure as never)?.spec;
	const op = typeof spec === "function" ? spec({} as Operation) : spec;
	return (op?.security as Security | undefined) ?? defaults;
}

/** Why a request may not run (401 or 403), or undefined when it may. */
export function authorize(authorization: string | null | undefined, security: Security | undefined, tokens: Token[]): { status: 401 | 403; message: string } | undefined {
	if (!security?.length) return undefined;
	const token = bearer(authorization);
	let known = false;
	const granted: string[] = [];
	for (const t of tokens) {
		if (t.value && token && same(token, t.value)) {
			known = true;
			granted.push(...t.scopes);
		}
	}
	if (!known) return { status: 401, message: "a valid token is required: Authorization: Bearer <token>" };
	// Security is a list of alternatives; the scopes of one are all needed.
	if (security.some(alternative => alternative[NAME]?.every(scope => granted.includes(scope)))) return undefined;
	return { status: 403, message: `this token may not do this: it needs ${security.map(a => `scope ${(a[NAME] ?? []).join(" and ")}`).join(", or ")}` };
}

function bearer(authorization: string | null | undefined) {
	const [scheme, token] = (authorization ?? "").split(" ", 2);
	return scheme?.toLowerCase() === "bearer" ? (token ?? "").trim() : "";
}

// Compares in time that depends only on the lengths.
function same(a: string, b: string) {
	const x = new TextEncoder().encode(a), y = new TextEncoder().encode(b);
	let diff = x.length ^ y.length;
	for (let i = 0; i < Math.max(x.length, y.length); i++) diff |= (x[i] ?? 0) ^ (y[i] ?? 0);
	return diff === 0;
}
