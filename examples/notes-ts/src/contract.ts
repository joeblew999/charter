import { asyncIteratorObject, oc } from "@orpc/contract";
import { openapi, type OpenAPIV3_2 } from "@orpc/openapi";
import { asyncapi } from "./asyncapi.ts";
import { z } from "zod";

// The API, contract first: every route with its method, path, input and output as Zod 4 schemas.
// The Worker implements it (src/index.ts) and the OpenAPI spec is generated from it (spec.ts ->
// fern/openapi.json), from which Fern makes SDKs, a CLI and docs. Everything the SDKs
// need is said here too: operationId/tags name the SDK methods, and `spec` adds Fern's extensions
// (x-fern-*) to the generated operation, so nothing is patched afterwards.

export const info = { title: "charter-notes-ts", version: "1.0.0", description: "Notes API: oRPC contract -> OpenAPI -> Fern." };
export const asyncInfo = { title: "charter-notes-ts live", version: "1.0.0" };

export const note = z.object({
	id: z.number().int(),
	body: z.string(),
	created_at: z.string(),
});

/** The resume position in every stream: a note id, as an opaque string like list's cursor (docs/realtime.md, rule 1). */
export const after = z.string().regex(/^\d+$/).optional()
	.describe("Resume after this note id (the id of the last note you received). Absent: only notes created from now on");

/**
 * What a planned stream end returns: SSE `event: close` with `data: "[end-of-stream]"`, Fern's terminator
 * for watch. Fern matches the terminator as a substring of each event's data, so no note may contain
 * it (note bodies reject it), and it is plain text because Fern's Rust generator pastes it into
 * source unescaped.
 */
// Upstream: fern-api/fern#17936 (when fixed: terminators match whole data values, so the body rule can go)
// Upstream: fern-api/fern#17939 (when fixed: the Rust generator escapes it, so any text would do)
export const END = "[end-of-stream]";

/** Fern's names for the SDK method: client.<group>.<method>() and `cli <group> <method>`. */
const sdk = (group: string, method: string, extra: Record<string, unknown> = {}) =>
	(op: OpenAPIV3_2.OperationObject): OpenAPIV3_2.OperationObject => ({ ...op, "x-fern-sdk-group-name": group, "x-fern-sdk-method-name": method, ...extra });

export const contract = {
	hello: oc
		.meta(openapi({ method: "GET", path: "/api/hello", summary: "Say hello", tags: ["meta"], operationId: "hello", spec: sdk("meta", "hello") }))
		.output(z.object({ message: z.string() })),
	notes: {
		list: oc
			.meta(openapi({
				method: "GET", path: "/api/notes", summary: "List notes, newest first (cursor pagination)", tags: ["notes"], operationId: "listNotes",
				spec: sdk("notes", "list", { "x-fern-pagination": { cursor: "$request.cursor", next_cursor: "$response.next_cursor", results: "$response.data" } }),
			}))
			.input(z.object({
				// Opaque string cursors: the generated CLI's --page-all stops on numeric ones.
				cursor: z.string().optional().describe("Opaque cursor from the previous page's next_cursor"),
				limit: z.coerce.number().int().min(1).max(100).default(20),
			}))
			.output(z.object({ data: z.array(note), next_cursor: z.string().optional().describe("Pass as cursor for the next page; absent on the last page") })),
		watch: oc
			.meta(openapi({
				method: "GET", path: "/api/notes/watch", summary: "Stream notes as they are created (Server-Sent Events). The stream ends after `seconds`; call again with `after` = the last note id to continue without gaps", description: "Each event's SSE id is the note id, so a browser EventSource resumes by itself (Last-Event-ID).", tags: ["notes"], operationId: "watchNotes",
				// Upstream: fern-api/fern#17938 (when fixed: Fern could read oRPC's envelope, event: message|close|error, as is)
				// oRPC describes the stream as its SSE envelope (event: message|close|error); tell Fern it's
				// an SSE stream whose `data:` payloads are notes.
				spec: op => {
					const ok = op.responses?.["200"] as OpenAPIV3_2.ResponseObject | undefined;
					const stream = ok?.content?.["text/event-stream"] as OpenAPIV3_2.MediaTypeObject | undefined;
					if (stream) stream.schema = z.toJSONSchema(note) as OpenAPIV3_2.SchemaObject;
					// resumable: the SDKs reconnect by themselves on a drop, sending Last-Event-ID (= the note id).
					// The terminator marks a planned end, so they only reconnect on genuine drops.
					return sdk("notes", "watch", { "x-fern-streaming": { format: "sse", terminator: END, resumable: true } })(op);
				},
			}))
			.input(z.object({
				after,
				seconds: z.coerce.number().int().min(1).max(300).default(30).describe("How long to keep the stream open"),
			}))
			.output(asyncIteratorObject(note, z.literal(END).optional())),
		// The WebSocket channel: in the AsyncAPI spec (src/asyncapi.ts), not in OpenAPI. Plain JSON notes;
		// `after` (a query parameter) resumes, the same position as watch.
		live: oc
			.meta(openapi({ method: "GET", path: "/api/notes/live" }))
			.meta(asyncapi({
				channel: "liveNotes", address: "/api/notes/live", operationId: "receiveNote", message: "Note",
				summary: "New notes over a WebSocket, as plain JSON. On close, reconnect with `after` = the last note id to continue without gaps",
			}))
			.input(z.object({ after }))
			.output(asyncIteratorObject(note)),
		create: oc
			.meta(openapi({ method: "POST", path: "/api/notes", summary: "Create a note", tags: ["notes"], operationId: "createNote", spec: sdk("notes", "create") }))
			.input(z.object({ body: z.string().min(1).refine(body => !body.includes(END), `must not contain ${END}`) }))
			.output(note),
	},
};
