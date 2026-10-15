// The conformance's two specs from its contract (charter-lib/serve's specsOf), for npm run spec.
// `server` is where the API is served: the deployed second Worker (mode api), whose URL the file's
// servers name with what it is.
import { specsOf } from "charter-lib/serve";
import { contract, document } from "./contract.ts";

const described = (server: string) =>
	specsOf(contract, { ...document, base: { ...document.base, servers: [{ url: server, description: "The deployed API (mode api). Fern's CLI takes its OAuth token URL from here; --base-url doesn't move it." }] } });

export const specs = {
	openapi: (server: string) => described(server).openapi(server),
	asyncapi: (server: string) => described(server).asyncapi(server),
};
