Bring the documentation in `docs/` of this repository up to date and into line with its own rules. You are editing documentation only.

## Read first

1. `AGENTS.md`, then `docs/README.md` (the start page) and `docs/rules.md`.
2. `docs/writing.md`: the rules every page is held to. They are the standard for this job.

## What a program already found

`mise run docs:lint` reports:

```
__LINT__
```

Fix every one of these. Fix the page, not the checker.

## Then, page by page

For every page under `docs/` (leave `_config.yml`, `_sass/` and `writing.md` alone: they are generated):

1. **Check each statement against the repo as it is now.** For a command, look in `mise.toml` (`mise tasks`). For a path, look for the file. For how something works, read the code it names. For a number or a result, find it in `docs/findings.md` or the benchmarks page. If a statement is wrong, correct it from what the code says. If it can't be confirmed, cut it or say plainly that it is unverified.
2. **Find what is missing.** Look at what the part actually has (its files, its tasks in `mise.toml`, its flags, its limits) and add what a reader of that page needs and can't currently find there.
3. **Make the page do its one job** as `docs/writing.md` defines it for that kind of page: restructure, cut history and repetition, move facts to the page that owns them and link.
4. **Keep the start page's index and "What is what" table complete and correct:** every page has a row, every name in the table is the name used everywhere.

## Limits

- **Don't change code, tasks, specs or generated files.** If the docs can only be made true by changing code, leave the code and say so in your summary.
- **Don't invent.** No result, number or behaviour that you did not find in the repo. `docs/findings.md` holds verified results only: fix its links and paths, but do not add results, and do not change what an entry says happened.
- **Don't touch `docs/plans/` beyond links, paths and anything that is now built** (move that to the page of the part it belongs to, and remove it from the plan).
- **Keep every page's front matter**, and give any page you add its own, plus a row in the start page.

## Finish

1. Run `mise run docs:lint` until it reports nothing.
2. Run `mise run dev:check` if the repo has that task.
3. Report, page by page: what you corrected, what you added, what you cut, and anything you could not verify.
