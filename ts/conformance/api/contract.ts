import { asyncIteratorObject, oc } from "@orpc/contract";
import { openapi, type OpenAPIV3_2 } from "@orpc/openapi";
import { z } from "zod";
import { asyncapi } from "charter-lib/asyncapi";
import { accessScheme, oidcScheme, scheme, type Security } from "charter-lib/auth";
import { type Limit, limited } from "charter-lib/ratelimit";
import { notATool } from "charter-lib/mcp";
import { chunk, END, note } from "./schemas.ts";

// The conformance API, contract first: one API with every Fern feature we use (docs/self/sdks.md). The
// Worker implements it (api/handlers.ts, served the standard way under /api) and both specs are
// generated from it (npm run spec -> fern/{openapi,asyncapi}.json). Each feature is switched on here:
// - on an operation, by `openapi({ operationId, tags, spec })`: the SDK names and Fern's x-fern-*;
// - for the whole document, by `base` below: oRPC's contract has no place for those;
// - the WebSocket by `asyncapi({...})` (charter-lib/asyncapi), and the webhooks by `webhooks` below.
// A Go server of the same API (go/conformance/api, go/conformance/fern) compares its spec with
// these names: operation ids, paths, parameters, x-fern-* values, the channel and its messages.

export const info = { title: "Conformance", version: "1.0.0", description: "Every Fern feature we care about, via standard OpenAPI + x-fern-* extensions." };

/**
 * How often one caller may create notes: the Worker's WRITE_LIMIT binding, which cloudflare.config.ts
 * declares from the spec. Over it: 429, with Retry-After.
 */
export const writeLimit: Limit = { binding: "WRITE_LIMIT", limit: 100, period: 60 };

/** An integer as the Go contract has it (int32): the specs say so, and the SDKs type it alike. */
const int32 = (examples: number[]) => ({ format: "int32", examples });

type Operation = OpenAPIV3_2.OperationObject;
/** Fern's audiences: group `typescript-public` in generators.yml keeps only the "public" operations. */
const everyone = { "x-fern-audiences": ["public", "internal"] };
const internal = { "x-fern-audiences": ["internal"] };
const fern = (extensions: Record<string, unknown>) => (op: Operation): Operation => ({ ...op, ...extensions });

/**
 * Four ways in, any of them: OAuth client credentials (the SDKs fetch the token themselves), and
 * charter's three (charter-lib/auth): the API's own tokens, Cloudflare Access and an OpenID Connect
 * issuer. Reading needs read and writing write, by any of them: an OAuth token has both.
 * `auth-schemes` in generators.yml tells Fern which operation is the token endpoint.
 */
const ways = oidcScheme(accessScheme(info.title, scheme("read")));
const oauth = { type: "oauth2" as const, flows: { clientCredentials: { tokenUrl: "/api/oauth/token", scopes: { read: "Read notes", write: "Create notes and upload files" } } } };

/** An operation's security: these scopes, by any of the four ways in (charter-lib/specs says each of charter's). */
const needs = (scopes: string[]): Security => [{ bearer: scopes }, { OAuth: scopes }];

/**
 * What the document says and no operation does: who may call, the idempotency header (an SDK request
 * option, not a parameter), and how webhooks are signed.
 */
export const base = {
	security: needs(["read"]),
	components: { securitySchemes: { ...ways.components!.securitySchemes, OAuth: oauth } },
	"x-fern-idempotency-headers": [{ header: "Idempotency-Key", name: "idempotency_key" }],
	"x-fern-webhook-signature": { type: "hmac", header: "x-webhook-signature", algorithm: "sha256", encoding: "hex" },
};

export const contract = {
	auth: {
		getToken: oc
			.meta(openapi({
				method: "POST", path: "/api/oauth/token", summary: "OAuth client-credentials token (used by the SDK itself)", tags: ["auth"], operationId: "getToken", successDescription: "token",
				// The token endpoint takes no token, and OAuth sends it a form: oRPC's handler reads one, but
				// its generator always writes the body as application/json.
				// (charter-lib/auth's securityOf calls this with an empty operation, for its security alone.)
				spec: op => {
					const body = op.requestBody as OpenAPIV3_2.RequestBodyObject | undefined, json = body?.content?.["application/json"];
					// Wrong client credentials: 401, which the handler answers before any token exists.
					const responses = { ...op.responses, "401": { description: "Unauthorized" } };
					return fern({ ...everyone, security: [], responses, ...(json && { requestBody: { ...body, content: { "application/x-www-form-urlencoded": json } } }) })(op);
				},
			}))
			// Not a tool: an MCP client sends its token as a header, as the SDKs do.
			.meta(notATool())
			// The client-credentials grant (RFC 6749, 4.4). The client's credentials are in it
			// (client_secret_post, what the SDKs send) or in an Authorization: Basic header
			// (client_secret_basic, what an OAuth client sends first, an MCP client among them).
			.input(z.strictObject({
				// A string, not an enum: with an enum here, Fern's Go SDK does not compile (the handler checks it).
				// Upstream: Fern, not filed (when fixed: z.enum(["client_credentials"]))
				grant_type: z.string().optional().describe("client_credentials, the one grant").meta({ examples: ["client_credentials"] }),
				client_id: z.string().optional().meta({ examples: ["id-1"] }),
				client_secret: z.string().optional().meta({ examples: ["secret-1"] }),
				scope: z.string().optional().describe("The scopes asked for: a token here may read and write whatever is asked"),
				resource: z.string().optional().describe("The API the token is for (RFC 8707): this one"),
			}))
			// The token response (RFC 6749, 5.1): a bearer token, for expires_in seconds.
			.output(z.object({ access_token: z.string(), token_type: z.string().meta({ examples: ["Bearer"] }), expires_in: z.number().int().meta(int32([3600])) })),
	},
	notes: {
		list: oc
			.meta(openapi({
				method: "GET", path: "/api/notes", summary: "List notes (cursor pagination)", tags: ["notes"], operationId: "listNotes", successDescription: "a page",
				spec: fern({ ...everyone, "x-fern-pagination": { cursor: "$request.cursor", next_cursor: "$response.next_cursor", results: "$response.data" } }),
			}))
			.input(z.object({
				cursor: z.string().optional().describe("Opaque cursor from the previous page's next_cursor").meta({ examples: ["2"] }),
				limit: z.coerce.number().int().min(1).max(100).default(2).meta(int32([2])),
			}))
			.output(z.object({ data: z.array(note), next_cursor: z.string().optional().describe("Pass as cursor for the next page; absent on the last page").meta({ examples: ["4"] }) })),
		create: oc
			.meta(openapi({
				method: "POST", path: "/api/notes", summary: "Create a note (idempotent: safe to retry with an Idempotency-Key)", tags: ["notes"], operationId: "createNote", successDescription: "created",
				spec: limited(writeLimit, op => ({ ...fern({ ...everyone, "x-fern-idempotent": true })(op), security: needs(["write"]) })),
			}))
			.input(z.strictObject({ body: z.string().refine(body => !body.includes(END), `a note may not contain ${END}`).meta({ examples: ["Buy milk"] }) }))
			.output(note),
		// A resumable SSE feed of the log: every event's id is its place there, so `after` and the
		// Last-Event-ID a client resends are the same position; a planned end (`seconds`) sends END.
		watch: oc
			.meta(openapi({
				method: "GET", path: "/api/notes/watch", summary: "Stream notes (Server-Sent Events) for `seconds`; call again with `after` = the last event's id to continue without gaps", tags: ["notes"], operationId: "watchNotes", successDescription: "notes",
				// Upstream: fern-api/fern#17938 (when fixed: Fern could read oRPC's envelope, event: message|close|error, as is)
				// oRPC describes the stream as its SSE envelope; tell Fern its `data:` payloads are notes.
				spec: op => {
					const stream = (op.responses?.["200"] as OpenAPIV3_2.ResponseObject | undefined)?.content?.["text/event-stream"] as OpenAPIV3_2.MediaTypeObject | undefined;
					if (stream) stream.schema = { $ref: "#/components/schemas/Note" };
					// resumable: the SDKs reconnect by themselves on a drop, sending Last-Event-ID. The
					// terminator marks a planned end, so they reconnect only on a genuine drop.
					return fern({ ...everyone, "x-fern-sdk-group-name": "notes", "x-fern-sdk-method-name": "watch", "x-fern-streaming": { format: "sse", terminator: END, resumable: true } })(op);
				},
			}))
			.input(z.object({
				after: z.string().regex(/^\d+$/).optional().describe("Resume after this event id (the last one you received). Absent: only notes created from now on").meta({ examples: ["3"] }),
				seconds: z.coerce.number().int().min(1).max(300).default(30).describe("How long to keep the stream open").meta(int32([30])),
			}))
			.output(asyncIteratorObject(note, z.literal(END).optional())),
		// The WebSocket channel: in the AsyncAPI spec, not in OpenAPI. Both ways are streams of plain JSON
		// messages: the client sends Subscribe, the server sends NoteEvent.
		live: oc
			.meta(openapi({ method: "GET", path: "/api/notes/live" }))
			.meta(asyncapi({
				channel: "liveNotes", address: "/api/notes/live", operationId: "receiveNoteEvent", message: "NoteEvent", send: { message: "Subscribe", operationId: "sendSubscribe" },
				summary: "Note events over a WebSocket: send `subscribe` with a topic, receive that topic's events",
				// A token on the channel's URL, which charter-lib/serve takes there and only there.
				query: z.object({ access_token: z.string().optional().describe("The OAuth token, for clients that can't send WebSocket headers (browsers, Workers). Others send Authorization") }),
			}))
			.input(asyncIteratorObject(z.strictObject({ topic: z.string().meta({ examples: ["notes"] }) }).meta({ id: "Subscribe" })))
			.output(asyncIteratorObject(z.object({
				event: z.string().meta({ examples: ["notes.created"] }),
				id: z.string().meta({ examples: ["note-1"] }),
				body: z.string().optional().meta({ examples: ["Buy milk"] }),
				auth: z.string().optional().describe("The Authorization value the server received on connecting: the harness test reads it"),
			}).meta({ id: "NoteEvent" }))),
	},
	files: {
		// A file in the input makes oRPC write (and read) the body as multipart/form-data.
		upload: oc
			.meta(openapi({ method: "POST", path: "/api/files", summary: "Upload a file (multipart)", tags: ["files"], operationId: "uploadFile", successDescription: "stored", spec: op => ({ ...fern(internal)(op), security: needs(["write"]) }) }))
			// Not a tool: a tool's arguments are JSON, and a file is not.
			.meta(notATool())
			.input(z.object({ file: z.file(), note: z.string().optional() }))
			.output(z.object({ id: z.string().meta({ examples: ["file-1"] }), size: z.number().int().meta(int32([1024])) })),
	},
	chat: oc
		.meta(openapi({
			method: "POST", path: "/api/chat", summary: "Stream a reply (Server-Sent Events)", tags: ["chat"], operationId: "chat", successDescription: "chunks",
			// Upstream: fern-api/fern#17938 (when fixed: Fern could read oRPC's envelope, event: message|close|error, as is)
			// oRPC describes the stream as its SSE envelope; tell Fern its `data:` payloads are chunks.
			spec: op => {
				const stream = (op.responses?.["200"] as OpenAPIV3_2.ResponseObject | undefined)?.content?.["text/event-stream"] as OpenAPIV3_2.MediaTypeObject | undefined;
				if (stream) stream.schema = { $ref: "#/components/schemas/Chunk" };
				return fern({ ...everyone, "x-fern-streaming": { format: "sse" } })(op);
			},
		}))
		.input(z.strictObject({ prompt: z.string().meta({ examples: ["Say hello"] }) }))
		.output(asyncIteratorObject(chunk)),
};

/** The calls the API makes to its users (OpenAPI 3.1 `webhooks`): each input is the payload sent. */
export const webhooks = {
	noteCreated: oc
		.meta(openapi({ method: "POST", summary: "Sent to you when a note is created", tags: ["notes"], operationId: "noteCreatedWebhook" }))
		.input(z.object({ event: z.string().meta({ examples: ["note.created"] }), note }).meta({ id: "NoteCreated" })),
};
