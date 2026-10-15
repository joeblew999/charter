// Both specs from one contract or router: for a project's npm run spec (offline, into its fern/ folder) and for a
// Worker (/api/openapi.json, /api/asyncapi.json). It knows no contract: the caller passes its `info`
// and URL. No Workers APIs and no files here, so Node and a Worker can both run it.
import { getOpenAPIMeta, OpenAPIGenerator, type OpenAPIV3_1, type OpenAPIV3_2 } from "@orpc/openapi";
import type { RouterContract } from "@orpc/contract";
import { DelegatingJsonSchemaConverter, mapJsonSchemaRefs, type JsonSchema } from "@orpc/json-schema";
import { walkProcedureContractsAsync, type AnyRouter } from "@orpc/server";
import { ZodToJsonSchemaConverter } from "@orpc/zod";
import { AsyncAPIGenerator, getAsyncAPIMeta } from "./asyncapi.ts";
import { expand, type Security } from "./auth.ts";
import { reasonOf } from "./problem.ts";
import { EXTENSION, needsScope, withLimit, type Limit } from "./ratelimit.ts";
import { withoutSafeBounds } from "./schema.ts";

const converters = [new ZodToJsonSchemaConverter()];

export interface OpenAPISpecOptions {
	info: OpenAPIV3_2.OpenAPIObject["info"];
	/** The API's URL. */
	server: string;
	/**
	 * Document fields a contract has no place for: `security`, `components.securitySchemes`,
	 * document-level `x-fern-*` extensions, or `servers` to say more than the URL.
	 */
	base?: Partial<Omit<OpenAPIV3_2.OpenAPIObject, "openapi" | "info">>;
	/**
	 * OpenAPI 3.1 `webhooks`, which oRPC's generator doesn't write: a contract of the calls the API
	 * makes to its users. Each procedure is one webhook, named by its key; its input is the payload.
	 */
	webhooks?: RouterContract;
	/** Rate limits per scope (charter-lib/ratelimit): every operation that needs one's scope has it, unless it has its own. */
	rateLimits?: Limit[];
}

/** OpenAPI 3.1.0 (oRPC 2.0 defaults to 3.2.0, which Fern rejects; Huma writes 3.1.0), without the
 * WebSocket channels, written as the standard document (standardDocument). */
// Upstream: fern-api/fern#9559 (when fixed: drop `version` and use oRPC's 3.2.0 default)
export const openapiSpec = async (router: RouterContract | AnyRouter, { info, server, base, webhooks, rateLimits = [] }: OpenAPISpecOptions): Promise<OpenAPIV3_1.OpenAPIObject> => {
	const doc = await new OpenAPIGenerator({ converters }).generate(router, {
		version: "3.1.0",
		base: { info, servers: [{ url: server }], ...base },
		filter: procedure => !getAsyncAPIMeta(procedure as any),
	});
	withScopeLimits(doc, rateLimits);
	withEveryScheme(doc);
	return standardDocument(withoutSafeBounds(webhooks ? await addWebhooks(doc, webhooks) : doc));
};

// Each operation's security names every scheme the document declares (charter-lib/auth's expand), and
// an operation that is public says so.
function withEveryScheme(doc: OpenAPIV3_1.OpenAPIObject) {
	const declared = doc.components?.securitySchemes;
	doc.security = expand(declared, doc.security);
	for (const item of Object.values(doc.paths ?? {})) {
		for (const op of Object.values(item ?? {})) {
			if (!op || typeof op !== "object" || !("responses" in op)) continue;
			const operation = op as OpenAPIV3_1.OperationObject;
			// Public, said so: generated SDKs then send nothing it does not need.
			operation.security = expand(declared, operation.security) ?? (declared && !doc.security?.length ? [] : undefined);
		}
	}
}

// Each operation that needs a limited scope, and has no limit of its own, has the scope's.
function withScopeLimits(doc: OpenAPIV3_1.OpenAPIObject, limits: Limit[]) {
	if (!limits.length) return;
	for (const item of Object.values(doc.paths ?? {}) as Record<string, unknown>[]) {
		for (const [method, op] of Object.entries(item ?? {})) {
			if (!op || typeof op !== "object" || !("responses" in op) || EXTENSION in op) continue;
			const security = ((op as { security?: Security }).security ?? doc.security) as Security | undefined;
			const limit = limits.find(l => needsScope(security, l.scope));
			if (limit) item[method] = withLimit(op as OpenAPIV3_2.OperationObject, limit);
		}
	}
}

// Each webhook is a POST the API sends. Its payload is converted as output (the API produces it), so
// it shares its named schemas (components.schemas) with the responses.
async function addWebhooks<T extends { components?: OpenAPIV3_2.ComponentsObject }>(doc: T, webhooks: RouterContract) {
	const converter = new DelegatingJsonSchemaConverter(converters);
	const schemas: Record<string, unknown> = (doc.components ??= {}).schemas ??= {};
	const component = (schema: JsonSchema) => mapJsonSchemaRefs(schema, ref => ref.replace(/^#\/\$defs\//, "#/components/schemas/"));
	const hooks: Record<string, OpenAPIV3_2.PathItemObject> = {};
	await walkProcedureContractsAsync(webhooks, (procedure, path) => {
		const meta = getOpenAPIMeta(procedure as any);
		const [converted] = converter.convert((procedure as any)["~orpc"].inputSchemas?.[0], "output");
		const { $defs = {}, ...payload } = typeof converted === "object" ? converted : {};
		for (const [name, def] of Object.entries($defs)) {
			const schema = component(def);
			if (name in schemas && JSON.stringify(schemas[name]) !== JSON.stringify(schema)) throw new Error(`webhook ${path.join(".")}: its schema ${name} differs from components.schemas.${name}`);
			schemas[name] = schema;
		}
		const operation: OpenAPIV3_2.OperationObject = {
			operationId: meta?.operationId ?? path.join("."),
			summary: meta?.summary,
			description: meta?.description,
			tags: meta?.tags?.map(tag => tag),
			requestBody: { content: { "application/json": { schema: component(payload) as OpenAPIV3_2.SchemaObject } } },
			responses: { "200": { description: "received" } },
		};
		hooks[path.join(".")] = { post: typeof meta?.spec === "function" ? meta.spec(operation) : operation };
	});
	return { ...doc, webhooks: hooks };
}

export interface AsyncAPISpecOptions {
	info: { title: string; version: string; description?: string };
	/** The API's URL: its host, its path if it has one, and ws or wss by its protocol. */
	server: string;
}

/** AsyncAPI 3.0.0: the WebSocket channels. */
export const asyncapiSpec = async (router: RouterContract | AnyRouter, { info, server }: AsyncAPISpecOptions) => {
	const url = new URL(server);
	const pathname = url.pathname.replace(/\/$/, "");
	return withoutSafeBounds(await new AsyncAPIGenerator({ converters }).generate(router, {
		base: { info, servers: { production: { host: url.host, ...(pathname && { pathname }), protocol: url.protocol === "http:" ? "ws" : "wss" } } },
	}));
};

/** Huma's problem details schemas (RFC 9457), as go/lib/problem writes them: the body of every refusal. */
const problemSchemas = {
	ErrorModel: { additionalProperties: false, properties: { detail: { description: "A human-readable explanation specific to this occurrence of the problem.", examples: ["Property foo is required but is missing."], type: "string" }, errors: { description: "Optional list of individual error details", items: { $ref: "#/components/schemas/ErrorDetail" }, type: "array" }, instance: { description: "A URI reference that identifies the specific occurrence of the problem.", examples: ["https://example.com/error-log/abc123"], format: "uri", type: "string" }, status: { description: "HTTP status code", examples: [400], format: "int64", type: "integer" }, title: { description: "A short, human-readable summary of the problem type. This value should not change between occurrences of the error.", examples: ["Bad Request"], type: "string" }, type: { default: "about:blank", description: "A URI reference to human-readable documentation for the error.", examples: ["https://example.com/errors/example"], format: "uri", type: "string" } }, type: "object" },
	ErrorDetail: { additionalProperties: false, properties: { location: { description: "Where the error occurred, e.g. 'body.items[3].tags' or 'path.thing-id'", type: "string" }, message: { description: "Error message text", type: "string" }, value: { description: "The value at the given location" } }, type: "object" },
};
const problemContent = { "application/problem+json": { schema: { $ref: "#/components/schemas/ErrorModel" } } };

/**
 * The document as the standard has it, from oRPC as go/lib/serve writes it from Huma
 * (docs/self/standard.md): every refusal declared as problem details (401 when the operation needs
 * credentials, 422 when it takes input, its 429 when it is limited, any other it declares, and a
 * default), a parameter's description on it as well as on its schema, a file as
 * `contentMediaType: application/octet-stream`, and none of oRPC's allowEmptyValue or allowReserved.
 */
export function standardDocument<T extends OpenAPIV3_1.OpenAPIObject>(doc: T): T {
	let refuses = false;
	for (const item of Object.values(doc.paths ?? {})) {
		for (const value of Object.values(item ?? {})) {
			if (!value || typeof value !== "object" || !("responses" in value)) continue;
			const op = value as OpenAPIV3_1.OperationObject;
			for (const param of (op.parameters ?? []) as OpenAPIV3_1.ParameterObject[]) {
				delete param.allowEmptyValue;
				delete (param as { allowReserved?: boolean }).allowReserved;
				const description = (param.schema as { description?: string } | undefined)?.description;
				if (description && !param.description) param.description = description;
			}
			for (const media of Object.values((op.requestBody as OpenAPIV3_1.RequestBodyObject | undefined)?.content ?? {})) media.schema = binaryFiles(media.schema);
			const responses = op.responses as Record<string, OpenAPIV3_1.ResponseObject>;
			const security = (op.security ?? doc.security) as Security | undefined;
			const refusals: Record<string, OpenAPIV3_1.ResponseObject> = {};
			if (security?.some(requirement => Object.keys(requirement).length > 0)) refusals["401"] = { description: reasonOf(401)! };
			if (op.parameters?.length || op.requestBody) refusals["422"] = { description: reasonOf(422)! };
			for (const [status, response] of Object.entries(responses)) {
				if (/^[23]/.test(status)) continue;
				delete responses[status];
				if (/^[4]\d\d$/.test(status)) refusals[status] = { ...response, description: status === "429" ? response.description : (reasonOf(status) ?? response.description) };
			}
			refusals.default = { description: "Error" };
			for (const [status, response] of Object.entries(refusals)) responses[status] = { ...response, content: problemContent };
			refuses = true;
		}
	}
	if (refuses) {
		const components = (doc.components ??= {});
		components.schemas = { ...problemSchemas, ...components.schemas } as never;
	}
	return doc;
}

/** A schema with its files said the 3.1 way: oRPC writes `contentEncoding: binary`, Huma `contentMediaType`. */
function binaryFiles<T>(schema: T): T {
	if (Array.isArray(schema)) return schema.map(binaryFiles) as T;
	if (!schema || typeof schema !== "object") return schema;
	const out: Record<string, unknown> = {};
	for (const [key, value] of Object.entries(schema)) {
		if (key === "contentEncoding" && value === "binary") out.contentMediaType = "application/octet-stream";
		else out[key] = binaryFiles(value);
	}
	return out as T;
}
