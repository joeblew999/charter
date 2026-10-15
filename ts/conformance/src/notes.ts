// The notes' log, which the conformance API serves: D1 with the hub Durable Object in the Worker
// (src/hub.ts), memory in the unit test (memoryNotes). It starts with the five notes
// migrations/0001_init.sql seeds.
import type { FollowSource } from "charter-lib/follow";

export interface Note {
	id: string;
	body: string;
}

/** A note in the log: `id` is its place there, follow()'s position (docs/guides/streaming.md, rule 1). */
export interface Entry {
	id: number;
	note: Note;
}

export interface Notes extends FollowSource<Entry> {
	/** The notes in the log's order from offset, at most limit. */
	page(offset: number, limit: number): Promise<Note[]>;
	/** Stores the note under its id if there is none: the one stored under the id, and whether it is new. */
	put(note: Note): Promise<{ entry: Entry; created: boolean }>;
	/** Announces a new note on the hub: only a wake-up for followers, who read the log. */
	publish(entry: Entry): Promise<void>;
}

/** The notes every log starts with: migrations/0001_init.sql writes the same into D1. */
export const seeds: Note[] = ["n1", "n2", "n3", "n4", "n5"].map(id => ({ id, body: `note ${id}` }));

/** A log and a hub in memory, with the seeded notes. */
export function memoryNotes(): Notes {
	const entries: Entry[] = seeds.map((note, i) => ({ id: i + 1, note }));
	const listeners = new Set<(entry: Entry) => void>();
	return {
		page: async (offset, limit) => entries.slice(offset, offset + limit).map(entry => entry.note),
		put: async note => {
			const stored = entries.find(entry => entry.note.id === note.id);
			if (stored) return { entry: stored, created: false };
			const entry = { id: entries.length + 1, note };
			entries.push(entry);
			return { entry, created: true };
		},
		publish: async entry => { for (const listener of listeners) listener(entry); },
		subscribe: async listener => {
			listeners.add(listener);
			return async () => { listeners.delete(listener); };
		},
		since: async (after, limit) => entries.filter(entry => entry.id > after).slice(0, limit),
		latest: async () => entries.length,
	};
}
