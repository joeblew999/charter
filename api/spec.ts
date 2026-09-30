// Writes the API's OpenAPI and AsyncAPI specs from the contract, offline (no Worker needed), for Fern:
//   node spec.ts <openapi.json> <asyncapi.json> [server-url]
// The same generators the Worker serves /api/openapi.json and /api/asyncapi.json with (src/specs.ts).
import { writeFileSync } from "node:fs";
import { contract } from "./src/contract.ts";
import { asyncapiSpec, openapiSpec } from "./src/specs.ts";

const [openapiOut, asyncapiOut, server = "https://api.example.com"] = process.argv.slice(2);
const openapi = await openapiSpec(contract, server);
const asyncapi = await asyncapiSpec(contract, server);
writeFileSync(openapiOut, JSON.stringify(openapi, null, 2) + "\n");
writeFileSync(asyncapiOut, JSON.stringify(asyncapi, null, 2) + "\n");
console.log(`${openapiOut}: ${Object.keys(openapi.paths ?? {}).length} paths; ${asyncapiOut}: ${Object.keys(asyncapi.channels).length} channels (${server})`);
