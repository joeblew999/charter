import { DurablePublisherObject } from "@orpc/cloudflare";

// The live fan-out for new notes: oRPC's Durable Object publisher. Subscribers (the Worker's follow()
// loops) are hibernatable WebSockets, so the hub sleeps between notes. It keeps no resume log: D1 is
// the log and follow() catches up from it, so the hub may restart at any time (docs/plans/realtime.md).
export class NotesHub extends DurablePublisherObject {}
