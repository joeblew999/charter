---
title: Rate limits
nav_order: 7
---

# Rate limits: how often each caller may call

A limited operation takes at most `limit` calls per caller in each `period` (10 or 60 s); one more is 429 with `Retry-After`. The caller is who auth let in, or the client's IP on a public operation; a call auth refuses is not counted. In the notes examples, creating a note takes 100 a minute.

```go
ratelimit.Declare(config.OpenAPI, ratelimit.Limit{Binding: "READ_LIMIT", Limit: 600, Period: 60, Scope: "read"}) // per scope
Extensions: ratelimit.Limited(ratelimit.Limit{Binding: "WRITE_LIMIT", Limit: 100, Period: 60}, nil)                   // per operation
```

In TypeScript: `spec: needs(["write"], limited(writeLimit))`, and `rateLimits` in the document. Each limit is a [Workers Rate Limiting](https://developers.cloudflare.com/workers/runtime-apis/bindings/rate-limit/) binding, which charter binds from the spec (`spec` in `charter.toml`), its counter named from the Worker and the binding. It counts per Cloudflare location and is eventually consistent: it stops abuse, it does not account. A Worker without the binding lets every call through, and logs it once.

Locally, `examples/conformance-ts/test/auth.ts` checks 429 with `Retry-After` after the write limit for one caller while another still writes (`charter check`, 2026-10-29). On Cloudflare, `charter live-test` of the conformance projects checks it (`examples/conformance-ts/test/ratelimit.ts`): on 2026-10-15, against both, a 429 with `Retry-After: 60` after 101 writes by one caller, while another still wrote.
