module github.com/joeblew999/charter/examples/conformance-go

go 1.27.1

require (
	github.com/coder/websocket v1.8.15
	github.com/danielgtaylor/huma/v2 v2.41.0
	github.com/joeblew999/charter v0.0.0
	github.com/syumai/workers-go v0.36.0
)

require (
	github.com/BurntSushi/toml v1.6.0 // indirect
	github.com/dustin/go-humanize v1.1.0 // indirect
	github.com/evanw/esbuild v0.28.2 // indirect
	github.com/go-jose/go-jose/v4 v4.1.7 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/mattn/go-isatty v0.0.24 // indirect
	github.com/ncruces/go-strftime v1.1.0 // indirect
	github.com/remyoudompheng/bigfft v0.0.0-20230129092748-24d4a6f8daec // indirect
	golang.org/x/mod v0.43.0 // indirect
	golang.org/x/sys v0.50.0 // indirect
	modernc.org/libc v1.80.1 // indirect
	modernc.org/mathutil v1.7.1 // indirect
	modernc.org/memory v1.12.1 // indirect
	modernc.org/sqlite v1.63.0 // indirect
)

replace github.com/joeblew999/charter => ../..

tool github.com/joeblew999/charter/cmd/charter
