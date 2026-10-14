// The notes API exists twice here, from a Go contract (go/notes) and from an oRPC one (ts/notes).
// Each project's check holds its committed specs to its contract (spec:check); these tests hold the
// two to each other: the same SDK surface, the same database, the same ways in. They are the repo's,
// not a project's: a project has one contract.
//   node --test surface/*.test.ts   (npm run check runs them)
import assert from "node:assert/strict";
import { readdirSync, readFileSync } from "node:fs";
import { test } from "node:test";

type Json = Record<string, unknown>;
const read = (file: string): Json => JSON.parse(readFileSync(file, "utf8")) as Json;
const at = (doc: unknown, ...path: string[]): unknown => path.reduce<unknown>((node, key) => (node && typeof node === "object" ? (node as Json)[key] : undefined), doc);
const asMap = (value: unknown): Json => (value && typeof value === "object" && !Array.isArray(value) ? (value as Json) : {});

/** What Fern builds an SDK's methods from: per operation its id, its parameters (name, place,
 * constraints), its x-fern-* extensions and the media types of its 200. */
function surface(doc: Json): Map<string, string> {
	const out = new Map<string, string>();
	for (const [path, item] of Object.entries(asMap(doc.paths))) {
		for (const [method, raw] of Object.entries(asMap(item))) {
			const op = asMap(raw);
			if (Object.keys(op).length === 0) continue;
			const entry: Json = { operationId: op.operationId, tags: op.tags, summary: op.summary };
			for (const [key, value] of Object.entries(op)) if (key.startsWith("x-fern-")) entry[key] = value;
			entry.parameters = ((op.parameters as Json[] | undefined) ?? [])
				.map(param => {
					const schema = asMap(param.schema);
					return `${param.in} ${param.name} ${JSON.stringify({ type: schema.type, min: schema.minimum, max: schema.maximum, default: schema.default, pattern: schema.pattern })}`;
				})
				.sort();
			entry["200"] = Object.keys(asMap(at(op, "responses", "200", "content")));
			out.set(`${method.toUpperCase()} ${path}`, JSON.stringify(sorted(entry)));
		}
	}
	return out;
}

/** Keys in order, so two entries compare as text whatever order their specs wrote them in. */
function sorted(value: unknown): unknown {
	if (Array.isArray(value)) return value.map(sorted);
	if (value && typeof value === "object") return Object.fromEntries(Object.keys(value).sort().map(key => [key, sorted((value as Json)[key])]));
	return value;
}

test("the Go and the oRPC contract give the SDKs the same operations and channels", () => {
	const want = surface(read("ts/notes/fern/openapi.json")), got = surface(read("go/notes/fern/openapi.json"));
	for (const [key, w] of want) assert.equal(got.get(key), w, `${key}\n  oRPC: ${w}\n  Go:   ${got.get(key)}`);
	for (const key of got.keys()) assert.ok(want.has(key), `${key} is in the Go contract only`);
	const orpc = read("ts/notes/fern/asyncapi.json"), go = read("go/notes/fern/asyncapi.json");
	for (const path of [
		["channels", "liveNotes", "address"],
		["channels", "liveNotes", "summary"],
		["channels", "liveNotes", "bindings", "ws", "query", "properties", "after"],
		["operations", "receiveNote"],
	]) {
		// Examples are left out: the Go contract gives them (`example` tags) and the oRPC one doesn't,
		// and they change no SDK method.
		const property = at(go, ...path);
		if (property && typeof property === "object") delete (property as Json).examples;
		assert.equal(JSON.stringify(sorted(at(go, ...path))), JSON.stringify(sorted(at(orpc, ...path))), `asyncapi ${path.join(".")}`);
	}
});

test("both notes projects serve the same API from the same table: their migrations/ are the same files", () => {
	const files = (dir: string) => new Map(readdirSync(dir).map(name => [name, readFileSync(`${dir}/${name}`, "utf8")]));
	const go = files("go/notes/migrations"), ts = files("ts/notes/migrations");
	assert.ok(go.size > 0, "go/notes/migrations: no files");
	assert.deepEqual([...go.keys()].sort(), [...ts.keys()].sort(), "the same migration files");
	for (const [name, content] of go) assert.equal(ts.get(name), content, `migrations/${name} differs between go/notes and ts/notes`);
});

// Fern sends only the credentials fern/generators.yml restates (auth-schemes, auth: any), so each
// notes project restates its spec's bearer and Access schemes, with the same headers and variables
// (an OIDC token goes as the bearer token), and the two declare the same schemes and security.
test("Fern restates the schemes, and both notes projects declare the same ones", () => {
	const security = new Map<string, string>();
	for (const project of ["go/notes", "ts/notes"]) {
		const doc = read(`${project}/fern/openapi.json`);
		const yml = readFileSync(`${project}/fern/generators.yml`, "utf8");
		const schemes = asMap(at(doc, "components", "securitySchemes"));
		assert.ok(Object.keys(schemes).length === 4 && schemes.oidc, `${project}: want bearer, Access's two and oidc`);
		for (const [name, raw] of Object.entries(schemes)) {
			if (name === "oidc") continue;
			const scheme = asMap(raw);
			const want = [`    ${name}:\n`, `      - ${name}\n`];
			if (scheme.type === "apiKey") {
				const fern = asMap(scheme["x-fern-header"]);
				want.push(`header: ${scheme.name}\n`, `name: ${fern.name}\n`, `env: ${fern.env}\n`);
			}
			for (const w of want) assert.ok(yml.includes(w), `${project}/fern/generators.yml: the scheme ${name} has no ${JSON.stringify(w.trim())}`);
		}
		assert.ok(yml.includes("  auth:\n    any:\n"), `${project}/fern/generators.yml: auth is not any`);
		security.set(project, `${Object.keys(schemes).sort().join(" ")} ${JSON.stringify(at(doc, "paths", "/api/notes", "post", "security"))}`);
	}
	assert.equal(security.get("go/notes"), security.get("ts/notes"), "the schemes and createNote's security differ");
});
