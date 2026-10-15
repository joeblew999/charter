// A Worker's API served the standard way (docs/reference/standard.md), as go/lib/serve serves a Huma
// API, so a project gives its router, its document and whom it trusts, and wires nothing itself:
//   - its procedures under /api, as plain REST (oRPC's OpenAPIHandler), each call allowed by the
//     credentials it carries and limited as its contract says (guard);
//   - its specs at /api/openapi.json and /api/asyncapi.json, with the request's origin as their server;
//   - its procedures that answer once as MCP tools at /api/mcp (charter-lib/mcp);
//   - how to get in, at /.well-known/ (charter-lib/auth's wellKnown), when it trusts anyone;
//   - its WebSocket channels (asyncapi() procedures) at their addresses, a token on them taken as
//     ?access_token= too, for a client that cannot send headers (a browser, a Worker);
//   - every refusal as RFC 9457 problem details, as Huma answers in Go (charter-lib/problem), checked
//     in Huma's order: the route (404, 405), the credentials and the limits (401, 403, 429), the body's
//     media type and its JSON (415, 400), then the contract (422).
//
//   const os = implement(contract).$context<Served>();
//   const api = os.use(guard);
//   export const router = os.router({ notes: { list: api.notes.list.handler(...) } });
//   const served = serve(router, { document, trusted: () => trusted, limits: () => env });
//   export default { fetch: request => served(request, {}) } satisfies ExportedHandler;
//
// The router is built from `os`, without the guard: oRPC would otherwise run it twice per call.
import { getAsyncIteratorObjectSchemaDetails, type RouterContract, ValidationError } from "@orpc/contract";
import { getOpenAPIMeta, type OpenAPIV3_1 } from "@orpc/openapi";
import { OpenAPIHandler } from "@orpc/openapi/fetch";
import { type AnyORPCError, call, COMMON_ERROR_STATUS_MAP, os, ORPCError, Procedure, type Router } from "@orpc/server";
import { ResponseHeadersPlugin } from "@orpc/server/plugins";
import { type AsyncAPIDocument, getAsyncAPIMeta } from "./asyncapi.ts";
import { authorize, type Base, type Caller, scopesOf, securityOf, type Security, type Trusted, wellKnown } from "./auth.ts";
import { mcpHandler, mcpPath } from "./mcp.ts";
import { type ErrorDetail, problem, problemOf, problemType } from "./problem.ts";
import { keyOf, limitOf, type Limit, rateLimit } from "./ratelimit.ts";
import { asyncapiSpec, openapiSpec } from "./specs.ts";

/** The standard paths (docs/reference/standard.md). */
export const openapiPath = "/api/openapi.json";
export const asyncapiPath = "/api/asyncapi.json";
export { mcpPath };

/** What an API's document says beside its procedures: both specs' info, the OpenAPI document's fields
 * a contract has no place for (its security schemes and default security, extensions), its webhooks,
 * and the rate limits per scope. */
export interface Document {
	info: { title: string; version: string; description?: string };
	/** The AsyncAPI document's info: the API's channels. */
	asyncInfo: { title: string; version: string; description?: string };
	base?: Base;
	webhooks?: RouterContract;
	rateLimits?: Limit[];
}

/** Both specs of a router (or its contract) with its document, by server: what npm run spec writes
 * (charter.config.ts's specs) and what serve() answers /api/openapi.json and /api/asyncapi.json with. */
export interface Specs {
	openapi(server: string): Promise<OpenAPIV3_1.OpenAPIObject>;
	asyncapi(server: string): Promise<AsyncAPIDocument>;
}

export const specsOf = (router: Parameters<typeof openapiSpec>[0], document: Document): Specs => ({
	openapi: (server: string) => openapiSpec(router, { info: document.info, server, base: document.base, webhooks: document.webhooks, rateLimits: document.rateLimits }),
	asyncapi: (server: string) => asyncapiSpec(router, { info: document.asyncInfo, server }),
});

/** What the guard reads and serve() puts in every call's context: who may call, and how often. */
export interface Policy {
	trusted?: Trusted;
	/** The Rate Limiting bindings, by name (the Worker's env). */
	limits?: object;
	/** The document's default security. */
	security?: Security;
	rateLimits?: Limit[];
	/** serve() let this request in already, before it read the body (as Huma does): the guard does not again. */
	admitted?: boolean;
}

/** What every call's context has: the request, the response's headers, the policy, and the caller once
 * the guard lets it in. */
export interface Served {
	request: Request;
	resHeaders?: Headers;
	policy?: Policy;
	caller?: Caller;
}

/**
 * The guard: every call allowed as its contract's security says (the document's when it says none),
 * when the API trusts anyone: 401 without credentials it knows, 403 without the scope, each with the
 * challenge that says where the API tells a client how to get in (an MCP client reads it to sign in);
 * then limited as its contract says, per caller (429). The caller is in the handler's context. A REST
 * call or a channel serve() has let in already, before it read the body, as Huma does.
 */
export const guard = os.$context<Served>().middleware(async ({ context, procedure, next }) => {
	if (context.policy?.admitted) return next({ context: { caller: context.caller } });
	const admission = await admit(procedure, context.request, context.policy);
	if (!admission.ok) throw admission.refusal;
	return next({ context: { caller: admission.caller } });
});

/** Whether a request may run a procedure: its caller, or the refusal (401 or 403 with the challenge, 429
 * with Retry-After), as an ORPCError. An API that trusts no one checks no credentials (as go/lib/serve). */
async function admit(procedure: unknown, request: Request, policy: Policy = {}): Promise<{ ok: true; caller?: Caller } | { ok: false; refusal: ORPCError<string, unknown>; headers: Record<string, string> }> {
	const decision = policy.trusted ? await authorize(request.headers, securityOf(procedure, policy.security), policy.trusted, new URL(request.url).origin) : { status: 200 as const, caller: undefined };
	if (decision.status !== 200) {
		return { ok: false, refusal: new ORPCError(decision.status === 401 ? "UNAUTHORIZED" : "FORBIDDEN", { message: decision.message, data: { challenge: decision.challenge } }), headers: { "www-authenticate": decision.challenge } };
	}
	const verdict = await rateLimit(limitOf(procedure, policy.rateLimits, policy.security), keyOf(decision.caller, request.headers), policy.limits ?? {});
	if (verdict.status === 429) return { ok: false, refusal: new ORPCError("TOO_MANY_REQUESTS", { message: verdict.message }), headers: { "retry-after": String(verdict.retryAfter) } };
	return { ok: true, caller: decision.caller };
}

export interface ServeOptions<C> {
	document: Document;
	/** Whom the API trusts, for a call with this context (none: it checks no credentials). */
	trusted?: (context: C) => Trusted;
	/** The Rate Limiting bindings, for a call with this context (the Worker's env). */
	limits?: (context: C) => object;
}

type AnyProcedure = Procedure<any, any, any, any, any>;

/** Workers' WebSockets (a Workers global, not the web's): accept() takes the socket in the Worker. */
type WorkerSocket = WebSocket & { accept(): void };
declare const WebSocketPair: new () => Record<0 | 1, WorkerSocket>;

/** The channels of a router: each asyncapi() procedure, by its address. */
function channelsOf(router: Router<any>): Map<string, AnyProcedure> {
	const channels = new Map<string, AnyProcedure>();
	const walk = (node: unknown): void => {
		if (node instanceof Procedure) {
			const meta = getAsyncAPIMeta(node as never);
			if (meta) channels.set(meta.address, node);
		} else if (node && typeof node === "object") Object.values(node).forEach(walk);
	};
	walk(router);
	return channels;
}

/** The HTTP status of an ORPCError's code. */
const statusOf = (error: AnyORPCError) => (COMMON_ERROR_STATUS_MAP as Record<string, number>)[error.code] ?? 500;

/** A refusal as problem details: a refused input's errors are in its data (refused(), and the
 * contract's, which standardized() turns into them). */
const problemOfError = (error: AnyORPCError) => problemOf(statusOf(error), error.message, (error.data as { errors?: ErrorDetail[] } | undefined)?.errors);

/** A refusal as the response: problem details, with the headers it needs (the challenge, Retry-After). */
function refusal(error: AnyORPCError, headers: Record<string, string> = {}): Response {
	const challenge = (error.data as { challenge?: string } | undefined)?.challenge;
	return problem(statusOf(error), error.message, problemOfError(error).errors, { ...(challenge ? { "www-authenticate": challenge } : {}), ...headers });
}

/**
 * What a handler throws to refuse an input the contract lets through, as Huma's handlers do: 422, saying
 * where (`query.cursor`, `body.note`) and what was wrong there.
 *   throw refused("query.cursor", "not a cursor from next_cursor", input.cursor);
 */
export function refused(location: string, message: string, value?: unknown): ORPCError<"UNPROCESSABLE_CONTENT", { errors: ErrorDetail[] }> {
	return new ORPCError("UNPROCESSABLE_CONTENT", { message: "validation failed", data: { errors: [detail(message, location, value)] } });
}

const detail = (message: string, location: string, value: unknown): ErrorDetail => ({ message, location, ...(value === undefined || value === null ? {} : { value }) });

type JsonSchema = { type?: string; $ref?: string; properties?: Record<string, JsonSchema>; items?: JsonSchema; format?: string; contentMediaType?: string };
type Operation = { parameters?: { name: string; in: string; schema?: JsonSchema }[]; requestBody?: { required?: boolean; content?: Record<string, { schema?: JsonSchema }> } };

/** A route of the API: its method, its path as a pattern, and what the spec says of it. */
interface Route {
	method: string;
	path: string;
	pattern: RegExp;
	procedure: AnyProcedure;
	params: string[];
}

/** The routes of a router: each procedure with an OpenAPI route that is no channel. */
function routesOf(router: Router<any>): Route[] {
	const routes: Route[] = [];
	const walk = (node: unknown): void => {
		if (node instanceof Procedure) {
			const meta = getOpenAPIMeta(node as never) as { method?: string; path?: string } | undefined;
			if (!meta?.path || getAsyncAPIMeta(node as never)) return;
			const params = [...meta.path.matchAll(/\{([^}]+)\}/g)].map(m => m[1]!);
			const pattern = new RegExp(`^${meta.path.replace(/[.*+?^$()|[\]\\]/g, "\\$&").replace(/\\?\{[^}]+\\?\}/g, "([^/]+)")}$`);
			routes.push({ method: (meta.method ?? "POST").toUpperCase(), path: meta.path, pattern, procedure: node, params });
		} else if (node && typeof node === "object") Object.values(node).forEach(walk);
	};
	walk(router);
	return routes;
}

const bodyless = (method: string) => method === "GET" || method === "HEAD" || method === "DELETE";

/** Where an input's key came from, for an error's location: the path, the query, or the body. */
const sourceOf = (route: Route, key: string | undefined) => (key !== undefined && route.params.includes(key) ? "path" : bodyless(route.method) ? "query" : "body");

/** What the spec says of a route. */
const operationOf = (spec: OpenAPIV3_1.OpenAPIObject, route: Route) =>
	(spec.paths as Record<string, Record<string, Operation>> | undefined)?.[route.path]?.[route.method.toLowerCase()];

/** The schema at a place in a request, from the spec: a query or path parameter, or a property of the body. */
function schemaAt(spec: OpenAPIV3_1.OpenAPIObject, route: Route, parts: (string | number)[]): JsonSchema | undefined {
	const op = operationOf(spec, route);
	const resolve = (schema: JsonSchema | undefined): JsonSchema | undefined =>
		schema?.$ref ? resolve((spec.components?.schemas as Record<string, JsonSchema> | undefined)?.[schema.$ref.split("/").pop()!]) : schema;
	const [first, ...rest] = parts;
	if (sourceOf(route, String(first)) !== "body") return resolve(op?.parameters?.find(p => p.name === first)?.schema);
	let schema = resolve(Object.values(op?.requestBody?.content ?? {})[0]?.schema);
	for (const part of parts) schema = resolve(typeof part === "number" ? schema?.items : schema?.properties?.[part]);
	return schema;
}

/** A query value as Huma reads it: a number for a numeric parameter. */
const typed = (value: unknown, schema: JsonSchema | undefined) =>
	typeof value === "string" && (schema?.type === "integer" || schema?.type === "number") && value.trim() !== "" && Number.isFinite(Number(value)) ? Number(value) : value;

type Issue = { code: string; path?: readonly PropertyKey[] | readonly { key: PropertyKey }[]; message: string; expected?: string; minimum?: number | bigint; maximum?: number | bigint; inclusive?: boolean; origin?: string; format?: string; pattern?: string; keys?: string[]; values?: unknown[] };

/** A refused input's issues (zod's, through Standard Schema) as Huma words them, each where it was. */
function errorsOf(issues: readonly Issue[], input: unknown, route: Route, spec: OpenAPIV3_1.OpenAPIObject): ErrorDetail[] {
	const at = (parts: (string | number)[]) => parts.reduce<unknown>((value, part) => (value && typeof value === "object" ? (value as Record<string | number, unknown>)[part] : undefined), input);
	return issues.flatMap(issue => {
		const parts = (issue.path ?? []).map(p => (typeof p === "object" ? p.key : p) as string | number);
		const where = sourceOf(route, parts[0] === undefined ? undefined : String(parts[0]));
		const location = (path: (string | number)[]) => [where, ...path].join(".");
		const schema = schemaAt(spec, route, parts);
		const value = typed(at(parts), schema);
		const bound = (limit: number | bigint | undefined, op: string) => `${op}${issue.inclusive === false ? "" : "="} ${limit}`;
		switch (issue.code) {
			case "invalid_type": {
				const parent = at(parts.slice(0, -1)), key = parts.at(-1);
				if (key !== undefined && !(parent && typeof parent === "object" && key in parent)) {
					return schema?.format === "binary" || issue.expected === "file"
						? [detail("expected required file to be present", location(parts), undefined)]
						: [detail(`expected required property ${key} to be present`, location(parts.slice(0, -1)), parent)];
				}
				if (where === "query" && (schema?.type === "integer" || schema?.type === "number") && typeof value === "string") return [detail(`invalid ${schema.type === "integer" ? "integer" : "float"}`, location(parts), value)];
				return [detail(`expected ${schema?.type ?? issue.expected}`, location(parts), value)];
			}
			case "too_small":
			case "too_big": {
				const op = issue.code === "too_small" ? ">" : "<";
				const limit = issue.code === "too_small" ? issue.minimum : issue.maximum;
				const message = issue.origin === "string" ? `expected length ${bound(limit, op)}` : issue.origin === "array" ? `expected array length ${bound(limit, op)}` : `expected number ${bound(limit, op)}`;
				return [detail(message, location(parts), value)];
			}
			case "invalid_format":
				return [detail(issue.format === "regex" && issue.pattern ? `expected string to match pattern ${issue.pattern.replace(/^\/(.*)\/[a-z]*$/, "$1")}` : issue.message, location(parts), value)];
			case "unrecognized_keys":
				return (issue.keys ?? []).map(key => detail("unexpected property", location([...parts, key]), at(parts)));
			case "invalid_value":
				return [detail(`expected value to be one of "${(issue.values ?? []).join(", ")}"`, location(parts), value)];
			default:
				return [detail(issue.message, location(parts), value)];
		}
	});
}

/** An error a call ended with, as the standard has it: the contract refusing the input is a 422 that
 * says where and what (Huma's words); anything else is as it was. */
function standardized(error: unknown, route: Route | undefined, spec: OpenAPIV3_1.OpenAPIObject | undefined): unknown {
	if (!(error instanceof ORPCError) || !(error.cause instanceof ValidationError) || !route || !spec) return error;
	return new ORPCError("UNPROCESSABLE_CONTENT", { message: "validation failed", data: { errors: errorsOf(error.cause.issues as readonly Issue[], error.cause.invalidData, route, spec) } });
}

/**
 * A WebSocket channel: the procedure called through its middleware, and each event it yields sent as
 * plain JSON. Its input is the query, checked by the contract (422 before upgrading), or, when the
 * contract says the client sends messages (an async iterator), the socket's messages, each checked as
 * it comes. A request that is no upgrade is 426, once it is let in. A refusal while the socket is open
 * (an ORPCError: a message the contract or the procedure refuses) closes it with 1008; any other end
 * (the feed gave up) with 1011, so the client reconnects where it was.
 */
async function channel(procedure: AnyProcedure, request: Request, context: object): Promise<Response> {
	const upgrade = request.headers.get("upgrade") === "websocket";
	const pair = upgrade ? new WebSocketPair() : undefined;
	const server = pair?.[1];
	const closed = new AbortController();
	const sends = !!getAsyncIteratorObjectSchemaDetails(procedure["~orpc"].inputSchemas?.[0]);
	const input = sends
		? new ReadableStream<unknown>({
			start(controller) {
				server?.addEventListener("message", event => {
					try { controller.enqueue(JSON.parse(String(event.data))); } catch { controller.error(new ORPCError("BAD_REQUEST", { message: "a message that is not JSON" })); }
				});
				server?.addEventListener("close", () => { try { controller.close(); } catch {} });
			},
		}).values()
		: Object.fromEntries(new URL(request.url).searchParams);
	let events: AsyncIterable<unknown>;
	try {
		events = (await call(procedure, input, { context, signal: closed.signal })) as AsyncIterable<unknown>;
	} catch (error) {
		return error instanceof ORPCError ? refusal(error) : problem(500, "the channel could not open");
	}
	if (!pair || !server) {
		closed.abort();
		return problem(426, "expected a WebSocket upgrade");
	}
	server.accept();
	server.addEventListener("close", () => closed.abort());
	server.addEventListener("error", () => closed.abort());
	void (async () => {
		try {
			for await (const event of events) server.send(JSON.stringify(event));
		} catch (error) {
			if (closed.signal.aborted) return;
			if (error instanceof ORPCError) server.close(1008, error.message.slice(0, 100));
			else {
				console.error("channel: the feed gave up", String(error));
				try { server.close(1011, "unavailable; reconnect"); } catch {}
			}
		}
	})();
	return new Response(null, { status: 101, webSocket: pair[0] } as ResponseInit);
}

/** The body's media type, without its parameters. */
const mediaType = (request: Request) => request.headers.get("content-type")?.split(";")[0]?.trim().toLowerCase();

/**
 * The API served the standard way, as a fetch function: what is not the API's is 404. `context` is what
 * the handlers need beyond the request (a deployment's store, say); the request, the response's headers
 * and the policy are added to it.
 */
export function serve<C extends object>(router: Router<C & Served>, { document, trusted, limits }: ServeOptions<C>): (request: Request, context: C) => Promise<Response> {
	const specs = specsOf(router, document);
	// The spec says each route's media types and the types of its inputs: read once, for any server.
	let spec: Promise<OpenAPIV3_1.OpenAPIObject> | undefined;
	const specOnce = () => (spec ??= specs.openapi("http://spec.invalid"));
	const handler = new OpenAPIHandler(router, {
		plugins: [new ResponseHeadersPlugin()],
		clientInterceptors: [async ({ next, context }) => {
			try {
				return await next();
			} catch (error) {
				const route = (context as { route?: Route }).route;
				throw standardized(error, route, route ? await specOnce() : undefined);
			}
		}],
		customErrorResponseBodyEncoder: problemOfError,
	});
	const channels = channelsOf(router);
	const routes = routesOf(router);
	// A refused tool call says what the REST route would answer: its problem details.
	const mcp = mcpHandler(router, {
		info: document.info,
		refusal: async (error, tool) => {
			const route = routes.find(r => r.procedure === tool.procedure);
			const standard = standardized(error, route, route ? await specOnce() : undefined);
			return standard instanceof ORPCError ? problemOfError(standard) : problemOf(500, standard instanceof Error ? standard.message : String(standard));
		},
	});
	const security = document.base?.security as Security | undefined;
	const scopes = scopesOf(router, security);
	return async (request, context) => {
		const response = await answer(request, context);
		// A body the answer did not read is read to its end: answered first, cf dev's proxy fails the
		// next request on the connection (fetch failed). Upstream: cloudflare/workers-sdk, not filed
		// (when fixed: drop this).
		if (request.body && !request.bodyUsed) await request.arrayBuffer().catch(() => {});
		return response;
	};
	async function answer(request: Request, context: C): Promise<Response> {
		const url = new URL(request.url);
		const socket = channels.get(url.pathname);
		// On a channel, and only there, a token sent as ?access_token= is the Authorization header.
		const token = url.searchParams.get("access_token");
		if (socket && token && !request.headers.has("authorization")) {
			const headers = new Headers(request.headers);
			headers.set("authorization", `Bearer ${token}`);
			request = new Request(request, { headers });
		}
		const policy: Policy = { trusted: trusted?.(context), limits: limits?.(context), security, rateLimits: document.rateLimits };
		const served = { ...context, request, policy } as C & Served;
		if (url.pathname === openapiPath) return Response.json(await specs.openapi(url.origin));
		if (url.pathname === asyncapiPath) return Response.json(await specs.asyncapi(url.origin));
		if (url.pathname === mcpPath) {
			const answered = await mcp(request, served);
			if (answered.status !== 405 || answered.headers.has("allow")) return answered;
			// A 405 says what the path takes (RFC 9110), which the MCP SDK's does not.
			const headers = new Headers(answered.headers);
			headers.set("allow", "POST");
			return new Response(answered.body, { status: 405, headers });
		}
		if (socket) {
			// Let in before the upgrade, as Huma does, so a refusal is the answer and no socket opens.
			const admission = await admit(socket, request, policy);
			if (!admission.ok) return refusal(admission.refusal, admission.headers);
			return channel(socket, request, { ...served, caller: admission.caller, policy: { ...policy, admitted: true } });
		}
		// An API that takes credentials says how to get in; one that takes none has nothing to say.
		if (url.pathname.startsWith("/.well-known/") && policy.trusted) {
			return wellKnown(request, { resource: url.origin, title: document.info.title, scopes, trusted: policy.trusted }) ?? problem(404, "not found");
		}
		// The route, then who may call it and how often, then its body: the order Huma checks them in.
		const matching = routes.filter(r => r.pattern.test(url.pathname));
		const route = matching.find(r => r.method === request.method);
		if (!route) {
			if (!matching.length) return problem(404, "not found");
			return problem(405, "method not allowed", undefined, { allow: [...new Set(matching.map(r => r.method))].sort().join(", ") });
		}
		const admission = await admit(route.procedure, request, policy);
		if (!admission.ok) return refusal(admission.refusal, admission.headers);
		if (!bodyless(route.method)) {
			const op = operationOf(await specOnce(), route);
			const accepted = Object.keys(op?.requestBody?.content ?? {});
			const type = mediaType(request);
			if (accepted.length && type && !accepted.includes(type)) {
				return problem(415, "validation failed", [detail(`unknown content type: ${request.headers.get("content-type")}`, "body", await request.clone().text())]);
			}
			if (op?.requestBody?.required && !(await request.clone().arrayBuffer()).byteLength) return problem(400, "request body is required");
			if (type === "application/json") {
				const text = await request.clone().text();
				try { if (text) JSON.parse(text); } catch {
					return problem(400, "validation failed", [detail("the body is not valid JSON", "body", text)]);
				}
			}
		}
		const { matched, response } = await handler.handle(request, { context: { ...served, caller: admission.caller, route, policy: { ...policy, admitted: true } } as C & Served });
		if (!matched) return problem(404, "not found");
		if (response.status < 400) return response;
		const headers = new Headers(response.headers);
		headers.set("content-type", problemType);
		return new Response(response.body, { status: response.status, headers });
	}
}
