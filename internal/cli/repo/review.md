Bring the documentation in `docs/` of this repository up to date and into line with its own rules. You are editing documentation only.

## Read first

1. `AGENTS.md`, then `docs/self/README.md` (the home page) and `docs/self/rules.md`.
2. `docs/charter/writing.md`: the rules every page is held to. They are the standard for this job.

## What a program already found

`charter docs` reports:

```
__LINT__
```

Fix every one of these. Fix the page, not the checker.

## Then, page by page

For every page under `docs/` (leave `_config.yml`, `llms.txt`, `_sass/` and `charter/` alone: charter writes them; the repo's own pages are in `self/`):

1. **Check each statement against the repo as it is now.** For a command, `charter help` lists charter's, and the repo's own are its mise tasks and package.json scripts. For a path, look for the file. For how something works, read the code it names. For a number or a result, find it on the benchmarks page. If a statement is wrong, correct it from what the code says. If it can't be confirmed, cut it or say plainly that it is unverified.
2. **Find what is missing.** Look at what the part actually has (its files, its commands, its flags, its limits) and add what a reader of that page needs and can't currently find there.
3. **Make the page do its one job** as `docs/charter/writing.md` defines it for that kind of page: restructure, cut history and repetition, move facts to the page that owns them and link.
4. **Keep the home page's "Every page" table complete and correct:** every page has a row, every name in it is the name used everywhere.

## Limits

- **Don't change code, scripts, specs or generated files.** If the docs can only be made true by changing code, leave the code and say so in your summary.
- **Don't invent.** No result, number or behaviour that you did not find in the repo.
- **Keep it short.** Cut what a reader does not act on: history, plans (those are GitHub issues), background. A page over about 500 words is a sign to cut.
- **Keep every page's front matter**, and give any page you add its own, plus a row in the home page's table.

## Finish

1. Run `charter docs` until it reports nothing.
2. Report, page by page: what you corrected, what you added, what you cut, and anything you could not verify.
