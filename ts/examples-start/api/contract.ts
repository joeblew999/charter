import { oc } from "@orpc/contract";
import { openapi, type OpenAPIV3_2 } from "@orpc/openapi";
import { z } from "zod";

// The API, contract first: every route with its method, path, input and output as Zod 4 schemas.
// The Worker implements it (api/handlers.ts) and the specs are generated from it (npm run spec ->
// fern/openapi.json, fern/asyncapi.json), from which Fern makes SDKs, a CLI and docs. Everything the
// SDKs need is said here too: operationId/tags name the SDK methods, and `spec` adds Fern's
// extensions (x-fern-*) to the generated operation. Add a route here, then: npm run spec.

/** Fern's names for the SDK method: client.<group>.<method>() and `cli <group> <method>`. */
const sdk = (group: string, method: string, extra: Record<string, unknown> = {}) =>
	(op: OpenAPIV3_2.OperationObject): OpenAPIV3_2.OperationObject => ({ ...op, "x-fern-sdk-group-name": group, "x-fern-sdk-method-name": method, ...extra });

/** What GET /api/hello answers. */
export const hello = z.object({ message: z.string().meta({ examples: ["Hello from charter-start-ts"] }) });

export const contract = {
	hello: oc
		.meta(openapi({ method: "GET", path: "/api/hello", summary: "Say hello", tags: ["meta"], operationId: "hello", spec: sdk("meta", "hello") }))
		.output(hello),
};
