// Writes the API's OpenAPI and AsyncAPI specs from the contract, offline (no Worker needed), for Fern:
//   node spec.ts [--check] <openapi.json> <asyncapi.json> [server-url]
// The same generators the Worker serves /api/openapi.json and /api/asyncapi.json with (src/specs.ts).
import { asyncInfo, contract, info } from "./src/contract.ts";
import { specFiles } from "./spec-files.ts";
import { asyncapiSpec, openapiSpec } from "./src/specs.ts";

await specFiles("api:ts:spec", {
	openapi: server => openapiSpec(contract, { info, server }),
	asyncapi: server => asyncapiSpec(contract, { info: asyncInfo, server }),
});
