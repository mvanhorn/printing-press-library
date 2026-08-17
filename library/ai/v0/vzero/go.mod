module github.com/mvanhorn/printing-press-library/library/ai/v0/vzero

// Module path note:
// The public catalog slug, skill, binary, config paths, and release tags remain
// v0. Go reserves a trailing /vN module path segment for semantic import
// versions, and explicit /v0 or /v1 suffixes are invalid, so install paths use
// this nested vzero module instead of ending at .../library/ai/v0.
//
// Invariants for future reprints and hand repairs:
// - keep .printing-press.json in the parent v0 catalog directory;
// - keep README/SKILL public installer examples on `install v0`;
// - keep direct Go fallback examples pointed at this vzero module;
// - keep npm install mapping from the v0 registry entry to this module;
// - keep cmd/v0-pp-cli and cmd/v0-pp-mcp binary names unchanged;
// - keep local config/data/cache names tied to v0-pp-cli;
// - keep release tags tied to v0-current;
// - do not introduce a second catalog entry named vzero;
// - do not move this module back to the parent v0 directory;
// - if Go semantic import versioning rules change, re-evaluate this split;
// - if a future public v1 CLI appears, avoid a trailing /v1 install module too;
// - if registry generation gains an explicit module override field, prefer that
//   over npm-side special casing and keep this note in sync;
// - if this nested module is regenerated, update imports to this full module
//   path before running gofmt, go vet, and go test;
// - if direct installs fail, first verify `go list ./cmd/v0-pp-cli` from this
//   directory prints the full vzero import path below.
//
go 1.26.6

require (
	github.com/mark3labs/mcp-go v0.47.0
	github.com/pelletier/go-toml/v2 v2.2.4
	github.com/spf13/cobra v1.9.1
	github.com/spf13/pflag v1.0.6
	golang.org/x/sys v0.46.0
	modernc.org/sqlite v1.37.0
)

require (
	github.com/dustin/go-humanize v1.0.1 // indirect
	github.com/google/jsonschema-go v0.4.2 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/ncruces/go-strftime v0.1.9 // indirect
	github.com/remyoudompheng/bigfft v0.0.0-20230129092748-24d4a6f8daec // indirect
	github.com/spf13/cast v1.7.1 // indirect
	github.com/yosida95/uritemplate/v3 v3.0.2 // indirect
	golang.org/x/exp v0.0.0-20250305212735-054e65f0b394 // indirect
	modernc.org/libc v1.62.1 // indirect
	modernc.org/mathutil v1.7.1 // indirect
	modernc.org/memory v1.9.1 // indirect
)
