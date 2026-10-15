// An API's OpenAPI and AsyncAPI specs for Fern, written from its contract into its fern/ folder, or
// checked: with check, nothing is written, and a committed spec that differs from what the contract
// gives fails (someone changed the contract and didn't run npm run spec).
import { readFileSync, writeFileSync } from "node:fs";

/** Both specs of an API, for the server it is served at. */
export interface Specs {
	openapi: (server: string) => Promise<{ paths?: object }>;
	asyncapi: (server: string) => Promise<{ channels: object }>;
}

export interface SpecFiles {
	/** Where the API is served: the specs name it as their server. */
	server: string;
	/** Check the committed files instead of writing them. */
	check?: boolean;
}

/** writeSpecs writes both specs, or with check fails (exit code 1) if a committed one is stale. */
export async function writeSpecs(specs: Specs, { server, check = false }: SpecFiles): Promise<void> {
	const openapi = await specs.openapi(server);
	const asyncapi = await specs.asyncapi(server);
	for (const [file, spec] of [["fern/openapi.json", openapi], ["fern/asyncapi.json", asyncapi]] as const) {
		const text = JSON.stringify(spec, null, 2) + "\n";
		if (!check) writeFileSync(file, text);
		else if (readFileSync(file, "utf8") !== text) {
			console.error(`${file} is stale: npm run spec`);
			process.exit(1);
		}
	}
	console.log(check ? "specs match the contract" : `fern/openapi.json: ${Object.keys(openapi.paths ?? {}).length} paths; fern/asyncapi.json: ${Object.keys(asyncapi.channels).length} channels (${server})`);
}
