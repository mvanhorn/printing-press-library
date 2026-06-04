# Changelog

## 0.1.16

- Add `install --bin-dir <dir>` and thread it through `update` / `reinstall` so Go binaries can be installed into a runtime-visible user bin directory such as `~/.local/bin`.
- Document OpenClaw installs as `--agent openclaw --bin-dir ~/.local/bin`, keeping skill placement and binary placement explicit.

## 0.1.15

- Add a version-lifecycle guard for npm releases: `preversion` now checks that `package-lock.json` is in sync before the bump, and `version` refreshes/checks the lockfile after `package.json` changes so future release commits cannot accidentally leave the lockfile version stale.
- Make the installer shadow-warning JSON test hermetic by explicitly stubbing `realpath` in the stale-binary case.

## 0.1.14

- Treat "installed but not on PATH" as a warning instead of aborting `install`. `go install` can succeed while the current agent or gateway process cannot discover `$GOPATH/bin`; the installer now prints the platform-specific PATH fix, continues installing the focused `pp-*` skill, and reports a successful install with a `pathWarning: "not_on_path"` field in JSON output. This avoids half-installed agent setups where the binary exists but the matching skill was skipped.
- Suppress stale-binary shadow warnings when the earlier PATH hit resolves to the same file as the freshly installed Go binary, such as a `~/.local/bin/<tool>` symlink pointing at `~/go/bin/<tool>`.
- Document that agent/gateway sessions may need a restart after PATH changes, and that a PATH-visible symlink can be a practical bridge in environments that already expose `~/.local/bin`.

## 0.1.13

- Speed up `update` (and `update` with no name) by refreshing detected CLIs concurrently instead of one at a time. The cost of a bulk update is dominated by per-CLI network round-trips — the go-proxy `@latest` resolution (~1s each, even when nothing changed, because the build cache can't shortcut it) plus the skill fetch — which serialized into ~30s for a dozen CLIs. These are independent, so they now run with bounded concurrency, and the catalog detection sweep (a `which`/`where` probe per catalog entry) is parallelized the same way. Each install's output is buffered and replayed in catalog order (preserving stdout/stderr ordering within each CLI), so concurrent runs don't interleave into scrambled lines. A failed PATH probe degrades to "not installed" instead of aborting the run. No command, flag, or output-shape changes — same behavior, less waiting.

## 0.1.12

- Add a `reinstall` command as an alias for `update`. `reinstall <name>` rebuilds one CLI's binary from the latest catalog code and re-adds its skill; `reinstall` with no name does the same for every Printing Press CLI already on `PATH`. The mechanics are identical to `update` (both run `go install …@latest` and re-add the skill) — this just exposes the verb users reach for when a binary or skill needs a clean refresh. The shared "no CLIs found on PATH" message is now verb-neutral so it reads correctly under either command.

## 0.1.11

- When `go install` writes a CLI to a directory that isn't on `PATH`, print the exact, copy-pasteable fix for the detected platform and shell instead of a single Unix-flavored hint. macOS zsh gets a `~/.zshrc` line, macOS bash gets `~/.bash_profile` (login shells don't read `.bashrc`), Linux bash gets `~/.bashrc`, fish gets `fish_add_path`, Windows gets the persistent PowerShell `[Environment]::SetEnvironmentVariable(... "User")` command plus a GUI fallback (and never the truncating `setx` footgun), and Git Bash gets a POSIX-translated path. The previous message printed `$(go env GOPATH)/bin` shell syntax that was wrong on Windows and imprecise on fish.

## 0.1.10

- Fix `install` / `uninstall` failing at the skill step on Windows with `ENOENT`. The process runner invoked `npx` through `node:child_process.execFile`, which does not resolve the `npx.cmd` shim Windows requires; it now uses `cross-spawn`, so the `npx skills …` install/remove step runs on Windows the same as on macOS and Linux. (#864)

## 0.1.9

- Catalog hints now match how the tool was invoked. The `list` and `search` usage lines and each entry's `install:` hint previously hardcoded the long `npx -y @mvanhorn/printing-press-library …` prefix; they now detect an `npx` run versus a globally-installed `printing-press-library` binary and print the shorter `printing-press-library …` form when that's how you're running it. (#749)

## 0.1.8

- Avoid treating one-character queries or the shared `-pp-cli` binary suffix as searchable content, so queries like `a`, `t`, `pp`, or `cli` no longer match broad slices of the catalog while full binary-name queries still resolve to the intended CLI.

## 0.1.7

- Refresh the GitHub and npm README surfaces now that `@mvanhorn/printing-press-library` is live.
- Document catalog discovery flows for `list`, `search`, category filtering, installed-only listing, and JSON output.
- Normalize search punctuation and simple plurals so queries like `cal.com`, `cal-com`, and `hotels` find the expected catalog entries.
- Add generated registry `search_terms` sourced from manifest descriptions, auth notes, and novel features so concise result descriptions can stay readable without weakening discovery.
- Document the optional global uninstall step for the older `@mvanhorn/printing-press` package.
- Show `npx -y @mvanhorn/printing-press-library install <name>` in discovery output so copy-paste installs work before the package is globally installed.

## 0.1.6

- Rename the npm package and command to `@mvanhorn/printing-press-library` / `printing-press-library` so the public catalog installer is unambiguous and does not collide with the Printing Press generator concept.
- Make `printing-press-library list` a public catalog discovery command by default, showing every available CLI with its category, binary name, and description. The previous installed-only view remains available as `printing-press-library list --installed`.
- Add `printing-press-library list --category <category>` for quick category browsing.

## 0.1.5

- Survive malformed upstream registry entries instead of aborting the whole parse. `parseRegistry` now skips entries that fail per-entry validation, writes a one-line warning to stderr naming the offending slug + field, and returns the rest of the catalog intact. Registry-level shape failures (wrong `schema_version`, non-array `entries`) still throw. This is the defense-in-depth pair for the library-side `--validate` gate (see `tools/generate-registry --validate` and `verify-library-conventions.yml`) so a single broken upstream entry — lawhub-shape — can never wedge every `install` / `search` / `list` / `update` call again.
- Detect and warn when an older binary earlier in `PATH` shadows the one `go install` just wrote. Previously `install` reported the first PATH hit as success, so a stale `/opt/homebrew/bin/<cli>` (for example) would mask a newer `~/go/bin/<cli>`. The installer now reads `go env GOBIN GOPATH`, compares the actual install path to what `which`/`where` returns, and emits a clear shadow warning when they differ. JSON output adds `installedPath` and `shadowedBy` fields. Fixes #470.

## 0.1.4

- Drop the `GOPRIVATE='github.com/mvanhorn/*' GOFLAGS=-mod=mod … @main` fallback from the `install` command. The library is fully public, so `go install …@latest` resolves through the public Go module proxy without any private-module configuration. The `@main` retry was only useful when paired with `GOPRIVATE` to bypass the proxy entirely; without it, `@main` issues an identical query subject to the same proxy cache and adds no value.

## 0.1.3

- Drop the `auth env vars: …` line from `install` output. The data was a bare list of env var names without the surrounding context (where to get the token, how to set it, what command verifies it) — that context lives in each CLI's `--help`, `doctor` command, and authenticated-error messages, which is the natural moment to discover auth requirements. JSON output no longer carries `authEnvVars` either; consumers that genuinely need a structured env-var list can read `mcp.env_vars` directly from `registry.json`.

## 0.1.2

- CI fix: pin the npm version used for Trusted Publishing to `npm@11.5.1`. The previous `npm install -g npm@latest` step is flaky on Actions runners — npm overwrites itself mid-install and the global install ends up with a missing `promise-retry` module. v0.1.1 was tagged but never reached npmjs.com because of this; this is the first published release on the OIDC pipeline.

## 0.1.1

- Rename binary from `pp` to `printing-press`. The previous two-letter name overlapped with our `pp-*` skill namespace, our `*-pp-cli` binary convention, and Perl's `pp` (PAR::Packer).
- Add bundles: `printing-press install starter-pack` installs `espn`, `flight-goat`, `movie-goat`, and `recipe-goat` together.
- Multi-name install: pass several names in one command, e.g. `printing-press install espn linear dub`. Bundle names and CLI names can mix freely.
- Add `--cli-only` and `--skill-only` flags so you can install just the Go binary (e.g. on a CI machine with no agent) or just the focused skill (relying on lazy binary install via the skill's prose). Mutually exclusive; both work with bundles.
- Switch the publish workflow to npm Trusted Publishing (OIDC). No long-lived `NPM_TOKEN` in repo secrets; releases mint short-lived tokens per workflow run and emit verifiable provenance attestations.
- Declare MIT license, repository, homepage, bugs URL, author/contributors, keywords, and `publishConfig` for npm discoverability.

## 0.1.0

- Initial scaffold for `@mvanhorn/printing-press`.
- Add `pp install`, `pp update`, `pp list`, `pp search`, and `pp uninstall`.
- Install per-CLI skills from `cli-skills/pp-<name>` via `skills@latest`.
