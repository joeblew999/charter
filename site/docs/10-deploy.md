# Deploying

```sh
go tool charter deploy
```

That is the D1 migrations, then `wrangler deploy` twice: a deployed app is two Workers.

## Two Workers

| | Name | What it is |
|---|---|---|
| The Go Worker | `<name>-go` | The app as `wrangler.jsonc` has it: every binding, its cron, its queue, its secrets. It has no address of its own |
| The Worker in front | `<name>` | A few KB of JavaScript at the app's address, with the browser's files. It answers a page that is the same for every reader from KV and hands everything else to the Go Worker |

You write one `wrangler.jsonc`, which is what `charter dev` runs, as one Worker. `charter deploy` writes the two configurations from it each time (`wrangler.go.built.json`, `wrangler.front.built.json`, not kept in git).

Why two: a Worker that is not running where a request arrives is loaded there first, and that takes as long as it is large. Measured on 2026-11-28, a one-line answer from an 18 MiB Worker took 210 to 630 ms where it was not running and 25 ms where it was, whether or not Go was started. The Worker in front is small enough to be running anywhere in a few milliseconds, and KV is one copy for every data centre ([a shared page](04-pages.md#a-page-that-is-the-same-for-every-reader)).

- **Health, logs and secrets are the Go Worker's**: that is where the app runs. `run health`, `run logs`, `run secrets` and `run secrets:put` mean it.
- **An app deployed as one Worker becomes two on its next deploy.** The Worker of the app's name hands its queue and cron to the Go Worker and becomes the one in front. A secret that fnox does not have cannot be moved (Cloudflare never gives one back): deploy names it, and it has to be put again.
- **Each of an app's requests comes through the Worker in front**, which gives them to fewer instances of the Go Worker than Cloudflare spreads readers over. The Go Worker keeps what its runtimes hold under a budget that follows the size of the build ([watching](16-watching.md)).

## With the commands

| When | Command | What it does |
|---|---|---|
| You ship | `deploy` | applies the migrations, then deploys the Go Worker and the Worker in front with wrangler; the first time it makes what `wrangler.jsonc` names and writes the ids there |
| After it | `test --deployed` | the same tests, asked of the deployed Worker, which they leave as they found it |
| A secret | `run secrets`, `run secrets:put NAME` | what is declared and what the Worker has; one kept by fnox and put on the Worker |
| To see what is there | `run resources` | the database, KV namespace, bucket and queue the deployed Worker is bound to |
| To take it away | `run delete` | the Worker and its secrets; its data only when you say so |

**Checked before it is deployed:** everything `check` says of the code; that the deployed database gets the schema that code was checked against, before the code; that a resource is the one it was (its id), so a renamed bucket stops the deploy and does not make an empty one.

**Not checked by a deploy:** that the app works there, and that each secret it names has a value on the Worker. `test --deployed` and `run secrets` say. A deploy waits on neither: it tells you how the Worker did in the day before.

## The plan

A charter app is for Workers Paid ($5 a month). Workers Free allows [10 ms of CPU time a request](https://developers.cloudflare.com/workers/platform/limits/#cpu-time), and a Go Worker takes that or more for many of its requests. What Cloudflare allows a Worker and what it charges, each with a link to Cloudflare's own page, is in [Cloudflare's limits](16-watching.md#cloudflares-limits).

## Nothing of your account in the repository

`wrangler.jsonc` names its database, KV namespace, bucket and queue without ids. wrangler makes them on the first deploy and finds them by name after.

```jsonc
"d1_databases": [{ "binding": "DB", "database_name": "my-app", "migrations_dir": "db/migrations" }],
"kv_namespaces": [{ "binding": "KV" }],
```

## Credentials

charter runs wrangler under fnox, which mise installs (`mise.toml`). The token comes from your keychain or password manager, never from a file. In CI the two variables are the environment's.

## Secrets

The Worker's secrets are named in `.dev.vars.example`, each with a throwaway value for `charter dev`. Their real values are fnox's to keep, declared in the app's own `fnox.toml`, which you commit:

```toml
[secrets]
ADMIN_TOKEN = { provider = "keychain", value = "CHARTER_MY_APP_ADMIN_TOKEN", if_missing = "ignore" }
```

That is a name and where its value is kept, never the value. The provider is yours to choose: the keychain, a password manager, an encrypted file ([fnox's providers](https://fnox.jdx.dev)). So each app has its own secrets, and nothing of an app's is in your global config.

```sh
go tool charter run secrets                            # which the app names, and which fnox has
go tool charter run secrets:put ADMIN_TOKEN --random   # a new random value: kept by fnox, put on the deployed Worker
echo "$address" | go tool charter run secrets:put MAIL_TO   # a value of your own, piped in
```

A value is never printed and never on a command line. charter does not write one into `fnox.toml`: with no provider to keep it, it stops and says how to add one. `go tool charter test --deployed` hands the tests the secrets fnox has.

An environment of `wrangler.jsonc` is a profile of `fnox.toml`, with the same names and its own values:

```sh
go tool charter run secrets staging
go tool charter run secrets:put staging ADMIN_TOKEN --random
```

Someone who clones the app has the file and not the values. charter tells them which are missing; nothing else stops.

## Resources

The Worker's database, KV namespace, bucket and queue are named in `wrangler.jsonc`, and the first deploy makes them. What a resource *is* is its id (a database, a KV namespace) or its name (a bucket and a queue, which Cloudflare knows by name and cannot rename).

charter deploy writes the ids into `wrangler.jsonc` after it has made them, and you commit that:

```jsonc
"d1_databases": [{ "binding": "DB", "database_name": "my-app", "database_id": "c26412c3-…" }],
"kv_namespaces": [{ "binding": "KV", "id": "a20e27f8…" }],
```

From then on the Worker is bound to exactly that database and that namespace, whatever the Worker or the resource is later called. Rename the database in the file and it is the same database, with its rows.

A bucket's or a queue's name is different: a changed name is a new, empty one, and the old one is left behind with what it holds. charter deploy stops and says so; `go tool charter deploy --new-resources` goes ahead on purpose.

```sh
go tool charter run resources    # what the file names, and what the deployed Worker is bound to
```

The ids are no secrets. They do tie the repository to the account it deploys to; `charter create` starts a new app without them.

## Taking an app away

```sh
go tool charter run delete                         # what there is: the Worker, its secrets, its data. Changes nothing
go tool charter run delete my-worker               # all of it
go tool charter run delete my-worker --keep-data   # the Worker and its secrets; its data stays
```

It deletes, so it is asked for twice: the second time with the Worker's name. Deleting the app deletes all of it (both Workers, and the pages the one in front kept), and `wrangler.jsonc` no longer names the ids of what is gone, so the next deploy makes its own.

With `--keep-data` the database, namespace, bucket and queue stay, and the file still names them: `go tool charter deploy` brings the same app back with the same data, and with its secrets from fnox, which deploy puts on a Worker that lacks them.

Cloudflare never gives a secret back. So while the Worker has a secret that fnox does not, delete stops and names it.

## Email

A Worker sends email through its `send_email` binding. To an address verified on your account it sends as it is. To anyone, the domain it sends from is onboarded once (Cloudflare Email Sending, on the Workers Paid plan):

```sh
go tool charter run mail                           # the domains that send, in the account
go tool charter run mail:enable mail.example.com   # onboard one, and show its records
```

Cloudflare adds the records that let it send for that domain. A subdomain keeps them apart from the domain's own mail.

## After a deploy

```sh
go tool charter test --deployed
go tool charter run seo:audit
go tool charter run logs
```

## Environments

An environment of `wrangler.jsonc` is deployed with `go tool charter deploy staging` and tested with `go tool charter test --env staging`. charter keeps its address in `charter.toml`.

## Everything Cloudflare gives a Worker

workers-go's own packages work as its docs say (the example's package `built` uses each for one job, with a page that shows it): KV, R2, Queues, D1, Cron Triggers, the Cache API, Durable Objects, sockets and fetch. charter adds what it lacks: D1 as typed rows, rate limits and email. [The example's pages for what it is built with](https://charter-example.gedw99.workers.dev/built/) use each one, each on a page of its own, with a test for each.
