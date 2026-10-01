// Writes the API's OpenAPI and AsyncAPI specs from the contract, offline (no Worker needed), for Fern:
//   node spec.ts [--check] <openapi.json> <asyncapi.json> [server-url]
// With --check it writes nothing and fails if either file differs from what the contract gives:
// someone changed the contract and didn't run `mise run api:spec`.
// The same generators the Worker serves /api/openapi.json and /api/asyncapi.json with (src/specs.ts).
import { readFileSync, writeFileSync } from "node:fs";
import { contract } from "./src/contract.ts";
import { asyncapiSpec, openapiSpec } from "./src/specs.ts";

const args = process.argv.slice(2);
const check = args[0] === "--check";
const [openapiOut, asyncapiOut, server = "https://api.example.com"] = check ? args.slice(1) : args;
const openapi = await openapiSpec(contract, server);
const asyncapi = await asyncapiSpec(contract, server);
for (const [file, spec] of [[openapiOut, openapi], [asyncapiOut, asyncapi]] as const) {
	const text = JSON.stringify(spec, null, 2) + "\n";
	if (!check) writeFileSync(file, text);
	else if (readFileSync(file, "utf8") !== text) { console.error(`${file} is stale: mise run api:spec`); process.exit(1); }
}
console.log(check ? "specs match the contract" : `${openapiOut}: ${Object.keys(openapi.paths ?? {}).length} paths; ${asyncapiOut}: ${Object.keys(asyncapi.channels).length} channels (${server})`);
