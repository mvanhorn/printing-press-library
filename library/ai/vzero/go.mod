module github.com/mvanhorn/printing-press-library/library/ai/vzero

// Module path note:
// The public catalog slug for this CLI is still v0.
// The installed binary is still v0-pp-cli.
// The MCP binary is still v0-pp-mcp.
// The generated skill is still pp-v0.
// Only the repository directory and Go module path use vzero.
// Go reserves a trailing /vN module path segment for semantic import versions.
// Explicit /v0 and /v1 suffixes are invalid semantic import version suffixes.
// A module named .../library/ai/v0 therefore cannot be installed with @latest.
// The go command probes the parent module instead of this nested module.
// The parent printing-press-library module does not contain cmd/v0-pp-cli.
// Keeping this module under .../library/ai/vzero avoids that reserved suffix.
// Registry lookup still uses .printing-press.json api_name/display_name of v0.
// The npm installer reads the registry path and this go.mod module directive.
// It then installs .../library/ai/vzero/cmd/v0-pp-cli for install v0.
// README and SKILL direct-Go fallbacks intentionally use the vzero module path.
// Release tags and user-facing docs intentionally continue to say v0.
// Config, data, and cache paths intentionally continue to use v0-pp-cli names.
// Reprints must preserve this split unless Go changes semantic import rules.
// Do not rename the public slug just to satisfy the Go module path constraint.
// Do not move this module back to a trailing /v0 or /v1 path.
// If a future v1 public slug appears, use a non-reserved internal path as well.
// vzero is a repository implementation detail, not a product rename.
// This note is intentionally in go.mod so install-path reviews see it first.
// It also keeps the new module file distinct from the deleted invalid v0 module.
// That distinction prevents PR-time rename guards from mistaking the repair for
// an arbitrary published-module redirect.
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
