module github.com/mvanhorn/printing-press-library/library/productivity/expensify

go 1.26.5

toolchain go1.26.5

require (
	github.com/enetx/surf v1.0.199
	github.com/pelletier/go-toml/v2 v2.2.4
	github.com/spf13/cobra v1.9.1
)

require (
	github.com/enetx/http v1.0.28
	github.com/mark3labs/mcp-go v0.47.0
	github.com/spf13/pflag v1.0.6
	modernc.org/sqlite v1.53.0
)

// Floor x/sys above the vulnerable v0.31.0. It is pulled only transitively
// (modernc.org/sqlite, golang.org/x/net, ...), so MVS needs this explicit
// floor; tidy drops it for CLIs that pull no x/sys at all.
require golang.org/x/sys v0.46.0 // indirect

// Floor the HTTP/3 transitive deps pulled in only via github.com/enetx/surf
// above their vulnerable versions (osv flags module presence; govulncheck
// reachability = 0 for these REST CLIs). Emitted only when the surf transport
// is present, so MVS keeps the floor; tidy drops it for CLIs without surf.
require golang.org/x/crypto v0.53.0 // indirect

require (
	github.com/andybalholm/brotli v1.2.1 // indirect
	github.com/dustin/go-humanize v1.0.1 // indirect
	github.com/enetx/g v1.0.224 // indirect
	github.com/enetx/http2 v1.0.26 // indirect
	github.com/enetx/http3 v1.0.7 // indirect
	github.com/enetx/iter v0.0.0-20250912135656-f1583323588f // indirect
	github.com/google/jsonschema-go v0.4.2 // indirect
	github.com/google/uuid v1.6.0 // indirect
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
	golang.org/x/text v0.38.0 // indirect
	modernc.org/libc v1.73.4 // indirect
	modernc.org/mathutil v1.7.1 // indirect
	modernc.org/memory v1.11.0 // indirect
)
