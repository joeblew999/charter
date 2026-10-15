// Fern's settings, written from the spec and charter.config.ts (sdk.ts): every scheme of the spec but
// OpenID Connect restated, any of them; the groups asked for, with the generators charter-ts pins.
//   node --test src/*.test.ts   (npm run check -w charter-ts runs them)
import assert from "node:assert/strict";
import { it } from "node:test";
import { fernSettings } from "./sdk.ts";

const spec = { components: { securitySchemes: {
	bearer: { type: "http", scheme: "bearer" },
	accessClientId: { type: "apiKey", in: "header", name: "CF-Access-Client-Id", "x-fern-header": { name: "accessClientId", env: "API_ACCESS_CLIENT_ID" } },
	oidc: { type: "openIdConnect", openIdConnectUrl: "/.well-known/openid-configuration" },
	OAuth: { type: "oauth2", flows: { clientCredentials: { tokenUrl: "/api/oauth/token", scopes: {} } } },
} } };

it("restates every scheme but OpenID Connect, OAuth first, and takes any of them", () => {
	const yml = fernSettings(spec, { name: "Api" }, "my-api/sdk/go", false);
	assert.match(yml, /^ {2}auth-schemes:\n {4}OAuth:\n/m);
	assert.match(yml, / {6}header: CF-Access-Client-Id\n {6}name: accessClientId\n {6}env: API_ACCESS_CLIENT_ID\n/);
	// The OAuth client's variables have the prefix of the access token's.
	assert.match(yml, / {6}client-id-env: API_CLIENT_ID\n {6}client-secret-env: API_CLIENT_SECRET\n/);
	assert.match(yml, / {8}endpoint: "POST \/api\/oauth\/token"\n/);
	assert.match(yml, /^ {2}auth:\n {4}any:\n {6}- OAuth\n {6}- bearer\n {6}- accessClientId\n/m);
	assert.doesNotMatch(yml, /oidc/);
});

it("has no auth for a spec without schemes, and the groups asked for", () => {
	const yml = fernSettings({}, { name: "Notes", more: ["python", "cli"], audiences: ["public"] }, "notes/sdk/go", true);
	assert.doesNotMatch(yml, /auth/);
	assert.match(yml, / {6}overlays: overlays.yml\n/);
	const groups = [...yml.split("\ngroups:\n")[1]!.matchAll(/^ {2}([a-z-]+):$/gm)].map(m => m[1]);
	assert.deepEqual(groups, ["go", "typescript", "typescript-dist", "python", "cli", "typescript-public"]);
	assert.match(yml, / {12}path: notes\/sdk\/go\n/);
	assert.match(yml, /packageName: notes\n/);
	assert.match(yml, /binaryName: notes\n/);
	assert.match(yml, / {2}typescript-public:\n {4}audiences:\n {6}- public\n/);
});

it("refuses a scheme the SDKs cannot send", () => {
	assert.throws(() => fernSettings({ components: { securitySchemes: { basic: { type: "http", scheme: "basic" } } } }, { name: "Api" }, "a/sdk/go", false), /bearer tokens, header keys and OAuth client credentials/);
});
