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

## After a deploy

```sh
go tool charter test --deployed
go tool charter run seo:audit
go tool charter run logs
```

## Environments

An environment of `wrangler.jsonc` is deployed with `go tool charter deploy staging` and tested with `go tool charter test --env staging`. charter keeps its address in `charter.toml`.

## Everything Cloudflare gives a Worker

workers-go's own packages work as its docs say: KV, R2, Queues, D1, Cron Triggers, the Cache API, Durable Objects, sockets and fetch. charter adds what it lacks: D1 as typed rows, rate limits and email. [The bindings page](../bindings) uses each one, with a test for each card.
