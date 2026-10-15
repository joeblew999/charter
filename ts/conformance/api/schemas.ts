import { z } from "zod";

// What the API answers, as Zod schemas: the contract's outputs (api/contract.ts), and what the store and
// the tests check answers against. Zod alone, so a Go twin's tests carry them as they are.

/**
 * What a planned end of watchNotes returns: SSE `event: close` with `data: "[end-of-stream]"`, Fern's
 * terminator. Fern matches it as a substring of each event's data, so no note may contain it (create
 * refuses one), and it is plain text because Fern's Rust generator pastes it into source unescaped.
 */
// Upstream: fern-api/fern#17936 (when fixed: terminators match whole data values, so the body rule can go)
// Upstream: fern-api/fern#17939 (when fixed: the Rust generator escapes it, so any text would do)
export const END = "[end-of-stream]";

// `id` names the schema in components.schemas, and so the type in the SDKs (Note, Chunk).
export const note = z.object({ id: z.string().meta({ examples: ["note-1"] }), body: z.string().meta({ examples: ["Buy milk"] }) }).meta({ id: "Note" });
export const chunk = z.object({ text: z.string().meta({ examples: ["echo"] }), done: z.boolean().optional() }).meta({ id: "Chunk" });
