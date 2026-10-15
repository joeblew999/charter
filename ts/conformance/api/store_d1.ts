// The notes' log on Cloudflare (as go/conformance's api/store_d1.go): the notes table in D1 (migrations/), and Hub (worker.ts), oRPC's
// Durable Object publisher, which fans every new note out to the WebSocket's follow() loops. The hub
// keeps no log: D1 is the log, so the hub may restart at any time (docs/self/streaming.md).
import { env } from "cloudflare:workers";
import { DurablePublisher } from "@orpc/cloudflare";
import type { Entry, Notes } from "./store.ts";

type Row = { seq: number; id: string; body: string };
const entry = (row: Row): Entry => ({ id: row.seq, note: { id: row.id, body: row.body } });

/** The log on the Worker's bindings: DB (D1) and HUB (Hub). */
export function d1Notes(): Notes {
	const publisher = () => new DurablePublisher<{ note: Entry }>(env.HUB);
	return {
		page: async (offset, limit) =>
			(await env.DB.prepare("SELECT seq, id, body FROM notes ORDER BY seq LIMIT ? OFFSET ?").bind(limit, offset).all<Row>()).results.map(row => entry(row).note),
		// RETURNING gives no row when the id is taken: then the stored note is read.
		put: async note => {
			const added = await env.DB.prepare("INSERT INTO notes (id, body) VALUES (?, ?) ON CONFLICT (id) DO NOTHING RETURNING seq, id, body").bind(note.id, note.body).first<Row>();
			if (added) return { entry: entry(added), created: true };
			const stored = await env.DB.prepare("SELECT seq, id, body FROM notes WHERE id = ?").bind(note.id).first<Row>();
			if (!stored) throw new Error(`note ${note.id} is neither new nor stored`);
			return { entry: entry(stored), created: false };
		},
		publish: async item => { await publisher().publish("note", item); },
		subscribe: (listener, onError) => publisher().subscribe("note", listener, { onError }),
		since: async (after, limit) =>
			(await env.DB.prepare("SELECT seq, id, body FROM notes WHERE seq > ? ORDER BY seq LIMIT ?").bind(after, limit).all<Row>()).results.map(entry),
		latest: async () => (await env.DB.prepare("SELECT COALESCE(MAX(seq), 0) AS seq FROM notes").first<{ seq: number }>())!.seq,
	};
}
