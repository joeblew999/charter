// Package transport is what a platform puts around the API's handler so that Go can serve
// WebSockets: workers-go can't answer an upgrade, and net/http needs a WebSocket library. In both
// places Go answers the upgrade request with plain HTTP, and an adapter carries it over the socket:
// worker/websocket.mjs on Cloudflare, Serve natively. Which paths are WebSockets, their input and
// what they send stay in the Go handler (and so in the contract).
//
// What Go answers a WebSocket upgrade (a GET with `Upgrade: websocket`) with:
//
//   - Anything but 2xx: a refusal (401, 422, 426). It is returned as it is, and no socket opens.
//   - 200 with a body: the feed, a stream of lines. Each line is sent as one text frame (empty lines
//     are padding). The feed only ends when it gave up, so the socket is then closed with 1011.
//   - 204: there is no feed. The socket stays open until the client closes it.
//   - The header MessagesHeader set to MessagesPost: the channel takes messages. Each text frame
//     the client sends is given to Go as a POST to the same URL, with the upgrade request's headers
//     (so Go authorizes every message as it did the upgrade) and the frame as the JSON body. The
//     answer is lines again, each sent as one frame. Messages are handled one at a time, in order.
//     Anything but 2xx closes the socket with 1008. Without the header, what the client sends is
//     ignored.
//
// The limit: on Cloudflare the feed and each message run in Go runtimes of their own (workers-go
// starts one per request), so a message can't change what the feed sends through memory. What must
// be shared goes through a binding, a Durable Object or a database, as the notes hub does.
package transport

// MessagesHeader, on the answer to an upgrade, says how the channel takes what the client sends.
const MessagesHeader = "X-Websocket-Messages"

// MessagesPost: every text frame from the client becomes a POST to the upgrade's URL.
const MessagesPost = "post"
