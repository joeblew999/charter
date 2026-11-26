# The API's clients, each against a running Worker

`go tool charter pack -prove` generates every client `charter.toml` names, proves each builds in its own language,
then starts the Worker and runs the program here that is in that client's language. Each asks the same three things,
which is who may do what, as the contract says it:

1. Nobody may write: a token nobody issued is refused (401). A generated client sends nothing without one.
2. Someone who may only read may not write (403), and may read.
3. Someone who may write writes, and what they wrote is read back.

A program gets `BASE` (where the Worker is), `READ_TOKEN` and `WRITE_TOKEN` (the Worker's own, from `.dev.vars`).
The Go client's turn is `test/api_test.go` and `test/page_signin_test.go`, which every `charter test` runs: there it also
holds roles to one rule on the page and through the API.
