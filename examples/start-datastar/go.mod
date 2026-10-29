module github.com/joeblew999/charter/examples/start-datastar

go 1.27.1

require (
	github.com/danielgtaylor/huma/v2 v2.40.0
	github.com/gsxhq/gsx v0.4.1
	github.com/joeblew999/charter v0.0.0
	github.com/syumai/workers-go v0.36.0
)

require (
	github.com/BurntSushi/toml v1.6.0 // indirect
	github.com/coder/websocket v1.8.15 // indirect
	github.com/dustin/go-humanize v1.1.0 // indirect
	github.com/editorconfig/editorconfig-core-go/v2 v2.6.4 // indirect
	github.com/fsnotify/fsnotify v1.10.1 // indirect
	github.com/go-jose/go-jose/v4 v4.1.5 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/mattn/go-isatty v0.0.24 // indirect
	github.com/ncruces/go-strftime v1.1.0 // indirect
	github.com/remyoudompheng/bigfft v0.0.0-20230129092748-24d4a6f8daec // indirect
	github.com/tdewolff/minify/v2 v2.24.13 // indirect
	github.com/tdewolff/parse/v2 v2.8.13 // indirect
	golang.org/x/exp/typeparams v0.0.0-20260611194520-c48552f49976 // indirect
	golang.org/x/mod v0.42.0 // indirect
	golang.org/x/net v0.61.0 // indirect
	golang.org/x/sync v0.24.0 // indirect
	golang.org/x/sys v0.49.0 // indirect
	golang.org/x/tools v0.52.0 // indirect
	gopkg.in/ini.v1 v1.67.0 // indirect
	modernc.org/libc v1.79.0 // indirect
	modernc.org/mathutil v1.7.1 // indirect
	modernc.org/memory v1.12.1 // indirect
	modernc.org/sqlite v1.61.0 // indirect
)

replace github.com/joeblew999/charter => ../..

tool (
	github.com/gsxhq/gsx/cmd/gsx
	github.com/joeblew999/charter/cmd/charter
)
