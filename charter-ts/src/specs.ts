// An API's OpenAPI and AsyncAPI specs for Fern, written from its contract into its fern/ folder, or
// checked: with check, nothing is written, and a committed spec that differs from what the contract
// gives fails (someone changed the contract and didn't run npm run spec).
import { readFileSync, writeFileSync } from "node:fs";

/** Both specs of an API, for the server it is served at. */
export interface Specs {
	openapi: (server: string) => Promise<{ paths?: object }>;
	asyncapi: (server: string) => Promise<{ channels: object }>;
}

/** writeSpecs writes both specs into fern/, naming server as the API's; with check it writes nothing,
 * and fails if a committed one is not what the contract gives. */
export async function writeSpecs(specs: Specs, { server, check = false }: { server: string; check?: boolean }): Promise<void> {
	const openapi = await specs.openapi(server);
	const asyncapi = await specs.asyncapi(server);
	for (const [file, spec] of [["fern/openapi.json", openapi], ["fern/asyncapi.json", asyncapi]] as const) {
		const text = JSON.stringify(spec, null, 2) + "\n";
		if (!check) writeFileSync(file, text);
		else if (readFileSync(file, "utf8") !== text) throw new Error(`${file} is not what the contract gives: npm run spec`);
	}
	console.log(check ? "specs match the contract" : `fern/openapi.json: ${Object.keys(openapi.paths ?? {}).length} paths; fern/asyncapi.json: ${Object.keys(asyncapi.channels).length} channels (${server})`);
}
