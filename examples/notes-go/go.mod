module github.com/joeblew999/charter/examples/notes-go

go 1.27.1

require (
	github.com/danielgtaylor/huma/v2 v2.39.1
	github.com/joeblew999/charter/go v0.0.0
	github.com/syumai/workers-go v0.36.0
)

require github.com/coder/websocket v1.8.15 // indirect

replace github.com/joeblew999/charter/go => ../../go
