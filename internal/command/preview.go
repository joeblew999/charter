package command

import ()

// Preview runs charter preview: the Worker as Cloudflare runs it, on wrangler dev, with the local D1's migrations
// applied first. More arguments are wrangler dev's.
func Preview(args []string) {
	p := load()
	check(migrate(p, nil, "--local"))
	bin, err := npm(p, "wrangler")
	check(err)
	check(passthrough(p, bin, append([]string{"dev"}, args...)...))
}
