// Writes the showcase API's OpenAPI and AsyncAPI specs from its contract (src/contract.ts), for Fern:
//   node spec.ts [--check] <openapi.json> <asyncapi.json> [server-url]
// The command is the library's (@charter/ts/spec-files, in ../../ts); only the contract differs.
import { specFiles } from "@charter/ts/spec-files";
import { specs } from "./src/specs.ts";

await specFiles("spec", specs);
