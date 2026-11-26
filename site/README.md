# charter's site

A front page and the docs, as a charter app. Deployed at https://charter.gedw99.workers.dev.

The docs are the Markdown in [`docs/`](docs): edit a page there. The pages that show them are the library's
(`charter/app/docs`), the same ones the example's docs and every app's docs get.

`npm install` at the repo's top, then here:

```sh
go tool charter dev                              # leave it running: a saved Markdown file reloads the page
go tool charter check && go tool charter test    # before a push
go tool charter deploy                           # then: go tool charter test --deployed
```
