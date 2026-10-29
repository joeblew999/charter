import { accessScheme, oidcScheme, scheme } from "charter-lib/auth";
import { type Document, specsOf } from "charter-lib/serve";
import { contract, info } from "./contract.ts";

// What both specs say beside the routes (charter-lib/serve), and the specs themselves: what the Worker
// serves at /api/openapi.json and /api/asyncapi.json, and what charter spec writes.

/** The API's name, its channels', and who may call: bearer tokens, Cloudflare Access and an OpenID
 * Connect issuer (api/handlers.ts). */
export const document: Document = {
	info,
	asyncInfo: { title: "charter-notes-ts live", version: "1.0.0" },
	base: oidcScheme(accessScheme(info.title, scheme())),
};

export const specs = specsOf(contract, document);
