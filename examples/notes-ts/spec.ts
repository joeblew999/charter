// Writes the API's OpenAPI and AsyncAPI specs from the contract, offline (no Worker needed), for Fern:
//   node spec.ts [--check] <openapi.json> <asyncapi.json> [server-url]
// The same generators the Worker serves /api/openapi.json and /api/asyncapi.json with (@charter/ts/specs).
import { asyncInfo, contract, info } from "./src/contract.ts";
import { specFiles } from "@charter/ts/spec-files";
import { asyncapiSpec, openapiSpec } from "@charter/ts/specs";
import { accessScheme, oidcScheme, scheme } from "@charter/ts/auth";

// Who may call: bearer tokens, Cloudflare Access and an OpenID Connect issuer (src/index.ts).
const base = oidcScheme(accessScheme(info.title, scheme()));

await specFiles("spec", {
	openapi: server => openapiSpec(contract, { info, server, base }),
	asyncapi: server => asyncapiSpec(contract, { info: asyncInfo, server }),
});
