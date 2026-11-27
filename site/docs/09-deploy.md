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

workers-go's own packages work as its docs say (the example's package `built` uses each for one job, with a page that shows it): KV, R2, Queues, D1, Cron Triggers, the Cache API, Durable Objects, sockets and fetch. charter adds what it lacks: D1 as typed rows, rate limits and email. [The bindings page](../bindings) uses each one, with a test for each card.
