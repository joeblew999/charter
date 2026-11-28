# The commands, and what each one proves

You build a charter app by running a handful of commands while you, or your coding agent, add code. This page goes through them in the order you meet them, and says for each one what it does and what it proves about your code.

That second part is the reason for the stack. Each tool in it was picked because it lets a mistake be found by a machine before a reader finds it: gsx turns templates into Go, structpages turns routes into Go types, sqlc checks SQL against the schema, Huma writes the API's spec from Go types, Fern writes clients from that spec. charter's job is to run them at the right moment so you never have to remember to.

Not everything is checked by a compiler. Where it is not, this page says so, and says what catches the mistake instead.

## What is checked, and when you find out

| What you write | Written once, in | Who checks it | You find a mistake |
|---|---|---|---|
| A page or a component | a `.gsx` file | gsx writes Go from it; the Go compiler checks every value, prop and call | on save, under `dev` |
| A link, an element's id, a swap's target | the page tree (Go structs) | structpages: `url`, `id`, `target` take a page's type, never a string; structpages-lint checks each against the tree | on save for a type that is gone; in `check` for the rest |
| A table | `db/schema.sql` | charter writes the migration from it | on save, under `dev` |
| A statement on the database | `db/queries.sql` | sqlc checks its columns and types against the schema; charter writes it as a Go function | on save, under `dev` |
| Who may do what | a scope, declared once as a Go value (`var Write = charter.NewScope("write")`) | the compiler: a page's `Needs`, an operation's `Security` and `caller.May` take the value, never a word | on save, under `dev` |
| A binding, a variable, a secret's name | `wrangler.jsonc` (a secret's name: `.dev.vars.example`, `fnox.toml`) | charter writes each as a Go value of its kind (`env.DB`, `env.KV`, `env.APP_NAME`); the compiler refuses one that is not declared, or of another kind | on save, under `dev` |
| The API | Go types (`api/contract.go`) | Huma writes the OpenAPI spec from the types; `check` compares it with the committed one | in `check` |
| A client of the API, in any language | nothing: it is generated | Fern writes it from the spec; it is proven by calling the Worker | in `test` and `pack --prove` |
| Text, in every language | `locales/*.toml` | charter writes each message as a Go name, so a page that uses one that is gone does not compile | on save; `check` for a language that is incomplete |
| A live feed | `live.Feed[Note]` | the Go compiler: a feed is of one type, for pages, API and SDK | on save |

The same holds for what Cloudflare gives the Worker. Take a bucket out of `wrangler.jsonc` and every line that used it stops compiling, because `env.BUCKET` is gone. Hand a KV namespace to something that takes a database and it does not compile either.

So one change, a column renamed in `db/schema.sql`, is followed through by machines: the migration is written, every query that used the column fails, the Go that used the row no longer compiles, the API's spec changes and `check` says the committed one is stale, and the generated clients change with it.

## What no compiler checks

These are strings or behaviour, and a wrong one compiles. Each is found by `charter test`, which runs the app on a real Worker, and not before.

| What | Why it is not checked | What finds a mistake |
|---|---|---|
| That a binding is there on the deployed Worker, and a secret has a value | The name is checked; what Cloudflare has under it is not Go's to know | `deploy` makes what `wrangler.jsonc` names; `charter run resources` and `charter run secrets` say what the Worker has; `test --deployed` |
| That a role grants what you meant it to | A scope is checked; whether a moderator should have it is your decision | `test`, signed in as each role |
| What a page's `Props` asks for | structpages hands it over by its type when the page is asked for | `test`: the page answers with an error |
| An htmx attribute written by hand: `hx-swap="innerMorph"` | It is text for htmx. What charter gives for the hard cases (`live.Follows()`, `app.Body()`) is Go | `test`, in a real browser |
| JavaScript: `web/`, and the glue that runs Go | charter has no TypeScript. It is kept small and free of logic | `test`, in a real browser and on the Worker |
| Go calling Cloudflare inside charter's `cloud` package | Go on Workers calls JavaScript by name | charter's own tests, on a Worker |
| Three statements of the admin screen | They ask a database what tables it has, which is no table of a schema | charter's own tests |

If you can express something as a Go type or a query, do: it moves from this table to the one above.

## `create`

```sh
mise exec go@latest -- go run github.com/joeblew999/charter/cmd/charter@latest create my-app
```

Copies [the example](https://github.com/joeblew999/charter/tree/main/example), the one app that uses all of charter, under your name. It pins the tools in the app's own `mise.toml`, installs them and writes the specs.

**What it proves:** nothing yet, but what it gives you passes `check` and `test` as it stands. You start from an app in which every layer above is already wired, and change it.

## `dev`

```sh
go tool charter dev
```

The loop you work in. It runs the app on the real runtime (workers-go on `wrangler dev`, with the bindings of `wrangler.jsonc`) and stays running; every other command works beside it.

Each save runs the generator that owns the file, then the Go compiler:

| You save | What is written for you | What is then checked |
|---|---|---|
| A `.gsx` file | its Go | the compiler: every value and prop in the template |
| A `.go` file | | the compiler |
| `db/schema.sql` | the migration, and the local database follows | every query, again, against the new schema |
| `db/queries.sql` | `db/queries.gen.go`: a function and a row type for each query | sqlc: columns and types; then the compiler, on every caller |
| `locales/*.toml` | the messages as Go names | the compiler, on every page that uses one |
| `wrangler.jsonc`, `.dev.vars.example`, `fnox.toml` | `env/env.built.go`: each binding, variable and secret's name as a typed value | the compiler, on every line that uses one |

A mistake shows in the browser in gsx's overlay within a second or two, and the last page that worked stays up.

**What it proves:** that the app compiles, with its templates, queries and messages. That is most type mistakes, at the moment you make them.

**What it does not:** it runs no linter and no tests, and it does not compare the API's spec. A link to a page that was removed can still be there. That is `check`.

## `add`

```sh
go tool charter add screen:sign-in
go tool charter add ui:dialog
go tool charter add locale:de
go tool charter add d1:DB
```

Brings something more into the app: a screen of charter's, a gsxui component, a language, a Cloudflare resource with its binding, a Go module or an npm package.

**What it proves:** what it adds arrives in the checked form. A screen is a `.gsx` file in your app, built from your own components, so it is compiled as your pages are. `screen:sign-in` writes its tables into your `db/schema.sql`, and from then on `check` holds your schema to the statements charter runs on them. `d1:DB` writes the binding into `wrangler.jsonc`, and with that `env.DB` exists for your Go to use.

## `check`

```sh
go tool charter check          # says
go tool charter check --fix    # writes what can be written
```

Everything that can be said without running the app. It starts nothing, so it is safe beside `dev`. Run it before a push; CI runs it too.

| Part | What it checks |
|---|---|
| `tools` | Go, Node and fnox are the versions `mise.toml` pins, so what you build is what CI builds |
| `generate` | the messages, the components' Go and the browser's files are written again, so everything after reads what is true now |
| `fmt` | Go and gsx files are formatted |
| `lint` | `go vet`, twice: for your machine and for the Worker's build. Then structpages-lint: every link, id and target against the page tree, and any internal address written as a string |
| `statements` | no statement on the database is written in Go: each is a query in `db/queries.sql`, checked by sqlc ([the database](07-database.md#every-statement-is-checked)). And your schema still has the sign-in screen's tables as charter uses them |
| `bindings` | no binding, variable or secret is named as text (`kv.NewNamespace("KV")`, `cloudflare.Getenv("X")`): each is its value in `env`, which only exists for what is declared |
| `scopes` | a scope is made only where it is declared, as a package's variable: never from a word where it is used |
| `contract` | the committed OpenAPI and AsyncAPI specs and the queries' Go are what the code gives now, and each rate limit the contract declares is bound |
| `languages` | every language is complete, plural forms are right, no message is unused |
| `screens` | charter's screens that you have not changed are charter's current ones |

**What it proves:** the whole first table on this page, for the whole app at once, and that nothing generated is stale. `dev` checks what you touched; `check` checks everything, including what a change somewhere else broke.

**What it does not:** it never runs your code. Everything in "What no compiler checks" is still open after a green `check`.

`check --json` gives the same as one JSON document, each part with what it said, for a script or a coding agent.

## `test`

```sh
go tool charter test
go tool charter test --deployed
```

Runs `go test` for plain logic, then starts the Worker on `wrangler dev` with a new database filled by [the seed](07-database.md#seeding), and tests it from outside: the pages in a real browser, the API through its own generated SDK, and the site as a search engine reads it.

**What it proves:** what types cannot. That the bindings answer, that a signed-in member may do what a member may and no more, that a page asks for what it is given, that a list updates in a second tab, that the JavaScript and Go meet. And it proves the generated clients against a running Worker, not only that they compile.

The tests use the types too: a test names a page by its type and an element by its id from the tree, never by an address or a selector, so a renamed page fails the test at compile time and not by timing out.

**What it does not:** what only shows at scale or far away: memory under many readers, a first hit on another continent. Those are [`run load` and `run cold`](16-watching.md), before a release.

`--deployed` runs the same tests against the deployed Worker, and leaves it as it found it.

## `deploy`

```sh
go tool charter deploy
```

Applies the migrations that have not run, then deploys with wrangler. The first time, it makes the database, the KV namespace, the bucket and the queue that `wrangler.jsonc` names, and writes their ids there.

**What it proves:** that the deployed database has the schema the deployed code was checked against. The migrations are the ones written from `db/schema.sql`, and they are applied before the code that needs them. It stops rather than guess when a bucket or a queue was renamed, since that would make a new, empty one.

**What it does not:** that the app works there. Run `test --deployed` after it. A deploy does not wait on tests or on health; it says how the Worker did in the day before.

## `pack`

```sh
go tool charter pack --prove
```

What others consume: SDKs written by Fern from the API's spec.

**What it proves:** that a client in each language, written from the same contract the Worker serves, calls the running Worker and gets what the types say. The spec comes from your Go types, so a client is typed all the way from your table: `db/schema.sql` gives the row, the row is what the API serves, the spec says so, and the SDK's type is that.

## `run`

```sh
go tool charter run            # lists the tasks
go tool charter run db:seed
go tool charter run health
```

Everything that is no verb: tasks for languages (`intl:`), search engines (`seo:`), the database (`db:`), components (`ui:`), people, secrets, and watching a deployed Worker.

**What they prove** is about the running system, which no type describes: [`health`](16-watching.md) says how requests ended on Cloudflare, `load` what happens under many readers, `cold` what a first reader waits. They take seconds or minutes, so none is part of `check`, `test` or `deploy`.

## The rest

| Command | For |
|---|---|
| `install` | everything the app pins, and gsx's language server for your editor |
| `fmt`, `lint` | the parts of `check`, alone |
| `build`, `preview` | what `dev` and `deploy` build, by hand |
| `exec` | one of the app's tools as it pins it: `charter exec wrangler …` |
| `update`, `outdated`, `upgrade` | the pins, and charter itself |
| `staged` | `check` for what is staged, as a pre-commit hook |
| `env` | what this machine has of what an app needs, and where `dev` is running |

`go tool charter <command> -h` says a command's flags.
