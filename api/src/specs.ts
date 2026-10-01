// Both specs from one contract or router: for a spec.ts (offline, into a Fern folder) and for a
// Worker (/api/openapi.json, /api/asyncapi.json). It knows no contract: the caller passes its `info`
// and URL. No Workers APIs and no files here, so Node and a Worker can both run it.
import { getOpenAPIMeta, OpenAPIGenerator, type OpenAPIV3_2 } from "@orpc/openapi";
import type { RouterContract } from "@orpc/contract";
import { DelegatingJsonSchemaConverter, mapJsonSchemaRefs, type JsonSchema } from "@orpc/json-schema";
import { walkProcedureContractsAsync, type AnyRouter } from "@orpc/server";
import { ZodToJsonSchemaConverter } from "@orpc/zod";
import { AsyncAPIGenerator, getAsyncAPIMeta } from "./asyncapi.ts";

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
}

/** OpenAPI 3.1.1 (oRPC 2.0 defaults to 3.2.0, which Fern rejects), without the WebSocket channels. */
// Upstream: fern-api/fern#9559 (when fixed: drop `version` and use oRPC's 3.2.0 default)
export const openapiSpec = async (router: RouterContract | AnyRouter, { info, server, base, webhooks }: OpenAPISpecOptions) => {
	const doc = await new OpenAPIGenerator({ converters }).generate(router, {
		version: "3.1.1",
		base: { info, servers: [{ url: server }], ...base },
		filter: procedure => !getAsyncAPIMeta(procedure as any),
	});
	return webhooks ? addWebhooks(doc, webhooks) : doc;
};

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
export const asyncapiSpec = (router: RouterContract | AnyRouter, { info, server }: AsyncAPISpecOptions) => {
	const url = new URL(server);
	const pathname = url.pathname.replace(/\/$/, "");
	return new AsyncAPIGenerator({ converters }).generate(router, {
		base: { info, servers: { production: { host: url.host, ...(pathname && { pathname }), protocol: url.protocol === "http:" ? "ws" : "wss" } } },
	});
};
