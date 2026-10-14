// What Fern makes an SDK from, taken out of an OpenAPI and an AsyncAPI document: operations with their
// ids, tags, parameters, bodies by media type, security and x-fern-* extensions; webhooks; named
// schemas; channels with their messages and operations. Left out, because no SDK shows them:
// descriptions, key order, `additionalProperties`, number bounds, and oRPC's query-parameter hints.

type Json = Record<string, any>;

/** A schema as an SDK type sees it: its type, a named schema by name, an object's fields and which are required. */
function shape(schema: Json | undefined): unknown {
	if (!schema) return undefined;
	if (schema.$ref) return { ref: String(schema.$ref).split("/").pop() };
	if (schema.type === "array") return { type: "array", items: shape(schema.items) };
	if (schema.type === "object") {
		const properties = Object.fromEntries(Object.entries<Json>(schema.properties ?? {}).sort(([a], [b]) => a.localeCompare(b)).map(([name, property]) => [name, shape(property)]));
		return { type: "object", required: [...(schema.required ?? [])].sort(), properties };
	}
	return { type: schema.type, ...(schema.format && { format: schema.format }) };
}

const extensions = (object: Json) => Object.fromEntries(Object.entries(object).filter(([key]) => key.startsWith("x-")).sort(([a], [b]) => a.localeCompare(b)));
const bodies = (content: Json | undefined) => Object.fromEntries(Object.entries<Json>(content ?? {}).map(([mediaType, body]) => [mediaType, shape(body.schema)]));

function operation(op: Json) {
	return {
		operationId: op.operationId,
		tags: op.tags,
		summary: op.summary,
		...(op.security && { security: op.security }),
		extensions: extensions(op),
		parameters: (op.parameters ?? []).map((p: Json) => ({ name: p.name, in: p.in, required: p.required ?? false, schema: shape(p.schema) })),
		request: { required: op.requestBody?.required ?? false, content: bodies(op.requestBody?.content) },
		responses: Object.fromEntries(Object.entries<Json>(op.responses ?? {}).map(([status, response]) => [status, bodies(response.content)])),
	};
}

const operations = (paths: Json | undefined) => Object.fromEntries(Object.entries<Json>(paths ?? {}).flatMap(([path, item]) =>
	Object.entries<Json>(item).map(([method, op]) => [`${method.toUpperCase()} ${path}`, operation(op)])));

const name = (ref: { $ref: string }) => ref.$ref.split("/").pop()!;

export function surface(openapi: Json, asyncapi: Json) {
	return {
		openapi: {
			version: openapi.openapi,
			servers: openapi.servers.map((server: Json) => server.url),
			security: openapi.security,
			securitySchemes: openapi.components?.securitySchemes,
			extensions: extensions(openapi),
			operations: operations(openapi.paths),
			webhooks: operations(openapi.webhooks),
			schemas: Object.fromEntries(Object.entries<Json>(openapi.components?.schemas ?? {}).sort(([a], [b]) => a.localeCompare(b)).map(([key, schema]) => [key, shape(schema)])),
		},
		asyncapi: {
			servers: asyncapi.servers,
			channels: Object.fromEntries(Object.entries<Json>(asyncapi.channels).map(([id, channel]) => [id, {
				address: channel.address,
				query: shape(channel.bindings?.ws?.query),
				extensions: extensions(channel),
				messages: Object.fromEntries(Object.keys(channel.messages).sort().map(message => [message, shape(asyncapi.components.messages[name(channel.messages[message])].payload)])),
			}])),
			operations: Object.fromEntries(Object.entries<Json>(asyncapi.operations).sort(([a], [b]) => a.localeCompare(b)).map(([id, op]) => [id, {
				action: op.action,
				channel: name(op.channel),
				messages: op.messages.map(name),
			}])),
		},
	};
}
