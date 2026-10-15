// The Worker Cloudflare runs: the conformance API, served the standard way under /api (api/handlers.ts,
// on D1 and the hub: api/store_d1.ts), and the harness (harness.ts) at /harness/.
import { env } from "cloudflare:workers";
import { DurablePublisherObject } from "@orpc/cloudflare";
import { type Deployment, handler, oauthOf, type Settings, trusted, webhookOf } from "./api/handlers.ts";
import { d1Notes } from "./api/store_d1.ts";
import { sdkTest, wsTest } from "./harness.ts";

/** The live fan-out for new notes: oRPC's Durable Object publisher (HUB). It keeps no log: D1 is the
 * log, so the hub may restart at any time (docs/self/streaming.md). */
export class Hub extends DurablePublisherObject {}

// The notes in D1 behind the hub; whom the API trusts, from the Worker's secrets; its Rate Limiting
// bindings; where its webhook goes. The four settings for Access and OIDC are optional secrets
// (cloudflare.config.ts), so the generated Env may not name them.
const settings = env as typeof env & Settings;
const deployment: Deployment = { notes: d1Notes(), trusted: trusted(settings), oauth: oauthOf(settings), limits: env, webhook: webhookOf(settings) };

export default {
	async fetch(request) {
		const url = new URL(request.url);
		if (url.pathname === "/harness/sdk-test") return sdkTest(deployment);
		// ?target= another Worker serving the API: on Cloudflare a Worker can't call its own URL (error 1042).
		if (url.pathname === "/harness/ws-test") return wsTest(url.searchParams.get("target") ?? url.origin, deployment);
		return handler(request, deployment);
	},
} satisfies ExportedHandler;
