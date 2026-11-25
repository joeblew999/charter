module github.com/joeblew999/charter/examples/workers

go 1.27.1

require github.com/syumai/workers-go v0.36.0

require (
	github.com/BurntSushi/toml v1.6.0 // indirect
	github.com/joeblew999/charter v0.27.1 // indirect
	github.com/tailscale/hujson v0.0.0-20260727124030-b80ff77dac4f // indirect
	golang.org/x/net v0.63.0 // indirect
)

replace github.com/joeblew999/charter => ../..

tool github.com/joeblew999/charter/cmd/charter
