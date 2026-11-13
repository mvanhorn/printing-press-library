module github.com/mvanhorn/printing-press-library/library/other/care

go 1.26.11

require (
	github.com/gorilla/websocket v1.5.3
	github.com/pelletier/go-toml/v2 v2.2.4
	github.com/refraction-networking/utls v1.8.2
	github.com/spf13/cobra v1.9.1
	github.com/spf13/pflag v1.0.6
	golang.org/x/net v0.60.0
)

require (
	github.com/chromedp/cdproto v0.0.0-20260714215040-dc233986426f
	github.com/chromedp/chromedp v0.16.0
	github.com/mark3labs/mcp-go v0.57.0
	modernc.org/sqlite v1.61.0
)

require (
	github.com/andybalholm/brotli v1.2.7 // indirect
	github.com/chromedp/sysutil v1.1.0 // indirect
	github.com/dustin/go-humanize v1.1.0 // indirect
	github.com/go-json-experiment/json v0.0.0-20260623181947-01eb4420fa68 // indirect
	github.com/gobwas/httphead v0.1.0 // indirect
	github.com/gobwas/pool v0.2.1 // indirect
	github.com/gobwas/ws v1.4.0 // indirect
	github.com/google/jsonschema-go v0.4.2 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/klauspost/compress v1.20.2 // indirect
	github.com/mattn/go-isatty v0.0.24 // indirect
	github.com/ncruces/go-strftime v1.1.0 // indirect
	github.com/remyoudompheng/bigfft v0.0.0-20230129092748-24d4a6f8daec // indirect
	github.com/santhosh-tekuri/jsonschema/v6 v6.0.2 // indirect
	github.com/spf13/cast v1.7.1 // indirect
	github.com/yosida95/uritemplate/v3 v3.0.2 // indirect
	golang.org/x/text v0.42.0 // indirect
	modernc.org/libc v1.79.0 // indirect
	modernc.org/mathutil v1.7.1 // indirect
	modernc.org/memory v1.12.1 // indirect
)

// x/sys is a DIRECT dependency for token-bearing bundles. The read-time
// credentials guard imports golang.org/x/sys/unix on non-Windows
// (creds_perms_unix.go; darwin's syscall package has no Openat) and
// golang.org/x/sys/windows on Windows. darwin_acl_darwin.go also imports
// golang.org/x/sys/unix. Emitted as a direct require (no // indirect) so a
// freshly generated bundle's go.mod is correct out of the box, WITHOUT a
// manual `go mod tidy`. The version matches the transitive floor below so a
// single x/sys version is pinned. `go mod tidy` on linux/darwin keeps the
// require direct because a unix import is compiled there; tidy on Windows
// keeps it direct because of the windows import.
require golang.org/x/sys v0.49.0

// Floor x/crypto, pulled in only transitively via the Chrome TLS stack, above
// its vulnerable versions (osv flags module presence; govulncheck reachability
// = 0 for these REST CLIs). Emitted only when the Chrome transport is present,
// so MVS keeps the floor; tidy drops it for CLIs without it.
require golang.org/x/crypto v0.57.0 // indirect
