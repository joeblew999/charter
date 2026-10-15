// MCP: a Worker's API as tools at one endpoint (/api/mcp), as go/lib/humamcp serves a Huma API. The
// protocol is the official SDK's (@modelcontextprotocol/server): Streamable HTTP, no state, the
// 2026-07-28 era and the 2025 handshake. What this adds is the tools themselves, from what the
// contract already says, so nothing is declared twice: every procedure that answers once is a tool
// named by its operationId, with its own input and output schemas, and a call runs the procedure
// through oRPC's call(), with its middleware and validation, in the REST handler's context (the
// request's headers): auth and rate limits are the REST route's.
//   const mcp = mcpHandler(router, { info });
//   if (url.pathname === mcpPath) return mcp(request, { headers: request.headers });
import type { Meta, MetaPlugin } from "@orpc/contract";
import { getAsyncIteratorObjectSchemaDetails } from "@orpc/contract";
import { getOpenAPIMeta } from "@orpc/openapi";
import { call, type Context, ORPCError, Procedure, type Router } from "@orpc/server";
import { createMcpHandler, McpServer } from "@modelcontextprotocol/server";

/** Where the endpoint is, beside the API's specs. */
export const mcpPath = "/api/mcp";

/** notATool leaves a procedure out of the tools, as humamcp.Expose(op, false) does. */
export const notATool = (): MetaPlugin<any, any, any> => ({
	name: "~charter-mcp",
	init: (current: Meta) => ({ ...current, "~charter-mcp": false }),
});

type AnyProcedure = Procedure<any, any, any, any, any>;

/** A tool: what tools/list says of it, and the procedure a call runs. */
export interface Tool {
	name: string;
	description?: string;
	annotations: { readOnlyHint: boolean; destructiveHint?: boolean };
	procedure: AnyProcedure;
	input?: unknown;
	output?: unknown;
}

/** toolOf is the tool a procedure is, or nothing: a stream, a channel, a procedure that is no API
 * route, or one left out. It says what humamcp says of an operation: the name is the operationId; the
 * description the summary, then the description; GET is read-only, and every method but POST may
 * overwrite or delete. Its schemas are the procedure's own (one input, one output). */
export function toolOf(procedure: AnyProcedure): Tool | undefined {
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
		annotations: method === "GET" ? { readOnlyHint: true } : { readOnlyHint: false, destructiveHint: method !== "POST" },
		procedure,
		input: inputs.length === 1 ? inputs[0] : undefined,
		output: outputs.length === 1 ? outputs[0] : undefined,
	};
}

/** toolsOf is every tool of the router, in its order. */
export function toolsOf(router: Router<any>): Tool[] {
	const tools: Tool[] = [];
	const walk = (node: unknown): void => {
		if (node instanceof Procedure) {
			const tool = toolOf(node);
			if (tool) tools.push(tool);
		} else if (node && typeof node === "object") Object.values(node).forEach(walk);
	};
	walk(router);
	return tools;
}

type JsonSchema = Record<string, unknown>;
type Standard = { "~standard": { jsonSchema?: { input(options: unknown): JsonSchema; output(options: unknown): JsonSchema } } & Record<string, unknown> };

/** objectRoot is the schema with its root's $ref resolved. zod writes a schema that has an id (a
 * named one: `.meta({ id: "Note" })`) as `{ $ref: "#/$defs/Note", $defs }`, and the MCP SDK takes a
 * root that does not say `type: "object"` for one that is no object: on a 2025 connection it then
 * wraps the result as `{ result }`, where humamcp answers the object itself.
 * Upstream: modelcontextprotocol/typescript-sdk, not filed (when fixed: drop objectRoot) */
function objectRoot(schema: unknown): unknown {
	const standard = (schema as Partial<Standard> | undefined)?.["~standard"];
	const json = standard?.jsonSchema;
	if (!standard || !json) return schema;
	const inline = (root: JsonSchema): JsonSchema => {
		const name = typeof root.$ref === "string" && root.$ref.startsWith("#/$defs/") ? root.$ref.slice("#/$defs/".length) : undefined;
		const named = name ? (root.$defs as Record<string, JsonSchema> | undefined)?.[name] : undefined;
		if (!named) return root;
		const { $ref: _, ...rest } = root;
		return { ...rest, ...named };
	};
	return { "~standard": { ...standard, jsonSchema: { input: (options: unknown) => inline(json.input(options)), output: (options: unknown) => inline(json.output(options)) } } };
}

/** What a refused call says: oRPC's error (its code, status, message and, for a validation failure,
 * the issues with their paths), so a model can correct itself. */
const refusal = (error: unknown) =>
	error instanceof ORPCError ? error.toJSON() : { message: error instanceof Error ? error.message : String(error) };

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
export function mcpHandler<T extends Context>(router: Router<T>, { info }: { info: { title: string; version: string } }): (request: Request, context: T) => Promise<Response> {
	const tools = toolsOf(router);
	const serverFor = (context: T, refused: { error?: ORPCError<string, { challenge: string }> }) => {
		const server = new McpServer({ name: info.title, version: info.version });
		for (const tool of tools) {
			server.registerTool(
				tool.name,
				{ description: tool.description, inputSchema: objectRoot(tool.input) as never, outputSchema: objectRoot(tool.output) as never, annotations: tool.annotations },
				(async (args: unknown) => {
					try {
						const output = await call(tool.procedure, (args ?? {}) as never, { context } as never);
						return { content: [{ type: "text", text: JSON.stringify(output) }], structuredContent: output };
					} catch (error) {
						if (asksForCredentials(error)) refused.error = error;
						return { isError: true, content: [{ type: "text", text: JSON.stringify(refusal(error)) }] };
					}
				}) as never,
			);
		}
		return server;
	};
	return async (request, context) => {
		const refused: { error?: ORPCError<string, { challenge: string }> } = {};
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
		return Response.json({ jsonrpc: "2.0", id: message?.id ?? null, error: { code: -32001, message: refusedMessage, data: refusal(refused.error) } }, { status: credentialStatus[code], headers: { "www-authenticate": data.challenge } });
	};
}
