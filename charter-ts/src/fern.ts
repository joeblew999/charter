// What a project's fern/generators.yml must say that its spec already says: Fern sends only the
// credentials generators.yml restates (api.auth-schemes, and auth: any of them), so each security
// scheme of the spec is restated there, an API key with the spec's header, option name and variable.
// An OpenID Connect scheme is not: its token goes as the bearer token.
import { existsSync, readFileSync } from "node:fs";

type Scheme = { type?: string; name?: string; "x-fern-header"?: { name?: string; env?: string } };

/** What fern/generators.yml lacks of the spec's schemes, one line each; none: nothing. */
export function unstatedSchemes(dir = "."): string[] {
	if (!existsSync(`${dir}/fern/openapi.json`) || !existsSync(`${dir}/fern/generators.yml`)) return [];
	const spec = JSON.parse(readFileSync(`${dir}/fern/openapi.json`, "utf8")) as { components?: { securitySchemes?: Record<string, Scheme> } };
	const yml = readFileSync(`${dir}/fern/generators.yml`, "utf8");
	const schemes = Object.entries(spec.components?.securitySchemes ?? {}).filter(([, scheme]) => scheme.type !== "openIdConnect");
	if (!schemes.length) return [];
	const missing: string[] = [];
	if (!/^ {2}auth:\n {4}any:\n/m.test(yml)) missing.push("api.auth is not `any:` of the schemes");
	for (const [name, scheme] of schemes) {
		const want = [`    ${name}:\n`, `      - ${name}\n`];
		if (scheme.type === "apiKey") want.push(`header: ${scheme.name}\n`, `name: ${scheme["x-fern-header"]?.name}\n`, `env: ${scheme["x-fern-header"]?.env}\n`);
		for (const line of want) if (!yml.includes(line)) missing.push(`the scheme ${name} has no ${JSON.stringify(line.trim())}`);
	}
	return missing;
}
