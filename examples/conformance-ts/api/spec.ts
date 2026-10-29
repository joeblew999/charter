// The conformance's document, and its two specs from its contract (charter-lib/serve's specsOf), for npm run spec.
// `server` is where the API is served: the deployed second Worker (mode api), whose URL the file's
// servers name with what it is.
import { type Document, specsOf } from "charter-lib/serve";
import { base, contract, info, webhooks } from "./contract.ts";

/** What both specs say beside the routes (charter-lib/serve): the API's name, its channel's, the
 * document's fields, and the webhooks. */
export const document: Document = { info, asyncInfo: { title: "Conformance live", version: "1.0.0" }, base, webhooks };

/** The specs for a server, which they name with what it is. */
const described = (server: string) =>
	specsOf(contract, { ...document, base: { ...document.base, servers: [{ url: server, description: "The deployed API (mode api). Fern's CLI takes its OAuth token URL from here; --base-url doesn't move it." }] } });

export const specs = {
	openapi: (server: string) => described(server).openapi(server),
	asyncapi: (server: string) => described(server).asyncapi(server),
};
