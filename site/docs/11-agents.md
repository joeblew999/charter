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

## What a language model reads

Any agent, with or without the plugins, can read these docs as Markdown:

| Address | What it is |
|---|---|
| [`/llms.txt`](../llms.txt) | charter in a few lines, the plugins to work with, and every page as a link to its Markdown |
| [`/llms-full.txt`](../llms-full.txt) | Every page, as one document |
| `/docs/<page>.md` | One page |

The skill is short on purpose: the rules, and which command follows which change. For how something works it points here. So the two never say different things: the docs are written once, as the Markdown of [`site/docs/`](https://github.com/joeblew999/charter/tree/main/site/docs).

[Your own app's docs](10-docs.md) get the same three addresses.
