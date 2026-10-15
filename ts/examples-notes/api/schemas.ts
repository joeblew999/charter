import { z } from "zod";

// What the API answers, as Zod schemas: the contract's outputs (api/contract.ts), and what the tests
// check every answer against (test/). Zod alone, so a Go twin's tests carry them as they are.

export const note = z.object({
	id: z.number().int().meta({ format: "int64", examples: [42] }),
	body: z.string().meta({ examples: ["Buy milk"] }),
	created_at: z.string().meta({ examples: ["2026-10-01 12:00:00"] }),
}).meta({ id: "Note" });

/**
 * What a planned stream end returns: SSE `event: close` with `data: "[end-of-stream]"`, Fern's terminator
 * for watch. Fern matches the terminator as a substring of each event's data, so no note may contain
 * it (note bodies reject it), and it is plain text because Fern's Rust generator pastes it into
 * source unescaped.
 */
// Upstream: fern-api/fern#17936 (when fixed: terminators match whole data values, so the body rule can go)
// Upstream: fern-api/fern#17939 (when fixed: the Rust generator escapes it, so any text would do)
export const END = "[end-of-stream]";
