// MCP: a Worker's API as tools at one endpoint (/api/mcp), as charter/mcp serves a Huma API. The
// protocol is the official SDK's (@modelcontextprotocol/server): Streamable HTTP, no state, the
// 2026-07-28 era and the 2025 handshake. What this adds is the tools themselves, from what the
// contract already says, so nothing is declared twice: every procedure that answers once is a tool
// named by its operationId, with its own input and output schemas, and a call runs the procedure
// through oRPC's call(), with its middleware and validation, in the REST handler's context: auth and
// rate limits are the REST route's. charter-lib/serve mounts it at /api/mcp.
import type { Meta, MetaPlugin } from "@orpc/contract";
import { getAsyncIteratorObjectSchemaDetails } from "@orpc/contract";
import { getOpenAPIMeta } from "@orpc/openapi";
import { call, type Context, ORPCError, type Router } from "@orpc/server";
import { createMcpHandler, McpServer } from "@modelcontextprotocol/server";
import { withoutSafeBounds, withoutUnusedDefs } from "./schema.ts";
import { type AnyProcedure, proceduresOf } from "./router.ts";

/** Where the endpoint is, beside the API's specs. */
export const mcpPath = "/api/mcp";

/** notATool leaves a procedure out of the tools, as charter/mcp's NotATool(op) does. */
export const notATool = (): MetaPlugin<any, any, any> => ({
	name: "~charter-mcp",
	init: (current: Meta) => ({ ...current, "~charter-mcp": false }),
});


/** A tool: what tools/list says of it, and the procedure a call runs. */
export interface Tool {
	name: string;
	description?: string;
	annotations: { readOnlyHint: boolean; destructiveHint?: boolean; idempotentHint?: boolean };
	procedure: AnyProcedure;
	input?: unknown;
	output?: unknown;
}

/** toolOf is the tool a procedure is, or nothing: a stream, a channel, a procedure that is no API
 * route, or one left out. It says what charter/mcp says of an operation: the name is the operationId; the
 * description the summary, then the description; GET is read-only, and every method but POST may
 * overwrite or delete. Its schemas are the procedure's own (one input, one output). */
function toolOf(procedure: AnyProcedure): Tool | undefined {
	const def = procedure["~orpc"] as { meta: Meta; inputSchemas?: unknown[]; outputSchemas?: unknown[] };
	const meta = def.meta as Record<string, unknown>;
	if (meta["~charter-mcp"] === false || meta["~asyncapi"]) return undefined;
	const outputs = def.outputSchemas ?? [], inputs = def.inputSchemas ?? [];
	if (outputs.some(schema => getAsyncIteratorObjectSchemaDetails(schema as never))) return undefined;
	const route = getOpenAPIMeta(procedure as never) as { method?: string; operationId?: string; summary?: string; description?: string } | undefined;
	if (!route?.operationId) return undefined;
	const method = (route.method ?? "POST").toUpperCase();
	return {
		name: route.operationId,
		description: [route.summary, route.description].filter(Boolean).join("\n\n") || undefined,
		annotations: method === "GET" ? { readOnlyHint: true } : { readOnlyHint: false, destructiveHint: method !== "POST", idempotentHint: method === "PUT" || method === "DELETE" },
		procedure,
		input: inputs.length === 1 ? inputs[0] : undefined,
		output: outputs.length === 1 ? outputs[0] : undefined,
	};
}

/** toolsOf is every tool of the router, in its order. */
export function toolsOf(router: Router<any>): Tool[] {
	return proceduresOf(router).flatMap(procedure => toolOf(procedure) ?? []);
}

type JsonSchema = Record<string, unknown>;
type Standard = { "~standard": { jsonSchema?: { input(options: unknown): JsonSchema; output(options: unknown): JsonSchema } } & Record<string, unknown> };

/** objectRoot is the schema with its root's $ref resolved. zod writes a schema that has an id (a
 * named one: `.meta({ id: "Note" })`) as `{ $ref: "#/$defs/Note", $defs }`, and the MCP SDK takes a
 * root that does not say `type: "object"` for one that is no object: on a 2025 connection it then
 * wraps the result as `{ result }`, where charter/mcp answers the object itself.
 * It also strips the bounds no JSON number can break (withoutSafeBounds), and lets an input through
 * unchecked: those two stay.
 * Upstream: modelcontextprotocol/typescript-sdk#3032 (when fixed: drop the inlining of the root's $ref, in inline) */
function objectRoot(schema: unknown, checked: boolean): unknown {
	const standard = (schema as Partial<Standard> | undefined)?.["~standard"];
	const json = standard?.jsonSchema;
	if (!standard || !json) return schema;
	const inline = (root: JsonSchema): JsonSchema => {
		const name = typeof root.$ref === "string" && root.$ref.startsWith("#/$defs/") ? root.$ref.slice("#/$defs/".length) : undefined;
		const named = name ? (root.$defs as Record<string, JsonSchema> | undefined)?.[name] : undefined;
		if (!named) return withoutSafeBounds(root);
		const { $ref: _, ...rest } = root;
		return withoutSafeBounds(withoutUnusedDefs({ ...rest, ...named }));
	};
	// An input the SDK passes on unchecked: the procedure checks it, so a refused argument is refused as
	// the REST route refuses it (charter-lib/serve's problem details), as charter/mcp does.
	const validate = checked ? standard.validate : (value: unknown) => ({ value });
	return { "~standard": { ...standard, validate, jsonSchema: { input: (options: unknown) => inline(json.input(options)), output: (options: unknown) => inline(json.output(options)) } } };
}

/** What a refused call says by default: oRPC's error (its code, status, message and, for a validation
 * failure, the issues with their paths), so a model can correct itself. charter-lib/serve says it as the
 * REST route does: problem details. */
const oRPCRefusal = (error: unknown) =>
	error instanceof ORPCError ? error.toJSON() : { message: error instanceof Error ? error.message : String(error) };

export interface MCPOptions {
	info: { title: string; version: string };
	/** What a refused tool call's text is, as JSON: by default oRPC's error. */
	refusal?: (error: unknown, tool: Tool) => unknown;
}

/** The input of a tool that takes nothing: an empty object, saying its dialect as every tool's schema does. */
const emptyObject = { $schema: "https://json-schema.org/draft/2020-12/schema", type: "object", properties: {} };
const noArguments = { "~standard": { version: 1, vendor: "charter", validate: (value: unknown) => ({ value }), jsonSchema: { input: () => emptyObject, output: () => emptyObject } } };

/** How long a client may keep the tool list, and who may (the 2026-07-28 revision's cache hints), as
 * charter/mcp says. */
const listCache = { ttlMs: 300_000, cacheScope: "public" as const };

/** A call refused for its credentials: a 401 or 403 whose ORPCError carries the challenge
 * (`data.challenge`, charter-lib/auth's Decision), which the answer is, not a tool error. */
const asksForCredentials = (error: unknown): error is ORPCError<string, { challenge: string }> =>
	error instanceof ORPCError && error.code in credentialStatus && typeof (error.data as { challenge?: unknown } | undefined)?.challenge === "string";
const credentialStatus: Record<string, number> = { UNAUTHORIZED: 401, FORBIDDEN: 403 };

/**
 * mcpHandler serves the router's tools at /api/mcp: a server per request (the SDK's stateless way on
 * Workers), whose calls run in the context given, the one the REST handler is given. A call refused
 * for its credentials (asksForCredentials) is the answer's own 401 or 403 with its WWW-Authenticate,
 * as MCP's authorization asks, so an MCP client knows to sign in; any other refusal is a tool error.
 */
export function mcpHandler<T extends Context>(router: Router<T>, { info, refusal = oRPCRefusal }: MCPOptions): (request: Request, context: T) => Promise<Response> {
	const tools = toolsOf(router);
	const serverFor = (context: T, refused: { error?: ORPCError<string, { challenge: string }>; tool?: Tool }) => {
		// The tools are the same for everyone, until the API changes: any cache may keep the list.
		const server = new McpServer({ name: info.title, version: info.version }, { cacheHints: { "tools/list": listCache, "server/discover": listCache } });
		for (const tool of tools) {
			server.registerTool(
				tool.name,
				{ description: tool.description, inputSchema: (tool.input ? objectRoot(tool.input, false) : noArguments) as never, outputSchema: objectRoot(tool.output, true) as never, annotations: tool.annotations },
				(async (args: unknown) => {
					try {
						const output = await call(tool.procedure, (args ?? {}) as never, { context } as never);
						return { content: [{ type: "text", text: JSON.stringify(output) }], structuredContent: output };
					} catch (error) {
						if (asksForCredentials(error)) Object.assign(refused, { error, tool });
						return { isError: true, content: [{ type: "text", text: JSON.stringify(await refusal(error, tool)) }] };
					}
				}) as never,
			);
		}
		return server;
	};
	return async (request, context) => {
		const refused: { error?: ORPCError<string, { challenge: string }>; tool?: Tool } = {};
		const message = request.method === "POST" ? ((await request.clone().json().catch(() => undefined)) as { id?: unknown; method?: string } | undefined) : undefined;
		let response = await createMcpHandler(() => serverFor(context, refused), { responseMode: "json" }).fetch(request);
		// The 2025 era answers as one Server-Sent Event (the SDK's fallback for it streams whatever the
		// response mode), sent before the tool has run: a tool call answers once, so its answer is read
		// to the end here, and a refusal for credentials can still be the answer.
		if (message?.method === "tools/call" && response.body && response.headers.get("content-type")?.startsWith("text/event-stream")) {
			response = new Response(await response.text(), { status: response.status, headers: response.headers });
		}
		if (!refused.error) return response;
		await response.body?.cancel();
		const { code, message: refusedMessage, data } = refused.error;
		return Response.json({ jsonrpc: "2.0", id: message?.id ?? null, error: { code: -32001, message: refusedMessage, data: await refusal(refused.error, refused.tool!) } }, { status: credentialStatus[code], headers: { "www-authenticate": data.challenge } });
	};
}
