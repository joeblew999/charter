import { type Document, specsOf } from "charter-lib/serve";
import { contract } from "./contract.ts";

// What both specs say beside the routes (charter-lib/serve), and the specs themselves: what the Worker
// serves at /api/openapi.json and /api/asyncapi.json, and what npm run spec writes (charter.config.ts).

/** The API's name, and its channels'. */
export const document: Document = {
	info: { title: "charter-start-ts", version: "1.0.0", description: "charter-start-ts: contract -> OpenAPI -> Fern." },
	asyncInfo: { title: "charter-start-ts live", version: "1.0.0" },
};

export const specs = specsOf(contract, document);
