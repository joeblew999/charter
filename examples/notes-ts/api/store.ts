import { env } from "cloudflare:workers";
import { DurablePublisher } from "@orpc/cloudflare";
import { follow, type FollowSource } from "charter-lib/follow";
import { desc, gt, lt, sql } from "drizzle-orm";
import { drizzle } from "drizzle-orm/d1";
import type { z } from "zod";
import type { note } from "./schemas.ts";
import { notes as table } from "./tables.ts";

// Where the notes are kept: D1 (DB) is the log, and the Hub Durable Object (HUB, worker.ts) only wakes
// the followers when one is added (docs/self/streaming.md). The queries are Drizzle's, on the tables
// npm run spec writes from migrations/ (api/tables.ts).

export type Note = z.infer<typeof note>;

const db = () => drizzle(env.DB);
const publisher = () => new DurablePublisher<{ note: Note }>(env.HUB);

/** A page of notes, newest first, before the cursor (a note id; none: from the newest), and one more
 * if there is a next page. */
export const before = (cursor: number, limit: number): Promise<Note[]> =>
	db().select().from(table).where(lt(table.id, cursor)).orderBy(desc(table.id)).limit(limit);

/** A new note, in the log and announced on the hub. */
export async function insert(body: string): Promise<Note> {
	const note = await db().insert(table).values({ body }).returning().get();
	await publisher().publish("note", note);
	return note;
}

// The feed both transports serve: the log after a position, then each note the hub announces.
const notes: FollowSource<Note> = {
	subscribe: (listener, onError) => publisher().subscribe("note", listener, { onError }),
	since: (after, limit) => db().select().from(table).where(gt(table.id, after)).orderBy(table.id).limit(limit),
	latest: async () => (await db().select({ id: sql<number>`COALESCE(MAX(${table.id}), 0)` }).from(table).get())!.id,
};

/** The one feed: notes after `after` (a note id string), or from now; hub drops are logged, not seen. */
export const followNotes = (after: string | undefined, signal: AbortSignal | undefined) =>
	follow(notes, { after: after === undefined ? undefined : Number(after), signal, onBroken: error => console.warn("follow: hub subscription broken, resubscribing", String(error)) });
