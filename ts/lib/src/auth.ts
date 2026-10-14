// Who may call an oRPC API, and with what scopes, the counterpart of go/auth: the contract declares
// what each operation needs (its `spec`), and one check enforces that, so the specs, the SDKs and the
// server agree.
//
// A caller proves who it is in one of three ways, and the API lists those it trusts (`Trusted`):
//   - a bearer token, one of the Worker's secrets (READ_TOKEN, WRITE_TOKEN), with the scopes it grants;
//   - Cloudflare Access in front of the Worker: a person logs in, a machine shows its own service
//     token; Access sends the Worker a JWT (Cf-Access-Jwt-Assertion), verified here against the
//     team's keys and the application's AUD tag (ACCESS_TEAM_DOMAIN, ACCESS_AUD). People and machines
//     get the scopes the API gives them;
//   - an OpenID Connect issuer's access token (Authorization: Bearer), verified here against the
//     issuer's keys, found by its discovery document (OIDC_ISSUER, OIDC_AUDIENCE). Its scope claim is
//     what it may do.
//
// `scheme()`, `accessScheme()` and `oidcScheme()` go into openapiSpec's `base`. An operation needs the
// scopes its security lists (`needs`; the API's default when it lists none; nothing when its security
// is an empty list), by any scheme. No credentials, or credentials that do not verify: 401. Known,
// without a scope the operation needs: 403. Unset, a secret or setting trusts no one.
//
// JWTs are verified with jose (github.com/panva/jose): RS256, ES256 and EdDSA only, the issuer, the
// audience, exp (required) and nbf with a minute's leeway. Keys are kept for an hour; a token whose
// key is not among them makes a new fetch, at most every 30 s.
import { getOpenAPIMeta, type OpenAPIV3_2 } from "@orpc/openapi";
import { createRemoteJWKSet, jwtVerify, type JWTPayload } from "jose";

/** The security schemes' names in the specs. */
export const NAME = "bearer";
export const ACCESS_CLIENT_ID = "accessClientId";
export const ACCESS_CLIENT_SECRET = "accessClientSecret";
export const OIDC_NAME = "oidc";

/** The header Cloudflare Access puts its JWT in, on every request it lets through. */
export const ACCESS_HEADER = "cf-access-jwt-assertion";

export type Security = Record<string, string[]>[];
type Operation = OpenAPIV3_2.OperationObject;
/** openapiSpec's `base`, or the part of it these functions write. */
export type Base = Partial<Omit<OpenAPIV3_2.OpenAPIObject, "openapi" | "info">>;

const withScheme = (base: Base, name: string, scheme: object): Base => ({
	...base,
	components: { ...base.components, securitySchemes: { ...base.components?.securitySchemes, [name]: scheme as OpenAPIV3_2.SecuritySchemeObject } },
});

/** For openapiSpec's `base`: bearer tokens, and the scopes an operation needs when it says nothing (none: public). */
export const scheme = (...scopes: string[]): Base => ({
	components: { securitySchemes: { [NAME]: { type: "http", scheme: "bearer" } } },
	...(scopes.length ? { security: [{ [NAME]: scopes }] } : {}),
});

/** The start of the variables an API's SDKs read: its name in capitals, _ for -. */
export const envPrefix = (api: string) => api.replace(/[-. ]/g, "_").toUpperCase();

/**
 * Adds a Cloudflare Access service token to a `base`: the two headers, which the SDKs take as options
 * or read from <API>_ACCESS_CLIENT_ID and <API>_ACCESS_CLIENT_SECRET (api: the API's name).
 */
export const accessScheme = (api: string, base: Base) => {
	const header = (name: string, option: string, variable: string, description: string) =>
		({ type: "apiKey", in: "header", name, description, "x-fern-header": { name: option, env: `${envPrefix(api)}${variable}` } });
	return withScheme(withScheme(base,
		ACCESS_CLIENT_ID, header("CF-Access-Client-Id", "accessClientId", "_ACCESS_CLIENT_ID", "A Cloudflare Access service token's Client ID: a machine's own (charter access token create)")),
		ACCESS_CLIENT_SECRET, header("CF-Access-Client-Secret", "accessClientSecret", "_ACCESS_CLIENT_SECRET", "The service token's Client Secret"));
};

/** Adds an OpenID Connect issuer's access tokens to a `base`; the discovery URL is the API's, which `discovery` serves. */
export const oidcScheme = (base: Base) => withScheme(base, OIDC_NAME, {
	type: "openIdConnect", openIdConnectUrl: "/.well-known/openid-configuration",
	description: "An access token from the OpenID Connect issuer the deployment trusts (OIDC_ISSUER), with the operation's scopes",
});

/** An operation's `spec`: it needs all of these scopes, by any scheme. Wraps another spec, e.g. Fern's names. */
export const needs = (scopes: string[], spec: (op: Operation) => Operation = op => op) =>
	(op: Operation): Operation => ({ ...spec(op), security: [{ [NAME]: scopes }] });

/** An operation's `spec`: it needs no credentials. */
export const open = (spec: (op: Operation) => Operation = op => op) =>
	(op: Operation): Operation => ({ ...spec(op), security: [] });

const OURS = [NAME, ACCESS_CLIENT_ID, ACCESS_CLIENT_SECRET, OIDC_NAME];

/** Every scope a requirement names, whichever scheme names it. */
const scopesIn = (requirement: Record<string, string[]>) =>
	[...new Set(Object.keys(requirement).sort().flatMap(name => requirement[name] ?? []))];

/**
 * A security list with each requirement that names only these schemes rewritten as one per scheme
 * the spec declares, with the same scopes: the specs then say every way in. openapiSpec applies it.
 */
export function expand(declared: Record<string, unknown> | undefined, security: Security | undefined): Security | undefined {
	if (!security?.length || !declared) return security;
	const out: Security = [], seen = new Set<string>();
	const add = (requirement: Record<string, string[]>, key: string) => { if (!seen.has(key)) { seen.add(key); out.push(requirement); } };
	for (const requirement of security) {
		if (!Object.keys(requirement).every(name => OURS.includes(name))) { out.push(requirement); continue; }
		const scopes = scopesIn(requirement), key = scopes.join(" ");
		if (declared[NAME]) add({ [NAME]: scopes }, `${NAME} ${key}`);
		if (declared[ACCESS_CLIENT_ID]) add({ [ACCESS_CLIENT_ID]: scopes, [ACCESS_CLIENT_SECRET]: [] }, `${ACCESS_CLIENT_ID} ${key}`);
		if (declared[OIDC_NAME]) add({ [OIDC_NAME]: scopes }, `${OIDC_NAME} ${key}`);
	}
	return out.length ? out : security;
}

/** One secret's value (undefined when unset) and the scopes it grants. */
export interface Token {
	secret?: string; // the secret's name, e.g. WRITE_TOKEN: the caller's `token`
	value: string | undefined;
	scopes: string[];
}

/** Cloudflare Access for one application (unset: trusts no one): what a person and a machine may do. */
export interface Access {
	team: string | undefined; // ACCESS_TEAM_DOMAIN: <team>.cloudflareaccess.com
	aud: string | undefined; // ACCESS_AUD: the application's AUD tag
	people: string[];
	machines: string[];
}

/** An OpenID Connect issuer (unset: trusts none); a token grants the scopes in its scope claim. */
export interface OIDC {
	issuer: string | undefined; // OIDC_ISSUER
	audience: string | undefined; // OIDC_AUDIENCE
}

/** The ways in the API accepts. */
export interface Trusted {
	tokens?: Token[];
	access?: Access;
	oidc?: OIDC;
}

/** Who a request comes from, once its credentials are verified, and what it may do. */
export interface Caller {
	scheme: "bearer" | "access" | "oidc";
	email?: string; // a person: from Access, or an OIDC token that has it
	machine?: string; // an Access service token's Client ID
	subject?: string; // the token's sub
	token?: string; // a bearer token: the name of the secret that holds it, e.g. WRITE_TOKEN
	scopes: string[];
}

/** The caller as a log line or an error message would name it. */
export const callerName = (c: Caller) => c.email ?? (c.machine ? `service token ${c.machine}` : c.token ? `the token ${c.token}` : c.subject ?? "the token");

/** The security a procedure's contract declares, or the API's default. */
export function securityOf(procedure: unknown, defaults?: Security): Security | undefined {
	const spec = getOpenAPIMeta(procedure as never)?.spec;
	const op = typeof spec === "function" ? spec({} as Operation) : spec;
	return (op?.security as Security | undefined) ?? defaults;
}

export type Decision = { status: 401 | 403; message: string } | { status: 200; caller?: Caller };

/** Whether a request with these headers may run an operation with this security: 200 with its caller, or 401 or 403. */
export async function authorize(headers: Headers, security: Security | undefined, trusted: Trusted): Promise<Decision> {
	if (!security?.length) return { status: 200 };
	let callers: Caller[];
	try {
		callers = await identify(headers, trusted);
	} catch (error) {
		return { status: 401, message: `the credentials were refused: ${(error as Error).message}` };
	}
	if (!callers.length) return { status: 401, message: `credentials are required: ${ways(trusted)}` };
	// Security is a list of alternatives; the scopes of one are all needed.
	for (const alternative of security) {
		const caller = callers.find(c => scopesIn(alternative).every(scope => c.scopes.includes(scope)));
		if (caller) return { status: 200, caller };
	}
	const needed = [...new Set(security.map(a => `scope ${scopesIn(a).join(" and ")}`))].join(", or ");
	return { status: 403, message: `${callerName(callers[0]!)} may not do this: it needs ${needed}` };
}

function ways(trusted: Trusted) {
	const ways = [];
	if (trusted.access?.team) ways.push("a Cloudflare Access login or service token (CF-Access-Client-Id, CF-Access-Client-Secret)");
	if (trusted.tokens?.length || trusted.oidc) ways.push("Authorization: Bearer <token>");
	return ways.join(", or ");
}

// Every credential the request carries, verified; one that does not verify throws.
async function identify(headers: Headers, trusted: Trusted): Promise<Caller[]> {
	const callers: Caller[] = [];
	const jwt = headers.get(ACCESS_HEADER), access = trusted.access;
	if (jwt && access?.team && access.aud) {
		const team = /^https?:\/\//.test(access.team) ? access.team.replace(/\/$/, "") : `https://${access.team.replace(/\/$/, "")}`;
		const claims = await verify(jwt, team, `${team}/cdn-cgi/access/certs`, access.aud, "the Access token");
		const email = typeof claims.email === "string" && claims.email ? claims.email : undefined;
		const machine = !email && typeof claims.common_name === "string" && claims.common_name ? claims.common_name : undefined;
		if (!email && !machine) throw new Error("the Access token names no one");
		callers.push({ scheme: "access", email, machine, subject: claims.sub || undefined, scopes: email ? access.people : access.machines });
	}
	const [kind, token = ""] = (headers.get("authorization") ?? "").split(" ", 2);
	if (kind?.toLowerCase() !== "bearer" || !token.trim()) return callers;
	const bearer = token.trim(), oidc = trusted.oidc;
	if (bearer.split(".").length === 3 && oidc?.issuer && oidc.audience) {
		const { issuer, jwks } = await discover(oidc.issuer);
		const claims = await verify(bearer, issuer, jwks, oidc.audience, "the OpenID Connect token");
		if (!claims.sub) throw new Error("the OpenID Connect token names no one");
		const scopes = typeof claims.scope === "string" ? claims.scope.split(" ").filter(Boolean) : Array.isArray(claims.scp) ? claims.scp.map(String) : [];
		return [...callers, { scheme: "oidc", email: typeof claims.email === "string" ? claims.email : undefined, subject: claims.sub, scopes }];
	}
	const matching = (trusted.tokens ?? []).filter(t => t.value && same(bearer, t.value));
	if (!matching.length) throw new Error("not a token this API knows");
	return [...callers, { scheme: "bearer", token: matching[0]!.secret, scopes: matching.flatMap(t => t.scopes) }];
}

// The keys of each issuer (createRemoteJWKSet keeps them; an unknown kid refetches after its cooldown),
// and the discovery documents, for the isolate's life.
const keySets = new Map<string, ReturnType<typeof createRemoteJWKSet>>();
const discovered = new Map<string, { issuer: string; jwks: string; fetched: number }>();
const HOUR = 3_600_000;

async function verify(token: string, issuer: string, jwks: string, audience: string, what: string): Promise<JWTPayload> {
	let keys = keySets.get(jwks);
	if (!keys) keySets.set(jwks, keys = createRemoteJWKSet(new URL(jwks), { cacheMaxAge: HOUR, cooldownDuration: 30_000 }));
	try {
		const { payload } = await jwtVerify(token, keys, { issuer, audience, algorithms: ["RS256", "ES256", "EdDSA"], clockTolerance: 60, requiredClaims: ["exp"] });
		return payload;
	} catch (error) {
		throw new Error(`${what}: ${(error as Error).message}`);
	}
}

// The issuer's discovery document; its issuer must be the one asked for (OpenID Connect Discovery 1.0, 4.3).
async function discover(issuer: string) {
	const kept = discovered.get(issuer);
	if (kept && Date.now() - kept.fetched < HOUR) return kept;
	const res = await fetch(`${issuer.replace(/\/$/, "")}/.well-known/openid-configuration`);
	if (!res.ok) throw new Error(`the discovery document of ${issuer}: HTTP ${res.status}`);
	const doc = await res.json() as { issuer?: string; jwks_uri?: string };
	if (doc.issuer !== issuer || !doc.jwks_uri) throw new Error(`the discovery document of ${issuer} names the issuer ${doc.issuer} and the keys ${doc.jwks_uri}`);
	const found = { issuer: doc.issuer, jwks: doc.jwks_uri, fetched: Date.now() };
	discovered.set(issuer, found);
	return found;
}

/**
 * Where /.well-known/openid-configuration (the specs' openIdConnectUrl) sends a client: the issuer's
 * discovery document. Undefined without an issuer: answer 404.
 */
export const discoveryURL = (issuer: string | undefined) => issuer && `${issuer.replace(/\/$/, "")}/.well-known/openid-configuration`;

// Compares in time that depends only on the lengths.
function same(a: string, b: string) {
	const x = new TextEncoder().encode(a), y = new TextEncoder().encode(b);
	let diff = x.length ^ y.length;
	for (let i = 0; i < Math.max(x.length, y.length); i++) diff |= (x[i] ?? 0) ^ (y[i] ?? 0);
	return diff === 0;
}
