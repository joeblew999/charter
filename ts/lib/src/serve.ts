// A Worker's API served the standard way (docs/reference/standard.md), as go/lib/serve serves a Huma
// API, so a project gives its router, its document and whom it trusts, and wires nothing itself:
//   - its procedures under /api, as plain REST (oRPC's OpenAPIHandler), each call allowed by the
//     credentials it carries and limited as its contract says (guard);
//   - its specs at /api/openapi.json and /api/asyncapi.json, with the request's origin as their server;
//   - its procedures that answer once as MCP tools at /api/mcp (charter-lib/mcp);
//   - how to get in, at /.well-known/ (charter-lib/auth's wellKnown), when it trusts anyone;
//   - its WebSocket channels (asyncapi() procedures) at their addresses, a token on them taken as
//     ?access_token= too, for a client that cannot send headers (a browser, a Worker).
//
//   const os = implement(contract).$context<Served>();
//   const api = os.use(guard);
//   export const router = os.router({ notes: { list: api.notes.list.handler(...) } });
//   const served = serve(router, { document, trusted: () => trusted, limits: () => env });
//   export default { fetch: request => served(request, {}) } satisfies ExportedHandler;
//
// The router is built from `os`, without the guard: oRPC would otherwise run it twice per call.
import { getAsyncIteratorObjectSchemaDetails, type RouterContract } from "@orpc/contract";
import type { OpenAPIV3_1 } from "@orpc/openapi";
import { OpenAPIHandler } from "@orpc/openapi/fetch";
import { call, os, ORPCError, Procedure, type Router } from "@orpc/server";
import { ResponseHeadersPlugin } from "@orpc/server/plugins";
import { type AsyncAPIDocument, getAsyncAPIMeta } from "./asyncapi.ts";
import { authorize, type Base, type Caller, scopesOf, securityOf, type Security, type Trusted, wellKnown } from "./auth.ts";
import { mcpHandler, mcpPath } from "./mcp.ts";
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
 * The guard: every call allowed as its contract's security says (when the API trusts anyone) (the document's when it says none),
 * 401 without credentials it knows, 403 without the scope, each with the challenge that says where the
 * API tells a client how to get in (an MCP client reads it to sign in); then limited as its contract
 * says, per caller (429 with Retry-After). The caller is in the handler's context.
 */
export const guard = os.$context<Served>().middleware(async ({ context, procedure, next }) => {
	const { request, policy = {} } = context;
	// An API that trusts no one checks no credentials (as go/lib/serve).
	const decision = policy.trusted ? await authorize(request.headers, securityOf(procedure, policy.security), policy.trusted, new URL(request.url).origin) : { status: 200 as const, caller: undefined };
	if (decision.status !== 200) {
		context.resHeaders?.set("WWW-Authenticate", decision.challenge);
		throw new ORPCError(decision.status === 401 ? "UNAUTHORIZED" : "FORBIDDEN", { message: decision.message, data: { challenge: decision.challenge } });
	}
	const verdict = await rateLimit(limitOf(procedure, policy.rateLimits, policy.security), keyOf(decision.caller, request.headers), policy.limits ?? {});
	if (verdict.status === 429) {
		context.resHeaders?.set("Retry-After", String(verdict.retryAfter));
		throw new ORPCError("TOO_MANY_REQUESTS", { message: verdict.message });
	}
	return next({ context: { caller: decision.caller } });
});

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

/** The HTTP status of a refusal a procedure threw. */
const statusOf: Record<string, number> = { BAD_REQUEST: 400, UNAUTHORIZED: 401, FORBIDDEN: 403, NOT_FOUND: 404, TOO_MANY_REQUESTS: 429 };

/**
 * A WebSocket channel: the procedure called through its middleware (the guard first, so a refusal is
 * the HTTP answer and no socket opens), and each event it yields sent as plain JSON. Its input is the
 * query, checked by the contract (400 before upgrading), or, when the contract says the client sends
 * messages (an async iterator), the socket's messages, each checked as it comes. A refusal while the
 * socket is open (an ORPCError: a message the contract or the procedure refuses) closes it with 1008;
 * any other end (the feed gave up) with 1011, so the client reconnects where it was.
 */
async function channel(procedure: AnyProcedure, request: Request, context: object): Promise<Response> {
	if (request.headers.get("upgrade") !== "websocket") return new Response("expected a WebSocket upgrade", { status: 426 });
	const { 0: client, 1: server } = new WebSocketPair();
	const closed = new AbortController();
	const sends = !!getAsyncIteratorObjectSchemaDetails(procedure["~orpc"].inputSchemas?.[0]);
	const input = sends
		? new ReadableStream<unknown>({
			start(controller) {
				server.addEventListener("message", event => {
					try { controller.enqueue(JSON.parse(String(event.data))); } catch { controller.error(new ORPCError("BAD_REQUEST", { message: "a message that is not JSON" })); }
				});
				server.addEventListener("close", () => { try { controller.close(); } catch {} });
			},
		}).values()
		: Object.fromEntries(new URL(request.url).searchParams);
	let events: AsyncIterable<unknown>;
	try {
		events = (await call(procedure, input, { context, signal: closed.signal })) as AsyncIterable<unknown>;
	} catch (error) {
		const refusal = error instanceof ORPCError ? error : undefined;
		const challenge = (refusal?.data as { challenge?: string } | undefined)?.challenge;
		return new Response(refusal?.message ?? "the channel could not open", { status: statusOf[refusal?.code ?? ""] ?? 500, headers: challenge ? { "www-authenticate": challenge } : {} });
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
	return new Response(null, { status: 101, webSocket: client } as ResponseInit);
}

/**
 * The API served the standard way, as a fetch function: what is not the API's is 404. `context` is what
 * the handlers need beyond the request (a deployment's store, say); the request, the response's headers
 * and the policy are added to it.
 */
export function serve<C extends object>(router: Router<C & Served>, { document, trusted, limits }: ServeOptions<C>): (request: Request, context: C) => Promise<Response> {
	const handler = new OpenAPIHandler(router, { plugins: [new ResponseHeadersPlugin()] });
	const mcp = mcpHandler(router, { info: document.info });
	const specs = specsOf(router, document);
	const channels = channelsOf(router);
	const security = document.base?.security as Security | undefined;
	const scopes = scopesOf(router, security);
	return async (request, context) => {
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
		if (url.pathname === mcpPath) return mcp(request, served);
		if (socket) return channel(socket, request, served);
		// An API that takes credentials says how to get in; one that takes none has nothing to say.
		if (url.pathname.startsWith("/.well-known/") && policy.trusted) {
			return wellKnown(request, { resource: url.origin, title: document.info.title, scopes, trusted: policy.trusted }) ?? new Response("not found", { status: 404 });
		}
		const { matched, response } = await handler.handle(request, { context: served });
		return matched ? response : new Response("not found", { status: 404 });
	};
}
