module github.com/joeblew999/charter/examples/api

go 1.27.1

require (
	github.com/danielgtaylor/huma/v2 v2.41.0
	github.com/joeblew999/charter v0.24.0
)

require (
	github.com/BurntSushi/toml v1.6.0 // indirect
	github.com/coder/websocket v1.8.15 // indirect
	github.com/go-jose/go-jose/v4 v4.1.7 // indirect
	github.com/syumai/workers-go v0.36.0 // indirect
	github.com/tailscale/hujson v0.0.0-20260727124030-b80ff77dac4f // indirect
	golang.org/x/net v0.63.0 // indirect
)

replace github.com/joeblew999/charter => ../..

tool github.com/joeblew999/charter/cmd/charter
