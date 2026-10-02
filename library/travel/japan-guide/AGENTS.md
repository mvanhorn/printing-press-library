# Japan Guide Printed CLI Agent Guide

This directory is a generated `japan-guide-pp-cli` printed CLI. It was produced by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press), so treat systemic fixes as upstream Printing Press fixes first. Keep local edits narrow and document why a generated-tree patch belongs here.

## Local Operating Contract

Start by asking the generated CLI for current runtime truth:

```bash
japan-guide-pp-cli doctor --json
japan-guide-pp-cli agent-context --pretty
```

Use runtime discovery instead of relying on a copied command list:

```bash
japan-guide-pp-cli which "<capability>" --json
japan-guide-pp-cli <command> --help
```

Add `--agent` to command invocations for JSON, compact output, non-interactive defaults, and no color:

```bash
japan-guide-pp-cli <command> --agent
```

Before running an unfamiliar command that may mutate remote state, inspect its help and prefer a dry run:

```bash
japan-guide-pp-cli <command> --help
japan-guide-pp-cli <command> --dry-run --agent
```

When a command requires confirmation, pass `--yes` explicitly only after the target, arguments, and side effects are clear. `--agent` does not imply `--yes`.

## Novel Command Data Sources

Every hand-written novel command must declare its strategy in a Go line comment:

```go
// pp:data-source auto
```

Use exactly one of `auto`, `local`, `live`, or `computed`. Keep `auto` when the command honors `--data-source auto|local|live` by preferring live data with a local fallback; use `local` for local-only reads, `live` for remote-only reads, and `computed` for pure computation from embedded rules. Change a generated scaffold's `auto` default deliberately when its implementation has a narrower source, and keep `cmd.Annotations["pp:data-source"]` on the same command in sync; `--agent` envelopes read that annotation for `meta.source`. Reject incompatible `--data-source` requests with a clear error. TODO stubs still fail dogfood even when annotated.

## Self-Learning Loop

This CLI ships a self-capturing teach/recall loop backed by the local SQLite store. The CLI journals every invocation, derives `flag_alias` candidates from failed-flag + corrected-retry pairs, and synthesizes a playbook candidate when a family is taught without one - no manual failure bookkeeping. The agent's role is judgment:

1. On a new user question, call `japan-guide-pp-cli recall` FIRST, passing the question as an argv or MCP argument - never by interpolating it into a shell command, quoted string, or heredoc (apostrophes break quotes; a body line equal to the delimiter breaks a heredoc). If a shell is required, write the question with a non-shell file tool, then `QUERY=$(cat /path/to/question.txt)` and `japan-guide-pp-cli recall "$QUERY" --agent`. If `found=true` and the top result has `entity_match == "exact"` and `confidence >= 2`, skip discovery and go straight to the live fetch for the returned resource IDs. If the store is cold (recall finds nothing and `learnings list` and `learnings candidates` are both empty), skip recall for the rest of the session.
2. When the envelope carries a `candidates` section (warning `candidates_present`), candidates are try-then-confirm, never facts: follow each candidate's two-step `next_action` verbatim (trial command first, then `learnings confirm <id>` only after the trial verified the behavior), and reject wrong ones with `learnings reject <id>`. Never re-teach something recall surfaced as a candidate; confirm or reject it instead.
3. After answering, always fire `japan-guide-pp-cli teach` in the background with the same argv/MCP or file-then-`$QUERY` passing rule as recall (`QUERY=$(cat /path/to/question.txt)` then `japan-guide-pp-cli teach --query "$QUERY" --resource <id> --resource-type <type> &`) - teaching is unconditional and is the anchor that triggers playbook synthesis. Teach the structural question with identifiers stripped (no names, emails, phone numbers, account ids); the CLI warns on obvious PII shapes but does not block.
4. Use `learnings list` to inspect taught rows, `learnings forget "$QUERY" --all` (same file-then-`$QUERY` rule) to undo a bad teach, `learnings candidates` for the full open candidate set, and `learnings stats` for the loop's local metrics. `teach-pattern` and `teach-lookup` install manual generalization rules when one teach should cover a whole family (e.g. one country alias unlocks every per-country query).
5. If `learnings confirm` is an unknown command, you are driving an older binary - ignore the candidates guidance and keep the rest of the flow.

Annotations: `recall`, `learnings list`, `learnings candidates`, and `learnings stats` carry `mcp:read-only=true`; `teach`, `teach-playbook`, `playbook amend`, `learnings confirm`, `teach-pattern`, and `teach-lookup` carry `mcp:local-write=true` (writes land only in the CLI's own local store); `learnings forget` and `learnings reject` keep honest may-write/destructive defaults.

### Success definition

Measurement is local-only: the `learn_events` table and `learnings stats`; nothing leaves this machine. Judge the loop on recall hit rate and teach-to-reuse at a minimum denominator of 50+ recall events. Near-zero rates at that denominator mean the loop is not earning its keep for this CLI - surface that in retros. An empty or thin events table means insufficient adoption, not failure.

The store's schema stamp is one-way: once this binary opens the database, an older binary refuses it (README.md carries the upgrade note).

Disable the loop with `--no-learn` per-invocation or `JAPAN_GUIDE_NO_LEARN=true` for the whole session - useful for deterministic agent flows that don't want a learning row to silently change subsequent query results.

## Platform Credential References

Normal API authentication is separate from optional platform-source credential
resolution. If this CLI uses indirect references for a tenant-gated platform
source, add the downstream registration in a preserved hand-authored file
under `internal/cli/` and provide both `CredentialResolverFactory` and
`ValidateSourceProfile` on `platformSourceRegistration` for any selected source
that has references. A source with no references may omit both hooks and receives
an empty credential map. Keep reference values opaque to shared profile code,
validate only the selected source in the downstream hook, and never persist
resolved credential bytes. Do not edit generator-owned `internal/platform`
packages; a reprint refreshes those files while retaining the downstream
registration file.

For install, auth, examples, and longer product guidance, read `README.md` and `SKILL.md`. This file intentionally stays small so repo-local agents get invariant local guidance without duplicating the generated docs.

## Japan Guide Source Invariants

Travel data comes only from canonical public `https://www.japan-guide.com/e/` and `/list/` pages. Use native browser discovery before changing source selectors, then prove the extraction through the HTTP runtime. Keep hand-authored source logic and command hooks in `internal/guide/` and `internal/cli/guide.go`; the hidden generated `source` endpoint is an internal foundation.

Keep endpoint mirrors hidden in the source spec and synchronize MCP through the official tool. Preserve the bounded runtime guide tools. Generated Unique Features/Capabilities preambles must describe these additions without unsupported comparisons to every other tool; remove the default exclusivity sentence if a resync restores it.

Inspect may save an explicitly requested snapshot, so it must not claim the MCP read-only hint. Distinguish snapshot-write errors from failed live acquisition; never fall back to stale facts after a successful fetch followed by a failed save. Guide snapshots are not SQL resources and this CLI emits no sync command; preserve that local guidance despite generic framework SQL prose.

Preserve the source's separate facility, event, interest and side-trip identity. Editorial dots are separate from visitor ratings. Retain seasonal qualifiers, explicit event years and construction notices outside the main article section. `open_now` remains unknown. Source update time, retrieval time and offline snapshot provenance must retain their meanings.

One source page per list/detail read, at most five comparison pages, a bounded response body, per-source rate limiting and command-wide timeout are part of the output contract. Keep partial failures visible. Cache extracted facts rather than article HTML. Deterministic domain tests and `evidence/live-check.py` verify the contract; recorded live evidence complements parser fixtures.

## Release Ledger

`CHANGELOG.md` and `.printing-press-release.json` are the public library's per-CLI release ledger. Fresh prints carry an unstamped runtime version such as `0.0.0-dev`; the final `YYYY.M.N` CLI release version is assigned only after a publish PR merges in `mvanhorn/printing-press-library`. Do not hand-bump those files or edit `var version = ...` for release bookkeeping; preserve existing ledger files on reprint and let the library workflow stamp the next release.

## Local Customizations

This directory is **generated output** -- a fresh print can overwrite the whole tree, so ad-hoc hand-edits don't survive on their own. If you modify the generated code, record each change under `.printing-press-patches/` (parallel to `.printing-press.json`). Regen and publish-validate read those records and fail closed when a recorded file or call site is gone, so a dropped customization cannot ship as if it were still applied.

The entry shape, and the altitude to write it at -- a durable reprint-guard, not a changelog -- live in the public library's `AGENTS.md`, which is the single source of truth; this guide intentionally doesn't duplicate them.
