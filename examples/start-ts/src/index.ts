import { env } from "cloudflare:workers";
import { OpenAPIHandler } from "@orpc/openapi/fetch";
import { implement } from "@orpc/server";
import { asyncInfo, contract, info } from "./contract.ts";
import { asyncapiSpec, openapiSpec } from "@charter/ts/specs";

// The contract (src/contract.ts) implemented, served by oRPC's OpenAPIHandler as plain REST. The
// D1 database is env.DB (cloudflare.config.ts).
const api = implement(contract);

export const router = api.router({
	hello: api.hello.handler(() => ({ message: `Hello from ${env.APP_NAME}` })),
});

const handler = new OpenAPIHandler(router);

export default {
	async fetch(request) {
		const url = new URL(request.url);
		// The specs, generated from the same router as fern/{openapi,asyncapi}.json.
		if (url.pathname === "/api/openapi.json") return Response.json(await openapiSpec(router, { info, server: url.origin }));
		if (url.pathname === "/api/asyncapi.json") return Response.json(await asyncapiSpec(router, { info: asyncInfo, server: url.origin }));
		const { matched, response } = await handler.handle(request, { context: {} });
		return matched ? response : new Response("not found", { status: 404 });
	},
} satisfies ExportedHandler;
