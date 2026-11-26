# Deploying

```sh
go tool charter deploy
```

That is the D1 migrations, then `wrangler deploy`.

## Nothing of your account in the repository

`wrangler.jsonc` names its database, KV namespace, bucket and queue without ids. wrangler makes them on the first deploy and finds them by name after.

```jsonc
"d1_databases": [{ "binding": "DB", "database_name": "my-app", "migrations_dir": "db/migrations" }],
"kv_namespaces": [{ "binding": "KV" }],
```

## Credentials

charter runs wrangler under fnox, which it brings itself. The token comes from your keychain or password manager, never from a file. In CI the two variables are the environment's.

## Secrets

The Worker's secrets are named in `.dev.vars.example`, each with a throwaway value for `charter dev`. Their real values are kept in fnox, beside Cloudflare's credentials, each under the Worker's name: `ADMIN_TOKEN` of the Worker `my-app` is `MY_APP_ADMIN_TOKEN`.

```sh
go tool charter run secrets                         # which the app names, and which fnox has
go tool charter run secrets:put ADMIN_TOKEN --random   # a new random value: kept in fnox, put on the deployed Worker
go tool charter run secrets:put MAIL_TO             # the value fnox has, or one piped in
```

A value is never printed and never on a command line. `go tool charter test --deployed` hands the tests the ones fnox has.

## After a deploy

```sh
go tool charter test --deployed
go tool charter run seo:audit
go tool charter run logs
```

## Environments

An environment of `wrangler.jsonc` is deployed with `go tool charter deploy staging` and tested with `go tool charter test --env staging`. charter keeps its address in `charter.toml`.

## Everything Cloudflare gives a Worker

workers-go's own packages work as its docs say (the example's package `built` uses each for one job, with a page that shows it): KV, R2, Queues, D1, Cron Triggers, the Cache API, Durable Objects, sockets and fetch. charter adds what it lacks: D1 as typed rows, rate limits and email. [The bindings page](../bindings) uses each one, with a test for each card.
