# charter example: workers

Everything [workers-go](https://github.com/syumai/workers-go) has, used the workers-go way: its own packages, called directly, with no charter in the code. charter builds it, keeps its Go runtime between requests, and tests each binding on wrangler dev.

```sh
go tool charter dev      # leave it running
go tool charter check && go tool charter test
```
