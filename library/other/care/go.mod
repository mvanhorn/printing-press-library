module github.com/mvanhorn/printing-press-library/library/other/care

go 1.26.5

require (
	github.com/enetx/surf v1.0.199
	github.com/gorilla/websocket v1.5.3
	github.com/pelletier/go-toml/v2 v2.2.4
	github.com/spf13/cobra v1.9.1
	github.com/spf13/pflag v1.0.6
)

require (
	github.com/chromedp/cdproto v0.0.0-20260714215040-dc233986426f
	github.com/chromedp/chromedp v0.16.0
	github.com/enetx/http v1.0.28
	github.com/mark3labs/mcp-go v0.47.0
	modernc.org/sqlite v1.33.1
)

// x/sys is a DIRECT dependency for token-bearing bundles: the read-time
// credentials-perms guard's Windows surface (internal/cliutil/creds_perms_windows.go)
// imports golang.org/x/sys/windows. Emitted as a direct require (no // indirect)
// so a freshly generated bundle's go.mod is correct out of the box, WITHOUT a
// manual `go mod tidy`. The version matches the transitive floor below so a
// single x/sys version is pinned. NOTE (go mod tidy GOOS caveat): the import is
// behind `//go:build windows`, so running `go mod tidy` under GOOS=linux/darwin
// re-marks this // indirect (that GOOS compiles no file that imports it); under
// GOOS=windows it stays direct. That is tolerated churn, NOT a bug to "fix".
require golang.org/x/sys v0.47.0

// Floor the HTTP/3 transitive deps pulled in only via github.com/enetx/surf
// above their vulnerable versions (osv flags module presence; govulncheck
// reachability = 0 for these REST CLIs). Emitted only when the surf transport
// is present, so MVS keeps the floor; tidy drops it for CLIs without surf.
require golang.org/x/crypto v0.53.0 // indirect

require (
	github.com/andybalholm/brotli v1.2.1 // indirect
	github.com/chromedp/sysutil v1.1.0 // indirect
	github.com/dustin/go-humanize v1.0.1 // indirect
	github.com/enetx/g v1.0.224 // indirect
	github.com/enetx/http2 v1.0.26 // indirect
	github.com/enetx/http3 v1.0.7 // indirect
	github.com/enetx/iter v0.0.0-20250912135656-f1583323588f // indirect
	github.com/go-json-experiment/json v0.0.0-20260623181947-01eb4420fa68 // indirect
	github.com/gobwas/httphead v0.1.0 // indirect
	github.com/gobwas/pool v0.2.1 // indirect
	github.com/gobwas/ws v1.4.0 // indirect
	github.com/google/jsonschema-go v0.4.2 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/hashicorp/golang-lru/v2 v2.0.7 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/klauspost/compress v1.18.5 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/ncruces/go-strftime v1.0.0 // indirect
	github.com/quic-go/qpack v0.6.0 // indirect
	github.com/quic-go/quic-go v0.60.0 // indirect
	github.com/refraction-networking/utls v1.8.3-0.20260301010127-aa6edf4b11af // indirect
	github.com/remyoudompheng/bigfft v0.0.0-20230129092748-24d4a6f8daec // indirect
	github.com/spf13/cast v1.7.1 // indirect
	github.com/wzshiming/socks5 v0.7.0 // indirect
	github.com/yosida95/uritemplate/v3 v3.0.2 // indirect
	golang.org/x/net v0.55.0 // indirect
	golang.org/x/text v0.40.0 // indirect
	modernc.org/gc/v3 v3.1.4 // indirect
	modernc.org/libc v1.74.1 // indirect
	modernc.org/mathutil v1.7.1 // indirect
	modernc.org/memory v1.11.0 // indirect
	modernc.org/strutil v1.2.1 // indirect
	modernc.org/token v1.1.0 // indirect
)
