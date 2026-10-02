// oRPC has no AsyncAPI generator, and wants one as a community package first (middleapi/orpc#2115,
// closed). This file is that generator.
// AsyncAPI from an oRPC contract, the way @orpc/openapi does OpenAPI (docs/plans/asyncapi.md, issue #4;
// offered upstream in middleapi/orpc#2115). Built only on oRPC's public APIs: a meta plugin
// (`asyncapi()`, like `openapi()`), `walkProcedureContractsAsync`, and the JSON Schema converters.
//
// A procedure with `asyncapi({ channel, address })` metadata is a WebSocket channel:
// - its output, `asyncIteratorObject(x)`, becomes a `receive` operation whose message payload is `x`;
// - its input is one of two things. An object becomes the channel's query parameters
//   (`bindings.ws.query`). `asyncIteratorObject(y)` is what the client sends: it becomes a `send`
//   operation whose message payload is `y`, named by `send` in the metadata.
// One message type each way, and not query parameters and a send side together, so far.
import type { AnySchema, Meta, MetaPlugin, RouterContract } from "@orpc/contract";
import { getAsyncIteratorObjectSchemaDetails } from "@orpc/contract";
import { DelegatingJsonSchemaConverter, type JsonSchemaConverter } from "@orpc/json-schema";
import { walkProcedureContractsAsync, type AnyRouter } from "@orpc/server";

export interface AsyncAPIMeta {
	/** Channel id; Fern names the client after it (e.g. liveNotes -> client.liveNotes.connect()). */
	channel: string;
	/** Path of the WebSocket endpoint. */
	address: `/${string}`;
	/** Operation id of the receive operation. @default `receive<Channel>` */
	operationId?: string;
	/** Name of the message in components.messages. @default the channel id */
	message?: string;
	/**
	 * The client-to-server side, for a channel whose input is `asyncIteratorObject(...)`: the name of
	 * its message in components.messages (Fern's TypeScript client gets `send<Message>()`), and the
	 * send operation's id. @default operationId `send<Message>`
	 */
	send?: { message: string; operationId?: string };
	summary?: string;
	description?: string;
	/** Last word on the generated channel object (for x-fern-* extensions and the like). */
	spec?: (channel: Record<string, unknown>) => Record<string, unknown>;
}

/** Marks a procedure as an AsyncAPI (WebSocket) channel. */
export const asyncapi = (meta: AsyncAPIMeta): MetaPlugin<any, any, any> => ({
	name: "~asyncapi",
	init: (current: Meta) => ({ ...current, "~asyncapi": { ...(current as any)["~asyncapi"], ...meta } }),
});

export const getAsyncAPIMeta = (procedure: { "~orpc": { meta: Meta } }): AsyncAPIMeta | undefined =>
	(procedure["~orpc"].meta as any)["~asyncapi"];

export interface AsyncAPIGenerateOptions {
	/** @default '3.0.0' (what Fern reads) */
	version?: "3.0.0";
	/** Document fields to start from, such as `info` and `servers`. */
	base?: Record<string, unknown>;
}

export class AsyncAPIGenerator {
	private readonly converter: DelegatingJsonSchemaConverter;
	constructor(options: { converters?: JsonSchemaConverter[] } = {}) {
		this.converter = new DelegatingJsonSchemaConverter(options.converters);
	}

	async generate(router: RouterContract | AnyRouter, options: AsyncAPIGenerateOptions = {}) {
		const doc: Record<string, any> = { asyncapi: options.version ?? "3.0.0", ...options.base, channels: {}, operations: {}, components: { messages: {} } };
		const errors: string[] = [];
		await walkProcedureContractsAsync(router, (procedure, path) => {
			const meta = getAsyncAPIMeta(procedure as any);
			if (!meta) return;
			// oRPC 2.0 stacks .input()/.output() calls; one schema each is what this generator reads.
			const def = (procedure as any)["~orpc"] as { inputSchemas?: AnySchema[]; outputSchemas?: AnySchema[] };
			const [inputSchema, ...moreInputs] = def.inputSchemas ?? [];
			const [outputSchema, ...moreOutputs] = def.outputSchemas ?? [];
			if (moreInputs.length || moreOutputs.length) { errors.push(`${path.join(".")}: stacked .input()/.output() schemas are not supported yet`); return; }
			const details = getAsyncIteratorObjectSchemaDetails(outputSchema);
			if (!details) { errors.push(`${path.join(".")}: an AsyncAPI channel's output must be asyncIteratorObject(...)`); return; }
			const message = meta.message ?? meta.channel;
			const [payload] = this.converter.convert(details.yieldSchema, "output");
			doc.components.messages[message] = { name: message, payload };

			let channel: Record<string, unknown> = {
				address: meta.address,
				...(meta.summary && { summary: meta.summary }),
				...(meta.description && { description: meta.description }),
				messages: { [message]: { $ref: `#/components/messages/${message}` } },
			};
			// What the client sends: an input that is itself a stream of messages.
			const sent = getAsyncIteratorObjectSchemaDetails(inputSchema);
			const send = meta.send;
			if (sent && !send) { errors.push(`${path.join(".")}: a channel whose input is asyncIteratorObject(...) needs \`send: { message }\` in its asyncapi() metadata`); return; }
			if (sent && send) {
				const [sentPayload] = this.converter.convert(sent.yieldSchema, "input");
				doc.components.messages[send.message] = { name: send.message, payload: sentPayload };
				(channel.messages as Record<string, unknown>)[send.message] = { $ref: `#/components/messages/${send.message}` };
			} else if (inputSchema) {
				const [query] = this.converter.convert(inputSchema, "input");
				if (typeof query !== "object" || query.type !== "object") errors.push(`${path.join(".")}: an AsyncAPI channel's input must be an object (its query parameters)`);
				else channel.bindings = { ws: { query } };
			}
			if (meta.spec) channel = meta.spec(channel);
			doc.channels[meta.channel] = channel;
			const operationId = meta.operationId ?? `receive${meta.channel[0]!.toUpperCase()}${meta.channel.slice(1)}`;
			doc.operations[operationId] = {
				action: "receive",
				channel: { $ref: `#/channels/${meta.channel}` },
				messages: [{ $ref: `#/channels/${meta.channel}/messages/${message}` }],
			};
			if (sent && send) doc.operations[send.operationId ?? `send${send.message}`] = {
				action: "send",
				channel: { $ref: `#/channels/${meta.channel}` },
				messages: [{ $ref: `#/channels/${meta.channel}/messages/${send.message}` }],
			};
		});
		if (errors.length) throw new Error(`AsyncAPIGenerator:\n${errors.join("\n")}`);
		return doc;
	}
}
