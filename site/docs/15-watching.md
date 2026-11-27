# Watching a deployed Worker

A deployed Worker fails in ways nothing on your machine shows. Cloudflare ends one that takes more than [128 MB of memory](https://developers.cloudflare.com/workers/platform/limits/#memory), or [too much CPU time](https://developers.cloudflare.com/workers/platform/limits/#cpu-time), and every request it was answering gets a 503 ([all of Cloudflare's limits](#cloudflares-limits)): no code of the app runs, no test of its logic fails, and `charter dev` has no such limits. charter looks where that is recorded, so that you do not have to.

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

Nobody has to remember to ask. It is asked where it costs no time:

| When | What is asked |
|---|---|
| `go tool charter deploy` | How the version that is going did the last day, before the new one goes out |
| `go tool charter test --deployed` | First, how the Worker did the last hour. At the end, what Cloudflare has counted of the run so far: a request it ended fails the run, though every test passed |
| Once a day | The last day, for every app, by a scheduled workflow (`.github/workflows/health.yml`): a day that was not well fails it, and GitHub tells you |

Cloudflare takes up to a minute to count a burst of requests. A test run does not wait for that: it asks once, at once, and what is counted later is seen by the next deploy, the next test and the daily check. What goes wrong under real use does so over hours, which is why the daily check is where it is watched for, and why no deploy waits on any of this.

## Three numbers

Go on Workers lives within three limits, and an app drifts towards each a little at a time. charter says each where it is known anyway, keeps the last ones (`.charter/times.json`), and remarks when one has grown or is near its limit:

| Number | Its limit | Said by |
|---|---|---|
| The size of what is deployed, uncompressed | [64 MiB](https://developers.cloudflare.com/workers/platform/limits/#worker-size), on every plan | `charter build` |
| The CPU time the Worker takes to start | [1 second](https://developers.cloudflare.com/workers/platform/limits/#worker-startup-time) | `charter deploy`, from what wrangler reports |
| The memory one Go runtime holds | [128 MB](https://developers.cloudflare.com/workers/platform/limits/#memory) for all of them | `charter test`, from what the Worker says |

```
✓ build/: 27.25 MiB of the 64.00 MiB a Worker may be, uncompressed
· a Go runtime holds about 20 MB: 4 fit in the 80 MB the Worker gives them, so it answers that many at once in one instance
```

The size is all of `build/`: the Wasm, and the JavaScript wrangler bundles from the rest, which is what `wrangler deploy --dry-run` calls the Total Upload. Cloudflare counts it as it is: "There is no compressed size limit. Only the uncompressed bundle size counts", the same on the free plan as on the paid one ([Cloudflare's limits](https://developers.cloudflare.com/workers/platform/limits/#worker-size)). The build fails above it, or above `[build] limit` in `charter.toml` when an app sets one lower. What a larger Worker costs is time to start, which is the second number.

A number is remarked on when it is past 80% of its limit, or above what is usual here by more than it varies by itself: 10% for the size, which is the same for the same code; 50% for the start-up, which varies from deploy to deploy; 30% for a runtime's memory, which grows with what it has served.

They are the numbers that say when a limit is coming, and the ones to measure a change of compiler or of design against.

## Cloudflare's limits

What Cloudflare allows a Worker, and what it charges, as its own pages said on 2026-11-27. They change: the links are the authority.

| Limit | Workers Free | Workers Paid | Cloudflare's page |
|---|---|---|---|
| The Worker's size, uncompressed. There is no limit on its compressed size | 64 MiB | 64 MiB | [Worker size](https://developers.cloudflare.com/workers/platform/limits/#worker-size) |
| CPU time to start (the module's top level) | 1 second | 1 second | [Worker startup time](https://developers.cloudflare.com/workers/platform/limits/#worker-startup-time) |
| Memory, for each isolate, shared by every request it is answering | 128 MB | 128 MB | [Memory](https://developers.cloudflare.com/workers/platform/limits/#memory) |
| CPU time for each request | 10 ms | 30 s by default, up to 5 minutes | [CPU time](https://developers.cloudflare.com/workers/platform/limits/#cpu-time) |
| Requests | 100,000 a day | no limit | [Daily requests](https://developers.cloudflare.com/workers/platform/limits/#daily-requests) |

What Workers Paid charges, beyond its $5 a month:

| What is counted | Included each month | Then | Cloudflare's page |
|---|---|---|---|
| Requests to the Worker (requests for static assets are free) | 10 million | $0.30 a million | [Workers pricing](https://developers.cloudflare.com/workers/platform/pricing/#workers) |
| CPU time while the Worker runs: not time spent waiting for D1, a fetch or KV, and not its start-up | 30 million ms | $0.02 a million ms | [Workers pricing](https://developers.cloudflare.com/workers/platform/pricing/#workers) |
| A Durable Object's time while it is active, counted at 128 MB whatever it holds | 400,000 GB-s | $12.50 a million GB-s | [Durable Objects pricing](https://developers.cloudflare.com/durable-objects/platform/pricing/#compute-billing) |
| Requests to a Durable Object | 1 million | $0.15 a million | [Durable Objects pricing](https://developers.cloudflare.com/durable-objects/platform/pricing/#compute-billing) |

That a Worker's start-up is not billed is said on [Dynamic Workers' pricing](https://developers.cloudflare.com/dynamic-workers/pricing/#cpu-time), where it is the difference.

What that means for a charter app, from the example over the 30 days to 2026-11-27:

- **It is for Workers Paid.** A Go Worker takes more than Free's 10 ms of CPU for many of its requests: the example's median is 10 ms, its 99th percentile 166 ms.
- **Size is not near its limit.** The example is 27.25 MiB of the 64. What size costs is time to start, which is not billed: it is the reader who waits for it.
- **Memory is the limit a Go Worker meets first.** A runtime holds 13 MB or more of the isolate's 128 MB ([below](#memory)). While a Go runtime held each page's stream, the example had 123 requests ended over it, on 2026-11-26 and 27; since the hub has held the streams (bdf9572), it has had none.
- **CPU time is what it pays for.** The example took 20 ms a request: 0.42 million ms for its 20,930 requests, well inside the 30 million included. Past what is included, 20 ms a request is $0.40 a million requests, beside the $0.30 for the requests themselves.
- **The hub is billed by time.** A page's stream held open keeps the hub's Durable Object active, which is counted at 128 MB a second. The example's hub was active for 1,406 seconds: 180 GB-s of the 400,000 included.

`go tool charter run health` says the CPU time and how requests ended. What the account has used this month, for every Worker on it, is on the Cloudflare dashboard's billing page.

## Under load

What a Go Worker does wrong under load it does only on Cloudflare, and only when requests come together. Before a release, or when something about what the Worker holds has changed:

```sh
go tool charter run load --deployed                          # 20 at once for 10 seconds, 10 streams held open
go tool charter run load --deployed -at 50 -for 30s -streams 40
go tool charter run load                                     # wrangler dev: how fast, with none of Cloudflare's limits
```

```
https://charter-example.gedw99.workers.dev: 30 pages asked 20 at once for 10s, with 10 streams held open
3,326 requests, 330 a second: half answered within 37ms, 99% within 503ms
✓ every request was answered
charter-example, while it was loaded: 3,222 requests
✓ every one was answered: none went over its memory or CPU time, none threw
```

It only reads: the pages of the sitemap, and the streams they open. It waits until Cloudflare has counted what it sent, so what it says is of all of it; that takes about a minute, and is why it is no part of a deploy.

## A first hit

Where no instance of the Worker is running, a reader waits for one to start. For an app without constant traffic everywhere, most readers in most places are that first hit, and one machine never shows it: its second request finds the instance its first one started.

```sh
go tool charter run cold                        # the deployed Worker's front page, from 30 places
go tool charter run cold -path /docs/ -from 50
go tool charter run cold staging                # an environment
```

```
charter-example, GET /docs/ from 30 places (Globalping), the time from the connection to the first byte:
· the first request: 29 met a new instance (half under 628 ms, 90% under 979 ms); 1 met a running one (half under 520 ms, 90% under 520 ms)
· again from each: 23 met a new instance (half under 501 ms, 90% under 635 ms); 7 met a running one (half under 54 ms, 90% under 131 ms)
```

The places are the probes of [Globalping](https://globalping.io), a public service: it is told the Worker's address and the page, and nothing else. Each asks once, then again. Which answers were a first hit is counted, not guessed: the Worker says which runtime answered (the header `x-go-runtime`), and one that started as the instance did is a first hit. The time is from the connection made to the first byte, so it is what the Worker and Cloudflare take, without the reader's distance to Cloudflare. Without an account Globalping gives an address 250 probes an hour; this takes two for each place.

What it found on 2026-11-27, from 30 places each:

| | A first hit, half under | A running instance, half under |
|---|---|---|
| `site/`, `/docs/`: 14.4 MiB, starts in 103 ms of CPU | 433 ms | 55 ms |
| The example, `/docs/`: 27.3 MiB, starts in 142 to 224 ms | 628 ms | 54 ms |
| The example, `/`, which reads D1 | 810 ms | 266 to 313 ms |

- **A second request is still a first hit, two times in three.** Cloudflare runs a Worker on many machines at a location, and each starts its own instance. So what counts is less how long an instance is kept than how many of them a reader's requests meet.
- **Of a first hit, charter's part is the smaller.** The request itself took at most 87 ms and the module's start-up 142 to 224 ms of CPU, by Cloudflare's own counts; the rest is Cloudflare making the instance, before any code of the app runs.
- **A smaller Worker that starts sooner has a shorter first hit**: `site/` is half the example's size and 0.2 s sooner.
- **A page that reads D1 waits for it**, 0.2 s and more, on a running instance too.

It is for after a change to what the Worker loads or does as it starts. It is no part of a deploy.

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

A Go runtime holds 13 MB or more for as long as it serves a request, of the [128 MB](https://developers.cloudflare.com/workers/platform/limits/#memory) an isolate has for every request it is answering. So two things are built in:

- **Nothing long holds one.** A page's stream is held by the hub, in JavaScript ([live feeds](05-live.md)).
- **The runtimes share a budget.** The Worker counts what all its Go runtimes hold and keeps the sum under it; a request that would take it over waits a moment for one to finish.

The [admin screen](14-admin.md)'s system page says how many Go runtimes this instance has alive, what they hold, and what they may. A request with the header `x-go-runtime` is told the same in the answer's.

## What charter leaves to Cloudflare

- **Sending logs elsewhere** (a log store of your own): `wrangler.jsonc`'s `observability` and Logpush.
- **Being told** when something is wrong: a notification on the Worker's error rate, in the Cloudflare dashboard.
- **Lowering what is kept**, for a busy app: `"observability": { "enabled": true, "head_sampling_rate": 0.1 }`.
