// The standard test: what every API charter serves must do (docs/reference/standard.md), checked on a
// running API from outside, as a client sees it. It knows no API: it reads the one it tests (its
// /api/openapi.json) and checks it against that. check runs it on the project's server (natively and
// under workerd), live-test on the deployed Worker, so no project writes or copies it.
//   - both specs are served, and name the API's own address;
//   - a refusal is problem details: an unknown path is 404, a method a path does not have is 405 with
//     the methods it has;
//   - its MCP endpoint lists tools that are the spec's operations, and answers MCP's protocol errors;
//   - an operation that needs credentials, called without them, is 401 with the challenge, which leads
//     to the API's metadata, which names the API and the scopes; through MCP too.
// Behind Cloudflare Access, every request carries the service token in <WORKER>_ACCESS_CLIENT_ID and
// _SECRET (WORKER: the first label of the host, in capitals), as the live tests send it.

type Operation = { operationId?: string; security?: Record<string, string[]>[]; requestBody?: { content?: Record<string, unknown> }; parameters?: { name: string; in: string }[] };
type Spec = { servers?: { url: string }[]; paths?: Record<string, Record<string, Operation>>; security?: Record<string, string[]>[] };

const methods = ["get", "put", "post", "delete", "patch"];
const mcpMeta = { "io.modelcontextprotocol/protocolVersion": "2026-07-28", "io.modelcontextprotocol/clientCapabilities": {}, "io.modelcontextprotocol/clientInfo": { name: "charter-standard", version: "1" } };

/** The Cloudflare Access service token for a host, from the environment; none when it is not set. */
function accessHeaders(origin: string): Record<string, string> {
	const prefix = new URL(origin).hostname.split(".")[0]!.replace(/[-. ]/g, "_").toUpperCase();
	const id = process.env[`${prefix}_ACCESS_CLIENT_ID`], secret = process.env[`${prefix}_ACCESS_CLIENT_SECRET`];
	return id && secret ? { "cf-access-client-id": id, "cf-access-client-secret": secret } : {};
}

/** Runs the standard test on the API at origin, says each check (by default on the console), and fails
 * if one did. */
export async function standard(origin: string, say: (line: string) => void = console.log): Promise<void> {
	const access = accessHeaders(origin);
	const failures: string[] = [];
	const check = (name: string, ok: boolean, detail: unknown) => {
		say(`${ok ? "PASS" : "FAIL"}  standard: ${name}${ok ? "" : `: ${typeof detail === "string" ? detail : JSON.stringify(detail)}`}`);
		if (!ok) failures.push(name);
	};
	const get = (path: string, init: RequestInit = {}) => fetch(origin + path, { ...init, headers: { ...access, ...(init.headers as Record<string, string>) }, redirect: "manual", signal: AbortSignal.timeout(15_000) });
	const json = async (response: Response) => (await response.json().catch(() => undefined)) as Record<string, unknown> | undefined;
	const problem = async (response: Response, status: number) => {
		const body = await json(response);
		return response.status === status && !!response.headers.get("content-type")?.startsWith("application/problem+json") && body?.status === status;
	};
	const mcp = (method: string, params: Record<string, unknown>, raw?: string) => get("/api/mcp", {
		method: "POST",
		headers: { "content-type": "application/json", accept: "application/json, text/event-stream", "mcp-protocol-version": "2026-07-28", "mcp-method": method, ...(params.name ? { "mcp-name": String(params.name) } : {}) },
		body: raw ?? JSON.stringify({ jsonrpc: "2.0", id: 1, method, params: { ...params, _meta: mcpMeta } }),
	});

	// Both specs, naming this address.
	const specAnswer = await get("/api/openapi.json");
	const spec = ((await json(specAnswer)) ?? {}) as Spec;
	check("GET /api/openapi.json names this address", specAnswer.status === 200 && spec.servers?.[0]?.url === origin, { status: specAnswer.status, servers: spec.servers });
	const asyncAnswer = await get("/api/asyncapi.json");
	const asyncSpec = (await json(asyncAnswer)) as { channels?: object; servers?: { production?: { host?: string } } } | undefined;
	const channels = Object.keys(asyncSpec?.channels ?? {}).length;
	check("GET /api/asyncapi.json, naming this host if it has channels", asyncAnswer.status === 200 && (!channels || asyncSpec?.servers?.production?.host === new URL(origin).host), { status: asyncAnswer.status, servers: asyncSpec?.servers });

	// Refusals are problem details.
	check("an unknown path is 404, as problem details", await problem(await get("/api/charter-standard-unknown"), 404), "not a 404 problem");
	const operations = Object.entries(spec.paths ?? {}).flatMap(([path, item]) => methods.filter(m => item[m]).map(method => ({ path, method, op: item[method]! })));
	const withoutParams = Object.entries(spec.paths ?? {}).find(([path]) => !path.includes("{"));
	if (withoutParams) {
		const [path, item] = withoutParams;
		const has = methods.filter(m => item[m]).map(m => m.toUpperCase()).sort();
		const other = ["DELETE", "PUT", "PATCH", "POST", "GET"].find(m => !has.includes(m))!;
		const answer = await get(path, { method: other });
		check(`${other} ${path} is 405, with the methods it has`, (await problem(answer, 405)) && answer.headers.get("allow") === has.join(", "), { status: answer.status, allow: answer.headers.get("allow") });
	}

	// MCP: the tools are the spec's operations, and MCP's own errors.
	const listed = await mcp("tools/list", {});
	const tools = (((await json(listed))?.result as { tools?: { name: string }[] } | undefined)?.tools ?? []).map(tool => tool.name);
	const operationIds = new Set(operations.map(o => o.op.operationId));
	check("POST /api/mcp lists tools, each an operation of the spec", listed.status === 200 && tools.every(name => operationIds.has(name)), { status: listed.status, tools });
	const getMcp = await get("/api/mcp");
	check("GET /api/mcp is 405, with Allow: POST", getMcp.status === 405 && getMcp.headers.get("allow") === "POST", { status: getMcp.status, allow: getMcp.headers.get("allow") });
	const parse = (await json(await mcp("tools/list", {}, '{"jsonrpc":"2.0","id":1,"method":'))) as { error?: { code?: number } } | undefined;
	check("MCP: JSON that does not parse is error -32700", parse?.error?.code === -32700, parse);
	const unknown = (await json(await mcp("charter/standard", {}))) as { error?: { code?: number } } | undefined;
	check("MCP: an unknown method is error -32601", unknown?.error?.code === -32601, unknown);

	// Signing in: an operation that needs credentials, called without them.
	const secured = operations.find(o => (o.op.security ?? spec.security ?? []).some(requirement => Object.keys(requirement).length > 0));
	if (!secured) {
		check("an API that takes no credentials says nothing at /.well-known/", (await get("/.well-known/oauth-protected-resource")).status === 404, "not 404");
	} else {
		const path = secured.path.replace(/\{[^}]+\}/g, "x");
		const body = secured.op.requestBody?.content?.["application/json"] ? { body: "{}", headers: { "content-type": "application/json" } } : {};
		const refused = await get(path, { method: secured.method.toUpperCase(), ...body });
		const challenge = refused.headers.get("www-authenticate") ?? "";
		const metadataURL = `${origin}/.well-known/oauth-protected-resource`;
		check(`${secured.method.toUpperCase()} ${secured.path} without credentials is 401, whose challenge points to the API's metadata`, (await problem(refused, 401)) && challenge.includes(`resource_metadata="${metadataURL}"`), { status: refused.status, challenge });
		const metadata = (await json(await get("/.well-known/oauth-protected-resource"))) as { resource?: string; scopes_supported?: string[]; authorization_servers?: string[] } | undefined;
		const scopes = [...new Set((secured.op.security ?? spec.security ?? []).flatMap(requirement => Object.values(requirement).flat()))];
		check("the API's metadata names it and its scopes", metadata?.resource === origin && scopes.every(scope => metadata.scopes_supported?.includes(scope)), metadata);
		if (metadata?.authorization_servers?.includes(origin)) {
			const server = (await json(await get("/.well-known/oauth-authorization-server"))) as { token_endpoint?: string } | undefined;
			const tokenPath = server?.token_endpoint?.startsWith(origin) ? server.token_endpoint.slice(origin.length) : undefined;
			check("the API, its own authorization server, names its token endpoint, a route of the spec", !!tokenPath && !!spec.paths?.[tokenPath]?.post, server);
		}
		const tool = operations.find(o => o.op.operationId && tools.includes(o.op.operationId) && (o.op.security ?? spec.security ?? []).some(r => Object.keys(r).length > 0));
		if (tool) {
			const call = await mcp("tools/call", { name: tool.op.operationId, arguments: {} });
			check(`MCP: ${tool.op.operationId} without credentials is 401, with the challenge`, call.status === 401 && (call.headers.get("www-authenticate") ?? "").includes(`resource_metadata="${metadataURL}"`), { status: call.status, challenge: call.headers.get("www-authenticate") });
		}
	}
	if (failures.length) throw new Error(`the standard test failed: ${failures.join("; ")}`);
}
