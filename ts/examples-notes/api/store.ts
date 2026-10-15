import { env } from "cloudflare:workers";
import { DurablePublisher } from "@orpc/cloudflare";
import { follow, type FollowSource } from "charter-lib/follow";
import type { z } from "zod";
import type { note } from "./contract.ts";

// Where the notes are kept: D1 (DB) is the log, and the Hub Durable Object (HUB, worker.ts) only wakes
// the followers when one is added (docs/guides/streaming.md).

export type Note = z.infer<typeof note>;

const publisher = () => new DurablePublisher<{ note: Note }>(env.HUB);

/** A page of notes, newest first, before the cursor (a note id; none: from the newest), and one more
 * if there is a next page. */
export const before = async (cursor: number, limit: number) =>
	(await env.DB.prepare("SELECT id, body, created_at FROM notes WHERE id < ? ORDER BY id DESC LIMIT ?").bind(cursor, limit).all<Note>()).results;

/** A new note, in the log and announced on the hub. */
export async function insert(body: string): Promise<Note> {
	const note = await env.DB.prepare("INSERT INTO notes (body) VALUES (?) RETURNING id, body, created_at").bind(body).first<Note>();
	await publisher().publish("note", note!);
	return note!;
}

// The feed both transports serve: the log after a position, then each note the hub announces.
const notes: FollowSource<Note> = {
	subscribe: (listener, onError) => publisher().subscribe("note", listener, { onError }),
	since: async (after, limit) =>
		(await env.DB.prepare("SELECT id, body, created_at FROM notes WHERE id > ? ORDER BY id LIMIT ?").bind(after, limit).all<Note>()).results,
	latest: async () => (await env.DB.prepare("SELECT COALESCE(MAX(id), 0) AS id FROM notes").first<{ id: number }>())!.id,
};

/** The one feed: notes after `after` (a note id string), or from now; hub drops are logged, not seen. */
export const followNotes = (after: string | undefined, signal: AbortSignal | undefined) =>
	follow(notes, { after: after === undefined ? undefined : Number(after), signal, onBroken: error => console.warn("follow: hub subscription broken, resubscribing", String(error)) });
