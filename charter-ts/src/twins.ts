// charter-ts twins <reference> <other>: the same API in two projects (a TypeScript one and its Go twin)
// must answer the same. Both run under workerd, as they are deployed (a Go Worker built first), each on
// a free port with local state of its own. Both get the same requests, in the same order, and every
// answer that differs is printed (its status, the headers a client reads, its body); it fails if any
// does. The requests come from the reference's own specs, so they need no upkeep: for each operation,
// the right request and each kind of wrong one (no credentials, too little scope, each parameter
// missing, of the wrong type or out of bounds, a body that is not JSON, of a media type the operation
// does not take, with a property it does not have, a method the path does not have); for each channel,
// a request that is no upgrade; and what every API serves (docs/reference/standard.md): /.well-known/,
// MCP, both specs, an unknown path.
import { mkdtempSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { join, resolve } from "node:path";
import { pathToFileURL } from "node:url";
import type { Config } from "./config.ts";
import { freePort, migrateLocal, run, withServer } from "./scripts.ts";

type Answer = { status: number; headers: Record<string, string>; body: unknown };
type Auth = "none" | "read" | "write";
interface Probe {
	name: string;
	method?: string;
	path: string;
	auth?: Auth;
	headers?: Record<string, string>;
	body?: () => string | URLSearchParams | FormData;
}
type Schema = { type?: string | string[]; properties?: Record<string, Schema>; required?: string[]; items?: Schema; $ref?: string; examples?: unknown[]; default?: unknown; minimum?: number; maximum?: number; minLength?: number; pattern?: string; enum?: unknown[]; contentMediaType?: string; format?: string };
type Parameter = { name: string; in: string; required?: boolean; schema?: Schema };
type Operation = { operationId?: string; parameters?: Parameter[]; requestBody?: { required?: boolean; content?: Record<string, { schema?: Schema }> }; responses?: Record<string, { content?: Record<string, unknown> }>; security?: Record<string, string[]>[]; "x-fern-idempotent"?: boolean };
type OpenAPI = { paths?: Record<string, Record<string, Operation>>; components?: { schemas?: Record<string, Schema> }; security?: Record<string, string[]>[]; "x-fern-idempotency-headers"?: { header: string }[] };

/** The headers a client reads. */
const read = ["content-type", "allow", "www-authenticate", "retry-after", "location", "cache-control"];
const json = (value: unknown) => () => JSON.stringify(value);
const methods = ["get", "put", "post", "delete", "patch"];

/** A valid value for a schema: its example, its default, or the least the schema allows. */
function valueOf(schema: Schema | undefined, spec: OpenAPI): unknown {
	const s = resolved(schema, spec);
	if (!s) return "x";
	if (s.examples?.length) return s.examples[0];
	if (s.default !== undefined) return s.default;
	if (s.enum?.length) return s.enum[0];
	const type = Array.isArray(s.type) ? s.type.find(t => t !== "null") : s.type;
	if (type === "integer" || type === "number") return s.minimum ?? 1;
	if (type === "boolean") return true;
	if (type === "array") return [valueOf(s.items, spec)];
	if (type === "object") return Object.fromEntries((s.required ?? []).map(name => [name, valueOf(s.properties?.[name], spec)]));
	return "x".repeat(Math.max(1, s.minLength ?? 1));
}

const resolved = (schema: Schema | undefined, spec: OpenAPI): Schema | undefined =>
	schema?.$ref ? resolved(spec.components?.schemas?.[schema.$ref.split("/").pop()!], spec) : schema;

const typeOf = (schema: Schema | undefined) => (Array.isArray(schema?.type) ? schema.type.find(t => t !== "null") : schema?.type);

/** A value of the wrong type for a schema. */
const wrongType = (schema: Schema | undefined) => (typeOf(schema) === "string" ? 5 : "x");

/** The probes of one operation, from what the spec says of it. */
function probesOf(path: string, method: string, op: Operation, spec: OpenAPI, item: Record<string, Operation>): Probe[] {
	const label = op.operationId ?? `${method.toUpperCase()} ${path}`;
	const security = op.security ?? spec.security ?? [];
	const secured = security.some(requirement => Object.keys(requirement).length > 0);
	const needsWrite = security.some(requirement => Object.values(requirement).some(scopes => scopes.includes("write")));
	const auth: Auth = secured ? "write" : "none";
	const params = op.parameters ?? [];
	const stream = Object.keys(op.responses?.["200"]?.content ?? {}).includes("text/event-stream");
	// A stream ends when its integer parameters are at their least (seconds=1).
	const value = (p: Parameter) => (stream && typeOf(p.schema) === "integer" && p.schema?.minimum !== undefined ? p.schema.minimum : valueOf(p.schema, spec));
	const query = (values: Record<string, unknown>) => {
		const search = new URLSearchParams(Object.entries(values).filter(([, v]) => v !== undefined).map(([k, v]): [string, string] => [k, String(v)])).toString();
		let filled = path;
		for (const p of params.filter(p => p.in === "path")) filled = filled.replace(`{${p.name}}`, encodeURIComponent(String(values[p.name] ?? value(p))));
		return search ? `${filled}?${search}` : filled;
	};
	const good = Object.fromEntries(params.filter(p => p.in === "query" && (p.required || stream)).map(p => [p.name, value(p)]));
	const media = Object.keys(op.requestBody?.content ?? {})[0];
	const bodySchema = resolved(op.requestBody?.content?.[media ?? ""]?.schema, spec);
	const goodBody = media ? valueOf(bodySchema, spec) as Record<string, unknown> : undefined;
	const encode = (body: unknown, type = media): Probe["body"] => () => {
		if (type === "application/x-www-form-urlencoded") return new URLSearchParams(Object.entries(body as object).map(([k, v]): [string, string] => [k, String(v)]));
		if (type === "multipart/form-data") {
			const form = new FormData();
			for (const [k, v] of Object.entries(body as object)) form.append(k, typeof v === "string" && resolved(bodySchema?.properties?.[k], spec)?.contentMediaType ? new File([v], "file.txt", { type: "text/plain" }) : String(v));
			return form;
		}
		return JSON.stringify(body);
	};
	const typed = (type?: string): Record<string, string> => (type && type !== "multipart/form-data" ? { "content-type": type } : {});
	// The idempotency key the document names: the same key, so the same id, on both.
	const keyHeader = op["x-fern-idempotent"] ? spec["x-fern-idempotency-headers"]?.[0]?.header : undefined;
	const key = (n: string): Record<string, string> => (keyHeader ? { [keyHeader]: `twins-${label}-${n}` } : {});
	const call = (name: string, values: Record<string, unknown>, extra: Partial<Probe> = {}): Probe => ({
		name: `${label}: ${name}`, method: method.toUpperCase(), path: query(values), auth, ...(media && { headers: { ...typed(media), ...key(name) }, body: encode(goodBody) }), ...extra,
	});
	const probes: Probe[] = [call("as the spec has it", good)];
	if (secured) probes.push(call("no credentials", good, { auth: "none" }));
	if (needsWrite) probes.push(call("the read token", good, { auth: "read" }));
	for (const p of params.filter(p => p.in === "query" || p.in === "path")) {
		const s = resolved(p.schema, spec);
		if (p.required) probes.push(call(`no ${p.name}`, { ...good, [p.name]: undefined }));
		probes.push(call(`${p.name} of the wrong type`, { ...good, [p.name]: typeOf(s) === "string" ? undefined : "x" }));
		if (s?.minimum !== undefined) probes.push(call(`${p.name} under its minimum`, { ...good, [p.name]: s.minimum - 1 }));
		if (s?.maximum !== undefined) probes.push(call(`${p.name} over its maximum`, { ...good, [p.name]: s.maximum + 1 }));
		if (s?.pattern) probes.push(call(`${p.name} not matching its pattern`, { ...good, [p.name]: "!" }));
	}
	if (media && goodBody && typeof goodBody === "object") {
		const others = Object.keys(op.requestBody?.content ?? {});
		probes.push(call("an empty body", good, { headers: typed(media), body: () => "" }));
		probes.push(call("a property it does not have", good, { headers: { ...typed(media), ...key("unknown") }, body: encode({ ...goodBody, twins: 1 }) }));
		for (const name of bodySchema?.required ?? []) {
			const { [name]: _, ...rest } = goodBody;
			probes.push(call(`no ${name}`, good, { headers: typed(media), body: encode(rest) }));
			if (media === "application/json") probes.push(call(`${name} of the wrong type`, good, { headers: typed(media), body: encode({ ...goodBody, [name]: wrongType(resolved(bodySchema?.properties?.[name], spec)) }) }));
		}
		if (media === "application/json") probes.push(call("JSON that does not parse", good, { headers: typed(media), body: () => "{" }));
		for (const type of ["application/json", "text/plain"].filter(t => !others.includes(t))) probes.push(call(`a body of ${type}`, good, { headers: { "content-type": type }, body: json(goodBody) }));
	}
	const other = ["delete", "put", "patch"].find(m => !item[m]);
	if (other) probes.push({ name: `${label}: ${other.toUpperCase()}, a method the path does not have`, method: other.toUpperCase(), path: query(good), auth });
	probes.push({ name: `${label}: a trailing slash`, method: method.toUpperCase(), path: `${query({}).split("?")[0]}/`, auth });
	return probes;
}

const mcpMeta = { "io.modelcontextprotocol/protocolVersion": "2026-07-28", "io.modelcontextprotocol/clientCapabilities": {}, "io.modelcontextprotocol/clientInfo": { name: "twins", version: "1" } };
const mcp = (name: string, method: string, params: Record<string, unknown>, auth: Auth, raw?: string): Probe => ({
	name: `MCP: ${name}`, method: "POST", path: "/api/mcp", auth,
	headers: { "content-type": "application/json", accept: "application/json, text/event-stream", "mcp-protocol-version": "2026-07-28", "mcp-method": method, ...(params.name ? { "mcp-name": String(params.name) } : {}) },
	body: () => raw ?? JSON.stringify({ jsonrpc: "2.0", id: 1, method, params: { ...params, _meta: mcpMeta } }),
});

/** Every probe of an API, from its specs: what every API serves, each operation's, each channel's. */
function probesFor(spec: OpenAPI, channels: string[], tools: { name: string }[], auth: Auth): Probe[] {
	const probes: Probe[] = [
		{ name: "protected-resource metadata", path: "/.well-known/oauth-protected-resource" },
		{ name: "authorization-server metadata", path: "/.well-known/oauth-authorization-server" },
		{ name: "OpenID Connect discovery", path: "/.well-known/openid-configuration" },
		{ name: "an unknown /.well-known/ document", path: "/.well-known/twins" },
		{ name: "an unknown path under /api", path: "/api/twins" },
		{ name: "/api itself", path: "/api" },
		{ name: "the root", path: "/" },
		{ name: "the OpenAPI spec", path: "/api/openapi.json" },
		{ name: "the AsyncAPI spec", path: "/api/asyncapi.json" },
		mcp("tools/list", "tools/list", {}, auth),
		mcp("a tool that is not one", "tools/call", { name: "twins", arguments: {} }, auth),
		mcp("an unknown method", "twins/twins", {}, auth),
		mcp("JSON that does not parse", "tools/list", {}, auth, '{"jsonrpc":"2.0","id":1,"method":'),
		{ name: "MCP: GET", path: "/api/mcp", auth },
	];
	for (const tool of tools) probes.push(mcp(`${tool.name} with no arguments`, "tools/call", { name: tool.name, arguments: {} }, auth), mcp(`${tool.name} without credentials`, "tools/call", { name: tool.name, arguments: {} }, "none"));
	for (const [path, item] of Object.entries(spec.paths ?? {})) {
		for (const method of methods) if (item[method]) probes.push(...probesOf(path, method, item[method], spec, item));
	}
	for (const address of channels) {
		probes.push({ name: `channel ${address}: no upgrade`, path: address, auth }, { name: `channel ${address}: no credentials`, path: address }, { name: `channel ${address}: a POST`, method: "POST", path: address, auth, headers: { "content-type": "application/json" }, body: json({}) });
	}
	return probes;
}

/** An answer with what differs only because the servers differ taken out: their origin, their name
 * (the Worker's, and as the SDKs' variables have it), a time. */
function normalize(answer: Answer, origin: string, name: string): Answer {
	const host = new URL(origin).host;
	const screaming = name.toUpperCase().replaceAll("-", "_");
	const scrub = (value: unknown): unknown => {
		if (typeof value === "string") return value.replaceAll(name, "{name}").replaceAll(screaming, "{NAME}").replaceAll(origin, "{origin}").replaceAll(origin.replace("http", "ws"), "{ws-origin}").replaceAll(host, "{host}").replace(/\b\d{4}-\d{2}-\d{2}[ T]\d{2}:\d{2}:\d{2}(\.\d+)?Z?\b/g, "{time}");
		if (Array.isArray(value)) return value.map(scrub);
		if (value && typeof value === "object") return Object.fromEntries(Object.entries(value).map(([k, v]) => [k, k === "access_token" && typeof v === "string" ? "{token}" : scrub(v)]));
		return value;
	};
	return { status: answer.status, headers: scrub(answer.headers) as Record<string, string>, body: scrub(answer.body) };
}

async function ask(origin: string, probe: Probe, tokens: Record<Auth, string | undefined>): Promise<Answer> {
	const headers: Record<string, string> = { ...probe.headers };
	const token = tokens[probe.auth ?? "none"];
	if (token) headers.authorization = `Bearer ${token}`;
	const response = await fetch(origin + probe.path, { method: probe.method ?? "GET", headers, body: probe.body?.(), redirect: "manual", signal: AbortSignal.timeout(20_000) });
	const text = await response.text();
	let body: unknown = text;
	try { body = JSON.parse(text); } catch {}
	return { status: response.status, headers: Object.fromEntries(read.flatMap(name => (response.headers.has(name) ? [[name, response.headers.get(name)!]] : []))), body };
}

/** Every place two values differ, as a path and the two values. */
function differences(a: unknown, b: unknown, path = ""): string[] {
	if (JSON.stringify(a) === JSON.stringify(b)) return [];
	const object = (v: unknown): v is Record<string, unknown> => !!v && typeof v === "object";
	if (object(a) && object(b) && Array.isArray(a) === Array.isArray(b)) {
		return [...new Set([...Object.keys(a), ...Object.keys(b)])].flatMap(key => differences(a[key], b[key], `${path}/${key}`));
	}
	const show = (v: unknown) => (v === undefined ? "(none)" : JSON.stringify(v).slice(0, 300));
	return [`${path || "/"}: ${show(a)}  |  other: ${show(b)}`];
}

const configOf = async (dir: string) => ((await import(pathToFileURL(resolve(dir, "charter.config.ts")).href)) as { default: Config }).default;

export async function twins(args: string[]): Promise<void> {
	const [reference, other] = args;
	if (!reference || !other) throw new Error("twins needs two projects, the reference first: charter-ts twins ts/conformance go/conformance");
	const [refConfig, otherConfig] = [await configOf(reference), await configOf(other)];
	const secrets = { ...otherConfig.localSecrets, ...refConfig.localSecrets };
	const state = mkdtempSync(join(tmpdir(), "charter-twins-"));
	const [refPort, otherPort] = [await freePort(), await freePort()];
	const origins = [`http://localhost:${refPort}`, `http://localhost:${otherPort}`] as const;
	for (const [dir, config] of [[reference, refConfig], [other, otherConfig]] as const) if (config.go) run("charter-ts", ["build"], {}, dir);
	const server = (dir: string, port: number, name: string) => ({ program: "cf", args: ["dev"], env: { ...secrets, PORT: String(port), CHARTER_LOCAL_STATE: join(state, name) }, ready: `http://localhost:${port}/api/openapi.json`, cwd: dir });
	try {
		await withServer(server(reference, refPort, "reference"), () => withServer(server(other, otherPort, "other"), async () => {
			await migrateLocal(refConfig, refPort, reference).catch(() => {});
			await migrateLocal(otherConfig, otherPort, other).catch(() => {});
			const spec = (await (await fetch(`${origins[0]}/api/openapi.json`)).json()) as OpenAPI;
			const asyncapi = (await (await fetch(`${origins[0]}/api/asyncapi.json`)).json()) as { channels?: Record<string, { address: string }> };
			const tokensOf = (): Record<Auth, string | undefined> => ({ none: undefined, read: secrets.READ_TOKEN, write: secrets.WRITE_TOKEN });
			const listed = await ask(origins[0], mcp("tools/list", "tools/list", {}, "write"), tokensOf());
			const tools = ((listed.body as { result?: { tools?: { name: string }[] } }).result?.tools ?? []);
			const probes = probesFor(spec, Object.values(asyncapi.channels ?? {}).map(c => c.address), tools, secrets.WRITE_TOKEN ? "write" : "none");
			let differ = 0;
			for (const probe of probes) {
				const a = normalize(await ask(origins[0], probe, tokensOf()), origins[0], refConfig.worker);
				const b = normalize(await ask(origins[1], probe, tokensOf()), origins[1], otherConfig.worker);
				const found = differences(a, b);
				if (!found.length) continue;
				differ++;
				console.log(`\nDIFFERS  ${probe.name}  (${probe.method ?? "GET"} ${probe.path})\n  ${reference} ${found.join(`\n  ${reference} `)}`);
			}
			console.log(`\n${differ ? `${differ} of ${probes.length}` : `none of ${probes.length}`} requests answered differently by ${other} than by ${reference}, the reference`);
			if (differ) throw new Error(`${other} differs from ${reference}: ${differ} of ${probes.length} requests`);
		}));
	} finally {
		rmSync(state, { recursive: true, force: true });
	}
}
