import { asyncIteratorObject, oc } from "@orpc/contract";
import { openapi, type OpenAPIV3_2 } from "@orpc/openapi";
import { z } from "zod";
import { asyncapi } from "../../../api/ts/src/asyncapi.ts";

// The showcase API, contract first: one API with every Fern feature we use (docs/sdk.md). The harness
// Worker implements it (src/showcase.ts, under /api/mock) and both specs are generated from it
// (spec.ts -> sdk/fern/apis/showcase/{openapi,asyncapi}.json). Each feature is switched on here:
// - on an operation, by `openapi({ operationId, tags, spec })`: the SDK names and Fern's x-fern-*;
// - for the whole document, by `document` below: oRPC's contract has no place for those;
// - the WebSocket by `asyncapi({...})` (api/ts/src/asyncapi.ts), and the webhooks by `webhooks` below.
// A Go server of the same API (api/go/showcase, sdk/fern/apis/showcase-go) compares its spec with
// these names: operation ids, paths, parameters, x-fern-* values, the channel and its messages.

export const info = { title: "Showcase", version: "1.0.0", description: "Every Fern feature we care about, via standard OpenAPI + x-fern-* extensions." };
export const asyncInfo = { title: "Showcase live", version: "1.0.0" };

// `id` names the schema in components.schemas, and so the type in the SDKs (Note, Chunk).
export const note = z.object({ id: z.string(), body: z.string() }).meta({ id: "Note" });
export const chunk = z.object({ text: z.string(), done: z.boolean().optional() }).meta({ id: "Chunk" });

type Operation = OpenAPIV3_2.OperationObject;
/** Fern's audiences: group `typescript-public` in generators.yml keeps only the "public" operations. */
const everyone = { "x-fern-audiences": ["public", "internal"] };
const internal = { "x-fern-audiences": ["internal"] };
const fern = (extensions: Record<string, unknown>) => (op: Operation): Operation => ({ ...op, ...extensions });

/**
 * What the document says and no operation does: OAuth client credentials for every operation, the
 * idempotency header (an SDK request option, not a parameter), and how webhooks are signed.
 * `auth-schemes` in generators.yml tells Fern which operation is the token endpoint.
 */
export const document = {
	security: [{ OAuth: [] }],
	components: { securitySchemes: { OAuth: { type: "oauth2" as const, flows: { clientCredentials: { tokenUrl: "/oauth/token", scopes: {} } } } } },
	"x-fern-idempotency-headers": [{ header: "Idempotency-Key", name: "idempotency_key" }],
	"x-fern-webhook-signature": { type: "hmac", header: "x-webhook-signature", algorithm: "sha256", encoding: "hex" },
};

export const contract = {
	auth: {
		getToken: oc
			.meta(openapi({
				method: "POST", path: "/oauth/token", summary: "OAuth client-credentials token (used by the SDK itself)", tags: ["auth"], operationId: "getToken", successDescription: "token",
				// The token endpoint takes no token, and OAuth sends it a form: oRPC's handler reads one, but
				// its generator always writes the body as application/json.
				spec: op => {
					const body = op.requestBody as OpenAPIV3_2.RequestBodyObject;
					return fern({ ...everyone, security: [], requestBody: { ...body, content: { "application/x-www-form-urlencoded": body.content!["application/json"]! } } })(op);
				},
			}))
			.input(z.object({ client_id: z.string(), client_secret: z.string() }))
			.output(z.object({ access_token: z.string(), expires_in: z.number().int() })),
	},
	notes: {
		list: oc
			.meta(openapi({
				method: "GET", path: "/notes", summary: "List notes (cursor pagination)", tags: ["notes"], operationId: "listNotes", successDescription: "a page",
				spec: fern({ ...everyone, "x-fern-pagination": { cursor: "$request.cursor", next_cursor: "$response.next_cursor", results: "$response.data" } }),
			}))
			.input(z.object({ cursor: z.string().optional(), limit: z.coerce.number().int().optional() }))
			.output(z.object({ data: z.array(note), next_cursor: z.string().optional() })),
		create: oc
			.meta(openapi({
				method: "POST", path: "/notes", summary: "Create a note (idempotent: safe to retry with an Idempotency-Key)", tags: ["notes"], operationId: "createNote", successDescription: "created",
				spec: fern({ ...everyone, "x-fern-idempotent": true }),
			}))
			.input(z.object({ body: z.string() }))
			.output(note),
		// The WebSocket channel: in the AsyncAPI spec, not in OpenAPI. Both ways are streams of plain JSON
		// messages: the client sends Subscribe, the server sends NoteEvent.
		live: oc
			.meta(openapi({ method: "GET", path: "/notes/live" }))
			.meta(asyncapi({ channel: "liveNotes", address: "/notes/live", operationId: "receiveNoteEvent", message: "NoteEvent", send: { message: "Subscribe", operationId: "sendSubscribe" } }))
			.input(asyncIteratorObject(z.object({ topic: z.string() })))
			.output(asyncIteratorObject(z.object({
				event: z.string(),
				id: z.string(),
				body: z.string().optional(),
				auth: z.string().optional().describe("The Authorization value the server received on connecting: the harness test reads it"),
			}))),
	},
	files: {
		// A file in the input makes oRPC write (and read) the body as multipart/form-data.
		upload: oc
			.meta(openapi({ method: "POST", path: "/files", summary: "Upload a file (multipart)", tags: ["files"], operationId: "uploadFile", successDescription: "stored", spec: fern(internal) }))
			.input(z.object({ file: z.file(), note: z.string().optional() }))
			.output(z.object({ id: z.string(), size: z.number().int() })),
	},
	chat: oc
		.meta(openapi({
			method: "POST", path: "/chat", summary: "Stream a reply (Server-Sent Events)", tags: ["chat"], operationId: "chat", successDescription: "chunks",
			// Upstream: fern-api/fern#17938 (when fixed: Fern could read oRPC's envelope, event: message|close|error, as is)
			// oRPC describes the stream as its SSE envelope; tell Fern its `data:` payloads are chunks.
			spec: op => {
				const stream = (op.responses?.["200"] as OpenAPIV3_2.ResponseObject | undefined)?.content?.["text/event-stream"] as OpenAPIV3_2.MediaTypeObject | undefined;
				if (stream) stream.schema = { $ref: "#/components/schemas/Chunk" };
				return fern({ ...everyone, "x-fern-streaming": { format: "sse" } })(op);
			},
		}))
		.input(z.object({ prompt: z.string() }))
		.output(asyncIteratorObject(chunk)),
};

/** The calls the API makes to its users (OpenAPI 3.1 `webhooks`): each input is the payload sent. */
export const webhooks = {
	noteCreated: oc
		.meta(openapi({ method: "POST", summary: "Sent to you when a note is created", tags: ["notes"], operationId: "noteCreatedWebhook" }))
		.input(z.object({ event: z.string(), note })),
};
