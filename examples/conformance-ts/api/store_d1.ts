// The notes' log on Cloudflare (as examples/conformance-go's api/store_d1.go): the notes table in D1 (migrations/), and Hub (worker.ts), oRPC's
// Durable Object publisher, which fans every new note out to the WebSocket's follow() loops. The hub
// keeps no log: D1 is the log, so the hub may restart at any time (docs/self/streaming.md). The
// queries are Drizzle's, on the tables charter spec writes from migrations/ (api/tables.ts).
import { env } from "cloudflare:workers";
import { DurablePublisher } from "@orpc/cloudflare";
import { eq, gt, sql } from "drizzle-orm";
import { drizzle } from "drizzle-orm/d1";
import type { Entry, Notes } from "./store.ts";
import { notes as table } from "./tables.ts";

const entry = (row: typeof table.$inferSelect): Entry => ({ id: row.seq, note: { id: row.id, body: row.body } });

/** The log on the Worker's bindings: DB (D1) and HUB (Hub). */
export function d1Notes(): Notes {
	const db = () => drizzle(env.DB);
	const publisher = () => new DurablePublisher<{ note: Entry }>(env.HUB);
	return {
		page: async (offset, limit) => (await db().select().from(table).orderBy(table.seq).limit(limit).offset(offset)).map(row => entry(row).note),
		// No row comes back when the id is taken: then the stored note is read.
		put: async note => {
			const added = await db().insert(table).values(note).onConflictDoNothing({ target: table.id }).returning().get();
			if (added) return { entry: entry(added), created: true };
			const stored = await db().select().from(table).where(eq(table.id, note.id)).get();
			if (!stored) throw new Error(`note ${note.id} is neither new nor stored`);
			return { entry: entry(stored), created: false };
		},
		publish: async item => { await publisher().publish("note", item); },
		subscribe: (listener, onError) => publisher().subscribe("note", listener, { onError }),
		since: async (after, limit) => (await db().select().from(table).where(gt(table.seq, after)).orderBy(table.seq).limit(limit)).map(entry),
		latest: async () => (await db().select({ seq: sql<number>`COALESCE(MAX(${table.seq}), 0)` }).from(table).get())!.seq,
	};
}
