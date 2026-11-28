# For your coding agent

charter is made to be worked on by a coding agent as much as by a person. An agent gets two things: skills, which are the rules to work by, and these docs as Markdown, which are the reference.

## The plugins

charter's repository is a [Claude Code plugin marketplace](https://code.claude.com/docs/en/plugin-marketplaces). It lists four plugins, each a skill kept by the owner of the tool it is about:

| Plugin | What it teaches | Kept in |
|---|---|---|
| `charter` | Where everything in a charter app goes, and which command follows which change | [joeblew999/charter](https://github.com/joeblew999/charter/tree/main/skills) |
| `gsx` | Writing gsx components | [gsxhq/gsx](https://github.com/gsxhq/gsx) |
| `gsxui` | Building the interface from gsxui's components | [gsxhq/gsxui](https://github.com/gsxhq/gsxui) |
| `structpages` | The page tree: routes, Props, regions, links | [jackielii/structpages](https://github.com/jackielii/structpages) |

## In an app made with charter

There is nothing to do. The app's `.claude/settings.json` names the marketplace and enables the four plugins, so Claude Code offers them to everyone who opens the repository, and keeps them current as their owners change them.

```sh
go tool charter install   # installs them too
```

## Anywhere else

In Claude Code:

```
/plugin marketplace add joeblew999/charter
/plugin install charter@charter
/plugin install gsx@charter
/plugin install gsxui@charter
/plugin install structpages@charter
```

## One set of commands

The skills of gsx, gsxui and structpages are written for any app. A charter app runs those tools through charter, so charter's skill says what each of their instructions is here:

| Their skill says | In a charter app |
|---|---|
| `gsx generate`, `gsx dev`, `gsx fmt` | Nothing to run: `charter dev` is gsx dev, and `check --fix` generates and formats |
| `gsxui add`, `gsxui list`, `gsxui apply` | `charter add ui:<name>`, `charter run ui:list`, `charter run ui:theme` |
| Run `structpages-lint` | `charter check` runs it |
| Mount the tree with an error handler and a target selector | `app.App` does |

Their tools are the app's own Go tools too (`go tool gsx info`), at the versions `go.mod` pins.

## What an agent can ask

- `go tool charter env`: is `charter dev` running here, at which address, and where is its log.
- `go tool charter check --json`: whether the code is right, as one JSON document: each part (formatting, lint, the contract, the languages) with what it said. No command asks a question.
- The page itself, at the address `env` gives: a save is on it in about two seconds.

## What a language model reads

Any agent, with or without the plugins, can read these docs as Markdown:

| Address | What it is |
|---|---|
| [`/llms.txt`](../llms.txt) | charter in a few lines, the plugins to work with, and every page as a link to its Markdown |
| [`/llms-full.txt`](../llms-full.txt) | Every page, as one document |
| `/docs/<page>.md` | One page |

The skill is short on purpose: the rules, and which command follows which change. For how something works it points here. So the two never say different things: the docs are written once, as the Markdown of [`site/docs/`](https://github.com/joeblew999/charter/tree/main/site/docs).

[Your own app's docs](11-docs.md) get the same three addresses.
