module github.com/joeblew999/charter/example

go 1.27.1

require (
	github.com/danielgtaylor/huma/v2 v2.41.0
	github.com/gsxhq/gsx v0.5.0
	github.com/jackielii/structpages v0.12.0
	github.com/jackielii/tailwind-merge-go v0.0.0-20260726184505-8a911ed0cf9a
	github.com/joeblew999/charter v0.44.1
	github.com/joeblew999/charter/example/sdk/go v0.0.0
	github.com/syumai/workers-go v0.36.0
)

require (
	github.com/BurntSushi/toml v1.6.0 // indirect
	github.com/a-h/parse v0.0.0-20250122154542-74294addb73e // indirect
	github.com/a-h/templ v0.3.1020 // indirect
	github.com/alecthomas/chroma/v2 v2.27.0 // indirect
	github.com/chromedp/cdproto v0.160.11 // indirect
	github.com/chromedp/chromedp v0.24.0 // indirect
	github.com/dlclark/regexp2/v2 v2.2.1 // indirect
	github.com/dustin/go-humanize v1.1.0 // indirect
	github.com/editorconfig/editorconfig-core-go/v2 v2.6.4 // indirect
	github.com/fsnotify/fsnotify v1.10.1 // indirect
	github.com/fxamacker/cbor/v2 v2.10.0 // indirect
	github.com/go-jose/go-jose/v4 v4.1.9 // indirect
	github.com/go-json-experiment/json v0.0.0-20260213210345-44df1a37e875 // indirect
	github.com/go-viper/mapstructure/v2 v2.5.0 // indirect
	github.com/go-webauthn/webauthn v0.18.2 // indirect
	github.com/go-webauthn/x v0.3.1 // indirect
	github.com/golang-jwt/jwt/v5 v5.3.1 // indirect
	github.com/google/go-tpm v0.9.8 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/gsxhq/gsxui v0.0.0-20261126121004-cef08eb4ec39 // indirect
	github.com/gsxhq/vite v0.3.2 // indirect
	github.com/jackielii/ctxkey v1.0.1 // indirect
	github.com/jackielii/structpages/tools/lint v0.1.21 // indirect
	github.com/mattn/go-isatty v0.0.24 // indirect
	github.com/mazznoer/csscolorparser v0.1.8 // indirect
	github.com/ncruces/go-strftime v1.1.0 // indirect
	github.com/philhofer/fwd v1.2.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/remyoudompheng/bigfft v0.0.0-20230129092748-24d4a6f8daec // indirect
	github.com/tailscale/hujson v0.0.0-20260727124030-b80ff77dac4f // indirect
	github.com/tdewolff/minify/v2 v2.24.13 // indirect
	github.com/tdewolff/parse/v2 v2.8.13 // indirect
	github.com/tinylib/msgp v1.6.5 // indirect
	github.com/x448/float16 v0.8.4 // indirect
	github.com/yuin/goldmark v1.8.7 // indirect
	github.com/yuin/goldmark-highlighting/v2 v2.0.0-20230729083705-37449abec8cc // indirect
	golang.org/x/crypto v0.59.0 // indirect
	golang.org/x/exp/typeparams v0.0.0-20260611194520-c48552f49976 // indirect
	golang.org/x/mod v0.43.0 // indirect
	golang.org/x/net v0.63.0 // indirect
	golang.org/x/sync v0.25.0 // indirect
	golang.org/x/sys v0.50.0 // indirect
	golang.org/x/term v0.48.0 // indirect
	golang.org/x/text v0.44.0 // indirect
	golang.org/x/tools v0.52.0 // indirect
	gopkg.in/ini.v1 v1.67.0 // indirect
	modernc.org/libc v1.80.1 // indirect
	modernc.org/mathutil v1.7.1 // indirect
	modernc.org/memory v1.12.1 // indirect
	modernc.org/sqlite v1.63.0 // indirect
)

replace github.com/joeblew999/charter => ../

tool (
	github.com/gsxhq/gsx/cmd/gsx
	github.com/gsxhq/gsxui/cmd/gsxui
	github.com/jackielii/structpages/tools/lint/cmd/structpages-lint
	github.com/joeblew999/charter/cmd/charter
)

replace github.com/joeblew999/charter/example/sdk/go => ./sdk/go
