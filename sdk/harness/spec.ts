// Writes the showcase API's OpenAPI and AsyncAPI specs from its contract (src/contract.ts), for Fern:
//   node spec.ts [--check] <openapi.json> <asyncapi.json> [server-url]
// The command is api/ts/'s (api/ts/spec-files.ts); only the contract differs.
import { specFiles } from "../../api/ts/spec-files.ts";
import { specs } from "./src/specs.ts";

await specFiles("showcase:spec", specs);
