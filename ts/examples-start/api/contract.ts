import { oc } from "@orpc/contract";
import { openapi, type OpenAPIV3_2 } from "@orpc/openapi";
import { hello } from "./schemas.ts";

// The API, contract first: every route with its method, path, input and output as Zod 4 schemas.
// The Worker implements it (api/handlers.ts) and the specs are generated from it (npm run spec ->
// fern/openapi.json, fern/asyncapi.json), from which Fern makes the SDKs. Everything the
// SDKs need is said here too: operationId/tags name the SDK methods, and `spec` adds Fern's
// extensions (x-fern-*) to the generated operation. Add a route here, then: npm run spec.

/** The API's name, version and what it is: the specs' info (api/spec.ts). */
export const info = { title: "charter-start-ts", version: "1.0.0", description: "charter-start-ts: contract -> OpenAPI -> Fern." };

/** Fern's names for the SDK method: client.<group>.<method>(). */
const sdk = (group: string, method: string, extra: Record<string, unknown> = {}) =>
	(op: OpenAPIV3_2.OperationObject): OpenAPIV3_2.OperationObject => ({ ...op, "x-fern-sdk-group-name": group, "x-fern-sdk-method-name": method, ...extra });

export const contract = {
	hello: oc
		.meta(openapi({ method: "GET", path: "/api/hello", summary: "Say hello", tags: ["meta"], operationId: "hello", spec: sdk("meta", "hello") }))
		.output(hello),
};
