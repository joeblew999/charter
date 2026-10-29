module github.com/joeblew999/charter/examples/start-go

go 1.27.1

require (
	github.com/danielgtaylor/huma/v2 v2.40.0
	github.com/joeblew999/charter v0.0.0
	github.com/syumai/workers-go v0.36.0
)

require (
	github.com/BurntSushi/toml v1.6.0 // indirect
	github.com/coder/websocket v1.8.15 // indirect
	github.com/go-jose/go-jose/v4 v4.1.5 // indirect
)

replace github.com/joeblew999/charter => ../..

tool github.com/joeblew999/charter/cmd/charter
