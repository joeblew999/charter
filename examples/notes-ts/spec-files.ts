// The command behind a spec.ts: write an API's OpenAPI and AsyncAPI specs for Fern, or check them.
//   node spec.ts [--check] <openapi.json> <asyncapi.json> [server-url]
// With --check it writes nothing and fails if either file differs from what the contract gives:
// someone changed the contract and didn't run the task that writes them.
import { readFileSync, writeFileSync } from "node:fs";

export interface Specs {
	openapi: (server: string) => Promise<{ paths?: object }>;
	asyncapi: (server: string) => Promise<{ channels: object }>;
}

/** `task` is the mise task that writes the files, named when one is stale. */
export async function specFiles(task: string, specs: Specs, args = process.argv.slice(2)) {
	const check = args[0] === "--check";
	const [openapiOut, asyncapiOut, server = "https://api.example.com"] = check ? args.slice(1) : args;
	if (!openapiOut || !asyncapiOut) throw new Error("usage: node spec.ts [--check] <openapi.json> <asyncapi.json> [server-url]");
	const openapi = await specs.openapi(server);
	const asyncapi = await specs.asyncapi(server);
	for (const [file, spec] of [[openapiOut, openapi], [asyncapiOut, asyncapi]] as const) {
		const text = JSON.stringify(spec, null, 2) + "\n";
		if (!check) writeFileSync(file, text);
		else if (readFileSync(file, "utf8") !== text) { console.error(`${file} is stale: mise run ${task}`); process.exit(1); }
	}
	console.log(check ? "specs match the contract" : `${openapiOut}: ${Object.keys(openapi.paths ?? {}).length} paths; ${asyncapiOut}: ${Object.keys(asyncapi.channels).length} channels (${server})`);
}
