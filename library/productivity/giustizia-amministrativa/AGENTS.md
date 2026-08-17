# Giustizia Amministrativa Printed CLI Agent Guide

This directory is a generated `giustizia-amministrativa-pp-cli` printed CLI. It was produced by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press), so treat systemic fixes as upstream Printing Press fixes first. Keep local edits narrow and document why a generated-tree patch belongs here.

## Local Operating Contract

Start by asking the generated CLI for current runtime truth:

```bash
giustizia-amministrativa-pp-cli doctor --json
giustizia-amministrativa-pp-cli agent-context --pretty
```

Use runtime discovery instead of relying on a copied command list:

```bash
giustizia-amministrativa-pp-cli which "<capability>" --json
giustizia-amministrativa-pp-cli <command> --help
```

Add `--agent` to command invocations for JSON, compact output, non-interactive defaults, no color, and confirmation-safe scripting:

```bash
giustizia-amministrativa-pp-cli <command> --agent
```

Before running an unfamiliar command that may mutate remote state, inspect its help and prefer a dry run:

```bash
giustizia-amministrativa-pp-cli <command> --help
giustizia-amministrativa-pp-cli <command> --dry-run --agent
```

Use `--yes --no-input` only after the target, arguments, and side effects are clear.

For install, auth, examples, and longer product guidance, read `README.md` and `SKILL.md`. This file intentionally stays small so repo-local agents get invariant local guidance without duplicating the generated docs.

## Testing the MCP Surface

Never point a test at an install path. On 2026-08-08 a 21-scenario MCP evaluation ran against `~/go/bin`, seven weeks behind HEAD, and reported three blocking defects that had already been fixed; see `LOG.md`. Ask any binary which commit it came from before trusting a verdict about it — Go stamps this at build time:

```bash
go version -m ~/go/bin/giustizia-amministrativa-pp-mcp | grep -E 'vcs.(revision|modified)'
```

`vcs.modified=true` means it was built from a dirty tree, so the revision alone does not identify it. Build first, then test what you built:

```bash
make build build-mcp        # both binaries into bin/
python3 tmp/mcp_harness.py  # initialize + tools/list against ./bin (override with GA_MCP_BIN)
```

The typed MCP tools shell out to the CLI via `SiblingCLIPath()`, so **both binaries must be current and side by side**. A new MCP next to an old CLI does not fail — it answers worse. The same applies to `make install`/`make install-mcp`: run them together, never one alone.

`.mcp.json` registers this server for Claude Code and deliberately points at `bin/`, not at `~/go/bin`, so `make build-mcp` changes what the host runs. `bin/` is gitignored: after a fresh clone the server fails to start until you build, which is the intended loud failure.

## Release Ledger

`CHANGELOG.md` and `.printing-press-release.json` are the public library's per-CLI release ledger. Fresh prints may carry blank skeletons, but the final `YYYY.M.N` CLI release version is assigned only after a publish PR merges in `mvanhorn/printing-press-library`. Do not hand-bump those files or edit `var version = ...` for release bookkeeping; preserve existing ledger files on reprint and let the library workflow stamp the next release.

## Local Customizations

This directory is **generated output** -- a fresh print can overwrite the whole tree, so ad-hoc hand-edits don't survive on their own. If you modify the generated code, record each change under `.printing-press-patches/` (parallel to `.printing-press.json`) so a regen carries the intent forward instead of silently dropping it.

The entry shape, and the altitude to write it at -- a durable reprint-guard, not a changelog -- live in the source catalog's `AGENTS.md`, which is the single source of truth; this guide intentionally doesn't duplicate them.

### `dogfood --research-dir` overwrites `internal/mcp/tools.go`

Run `cli-printing-press dogfood --dir .` without `--research-dir`, or check `git diff internal/mcp/tools.go` immediately afterwards.

With `--research-dir` pointing at `.manuscripts/<run>/`, dogfood re-synchronises the `command_mirror_capabilities` block in `handleContext` from `research.json` -- the June generation record -- and silently reverts three later fixes: the `tool` (MCP name, `watch_run`) plus `cli_command` (shell spelling, `watch run`) key pair collapses back to a single `command` key, `get` loses "Passa l'ECLI in `id`", and `stats` loses the `sede-sweep` clause. That key pair *is* commit `6578e7a`: an agent reading `watch run` cannot call it, because the tool is named `watch_run`.

Editing `research.json` is not the fix. It is archived run evidence, and its schema has one command field where the mirror needs two, so a sync would flatten the pair whatever the text says. Observed on 2026-08-08 with press 4.30.1.
