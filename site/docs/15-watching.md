# Watching a deployed Worker

A deployed Worker fails in ways nothing on your machine shows. Cloudflare ends one that takes more than 128 MB of memory, or too much CPU time, and every request it was answering gets a 503: no code of the app runs, no test of its logic fails, and `charter dev` has no such limits. charter looks where that is recorded, so that you do not have to.

## Is it well?

```sh
go tool charter run health               # the last 24 hours
go tool charter run health --since 7d    # another span: 30m, 6h, 7d
go tool charter run health staging       # an environment
```

```
charter-example, the last 7 days: 10,571 requests
✗ 123 (1.2%) ended because the Worker went over its 128 MB of memory: every request it was answering then got a 503. …
· 1,281 readers left before their answer was done (a stream that was closed: usual)
· CPU time: half under 12 ms, 99% under 192 ms
```

It is Cloudflare's own count of how each request ended. A request the Worker answered is well there whatever it answered; what this finds is the requests it never got to answer. It ends with an error when there are any, so a pipeline can run it.

Two commands ask it for you:

| Command | What it adds |
|---|---|
| `go tool charter test --deployed` | After the tests, how the Worker did while they ran. A request Cloudflare ended fails the run, though every test passed |
| `go tool charter deploy` | Before the new version goes out, how the one that is going did the last day |

## What happened?

Workers Logs keep what the Worker printed and how each request ended, for a week. They are on in `wrangler.jsonc`:

```jsonc
"observability": { "enabled": true },
```

```sh
go tool charter run logs                              # live, until Ctrl-C
go tool charter run logs --since 1h                   # what was kept
go tool charter run logs --since 1h --errors          # only what went wrong
go tool charter run logs --since 1d --find 8f3a1b2c   # one request, by its reference
```

`--errors` is a line printed as an error, a request Cloudflare ended, or one answered with a 5xx. A reader who closed a stream is not one.

## A failure has a reference

When a page or a handler returns an error the app does not know how to answer, the reader is told "Something went wrong" and a reference, and never what went wrong inside. The app logs one line of JSON for it, which Workers Logs keeps field by field:

```json
{"level":"error","message":"the app could not answer a request","method":"POST","path":"/notes","route":"POST /notes","status":500,"error":"…","reference":"8f3a1b2c3d4e5f60-SIN"}
```

The reference is Cloudflare's own id of the request, and is also the answer's `X-Request-Id`. Someone who says "it said reference 8f3a…" has named that very request: `--find` brings its line.

The line says what was asked and what went wrong. It never has what was sent: no body, no query, no header, nobody's name.

## Memory

A Go runtime holds 13 MB or more for as long as it serves a request. So two things are built in:

- **Nothing long holds one.** A page's stream is held by the hub, in JavaScript ([live feeds](05-live.md)).
- **The runtimes share a budget.** The Worker counts what all its Go runtimes hold and keeps the sum under it; a request that would take it over waits a moment for one to finish.

The [admin screen](14-admin.md)'s system page says how many Go runtimes this instance has alive, what they hold, and what they may. A request with the header `x-go-runtime` is told the same in the answer's.

## What charter leaves to Cloudflare

- **Sending logs elsewhere** (a log store of your own): `wrangler.jsonc`'s `observability` and Logpush.
- **Being told** when something is wrong: a notification on the Worker's error rate, in the Cloudflare dashboard.
- **Lowering what is kept**, for a busy app: `"observability": { "enabled": true, "head_sampling_rate": 0.1 }`.
