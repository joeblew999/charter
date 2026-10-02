// The specs generated from the contract give Fern what the hand-written specs gave it.
// handwritten-surface.json is surface() of the showcase's last hand-written openapi.json and
// asyncapi.yml (commit c6093ae), so this test is the claim "same SDK surface", checkable. Every
// difference is named below, with why. If the contract changes on purpose, change this list too
// (and the Go server of the same API, ../showcase-go/fern, once it is here).
//   mise run test
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { test } from "node:test";
import { specs } from "../src/specs.ts";
import { surface } from "./surface.ts";

const server = "https://charter-showcase-ts-api.gedw99.workers.dev/api/mock";
const json = (url: URL) => JSON.parse(readFileSync(url, "utf8"));

const expected = json(new URL("handwritten-surface.json", import.meta.url));
// oRPC writes 3.1.1 (asked for in ../notes-ts/src/specs.ts); the hand-written file said 3.1.0.
expected.openapi.version = "3.1.1";
// The WebSocket server is the deployed harness, as in openapi.json; it was a placeholder (api.example.com).
expected.asyncapi.servers.production = { host: "charter-showcase-ts-api.gedw99.workers.dev", pathname: "/api/mock", protocol: "wss" };
// The server has always sent `auth` in each event and the harness test reads it. The contract
// validates what a handler yields, so the field is now declared.
expected.asyncapi.channels.liveNotes.messages.NoteEvent.properties = { auth: { type: "string" }, ...expected.asyncapi.channels.liveNotes.messages.NoteEvent.properties };

/** Without the keys that are undefined, as the fixture (JSON) has it. */
const plain = (value: unknown) => JSON.parse(JSON.stringify(value));

test("the contract's specs have the hand-written specs' surface, but for the named differences", async () => {
	assert.deepStrictEqual(plain(surface(await specs.openapi(server), await specs.asyncapi(server))), expected);
});

test("so do the committed specs, which Fern reads", () => {
	const committed = (file: string) => json(new URL(`../fern/${file}`, import.meta.url));
	assert.deepStrictEqual(plain(surface(committed("openapi.json"), committed("asyncapi.json"))), expected);
});
