// Both specs from one contract or router, for spec.ts (offline, into sdk/fern/apis/api/) and for the
// Worker (/api/openapi.json, /api/asyncapi.json). No Workers APIs here, so Node can run it.
import { OpenAPIGenerator } from "@orpc/openapi";
import type { RouterContract } from "@orpc/contract";
import type { AnyRouter } from "@orpc/server";
import { ZodToJsonSchemaConverter } from "@orpc/zod";
import { AsyncAPIGenerator, getAsyncAPIMeta } from "./asyncapi.ts";
import { asyncInfo, info } from "./contract.ts";

const converters = [new ZodToJsonSchemaConverter()];

/** OpenAPI 3.1.1 (oRPC 2.0 defaults to 3.2.0, which Fern rejects), without the WebSocket channels. */
// Upstream: fern-api/fern#9559 (when fixed: drop `version` and use oRPC's 3.2.0 default)
export const openapiSpec = (router: RouterContract | AnyRouter, server: string) =>
	new OpenAPIGenerator({ converters }).generate(router, {
		version: "3.1.1",
		base: { info, servers: [{ url: server }] },
		filter: procedure => !getAsyncAPIMeta(procedure as any),
	});

/** AsyncAPI 3.0.0: the WebSocket channels. */
export const asyncapiSpec = (router: RouterContract | AnyRouter, server: string) => {
	const url = new URL(server);
	return new AsyncAPIGenerator({ converters }).generate(router, {
		base: { info: asyncInfo, servers: { production: { host: url.host, protocol: url.protocol === "http:" ? "ws" : "wss" } } },
	});
};
