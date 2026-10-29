module github.com/joeblew999/charter/examples/notes-go

go 1.27.1

require (
	github.com/danielgtaylor/huma/v2 v2.40.0
	github.com/joeblew999/charter v0.0.0
	github.com/syumai/workers-go v0.36.0
)

require (
	github.com/BurntSushi/toml v1.6.0 // indirect
	github.com/coder/websocket v1.8.15 // indirect
	github.com/dustin/go-humanize v1.1.0 // indirect
	github.com/go-jose/go-jose/v4 v4.1.5 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/mattn/go-isatty v0.0.24 // indirect
	github.com/ncruces/go-strftime v1.1.0 // indirect
	github.com/remyoudompheng/bigfft v0.0.0-20230129092748-24d4a6f8daec // indirect
	golang.org/x/sys v0.49.0 // indirect
	modernc.org/libc v1.79.0 // indirect
	modernc.org/mathutil v1.7.1 // indirect
	modernc.org/memory v1.12.1 // indirect
	modernc.org/sqlite v1.61.0 // indirect
)

replace github.com/joeblew999/charter => ../..

tool github.com/joeblew999/charter/cmd/charter
