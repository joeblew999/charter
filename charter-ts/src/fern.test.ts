// What a project's fern/generators.yml must restate of its spec (fern.ts): each security scheme, an
// API key with its header, option name and variable, and auth: any of them.
//   node --test src/*.test.ts   (npm run check -w charter-ts runs them)
import assert from "node:assert/strict";
import { mkdirSync, mkdtempSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { after, it } from "node:test";
import { unstatedSchemes } from "./fern.ts";

const scratch = mkdtempSync(join(tmpdir(), "charter-ts-fern-"));
after(() => rmSync(scratch, { recursive: true, force: true }));
const project = (name: string, yml: string) => {
	const dir = join(scratch, name);
	mkdirSync(join(dir, "fern"), { recursive: true });
	writeFileSync(join(dir, "fern", "openapi.json"), JSON.stringify({ components: { securitySchemes: {
		bearer: { type: "http", scheme: "bearer" },
		accessClientId: { type: "apiKey", in: "header", name: "CF-Access-Client-Id", "x-fern-header": { name: "accessClientId", env: "API_ACCESS_CLIENT_ID" } },
		oidc: { type: "openIdConnect", openIdConnectUrl: "/.well-known/openid-configuration" },
	} } }));
	writeFileSync(join(dir, "fern", "generators.yml"), yml);
	return dir;
};
const restated = `api:
  auth-schemes:
    bearer:
      scheme: bearer
    accessClientId:
      header: CF-Access-Client-Id
      name: accessClientId
      env: API_ACCESS_CLIENT_ID
  auth:
    any:
      - bearer
      - accessClientId
`;

it("is nothing when every scheme but OpenID Connect is restated, and auth is any of them", () => {
	assert.deepEqual(unstatedSchemes(project("whole", restated)), []);
});

it("names what is missing: a scheme, an API key's variable, auth: any", () => {
	const missing = unstatedSchemes(project("partial", restated.replace("      env: API_ACCESS_CLIENT_ID\n", "").replace("      - bearer\n", "")));
	assert.deepEqual(missing, ['the scheme bearer has no "- bearer"', 'the scheme accessClientId has no "env: API_ACCESS_CLIENT_ID"']);
	assert.ok(unstatedSchemes(project("no-any", restated.replace("    any:\n", "    all:\n"))).includes("api.auth is not `any:` of the schemes"));
});
