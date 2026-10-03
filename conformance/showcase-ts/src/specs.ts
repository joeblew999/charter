// The showcase's two specs from its contract, with the library's generators (@charter/ts/specs, in ../../ts).
// `server` is where the API is served: the deployed harness, .../api/mock.
import { asyncapiSpec, openapiSpec } from "@charter/ts/specs";
import { asyncInfo, contract, document, info, webhooks } from "./contract.ts";

export const specs = {
	openapi: (server: string) => openapiSpec(contract, {
		info,
		server,
		base: { ...document, servers: [{ url: server, description: "The deployed mock of this API (mode api). Fern's CLI takes its OAuth token URL from here; --base-url doesn't move it." }] },
		webhooks,
	}),
	asyncapi: (server: string) => asyncapiSpec(contract, { info: asyncInfo, server }),
};
