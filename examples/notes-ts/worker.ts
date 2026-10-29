// The Worker Cloudflare runs: the API (api/handlers.ts), and the hub its notes are announced on.
import { DurablePublisherObject } from "@orpc/cloudflare";
import { handler } from "./api/handlers.ts";

// The live fan-out for new notes: oRPC's Durable Object publisher. Subscribers (the Worker's follow()
// loops) are hibernatable WebSockets, so the hub sleeps between notes. It keeps no resume log: D1 is
// the log and follow() catches up from it, so the hub may restart at any time (docs/self/streaming.md).
export class Hub extends DurablePublisherObject {}

export default {
	fetch: request => handler(request, {}),
} satisfies ExportedHandler;
