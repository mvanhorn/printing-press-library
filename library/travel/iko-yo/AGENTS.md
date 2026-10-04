# Iko-yo Trip Printed CLI Agent Guide

This directory is a generated `iko-yo-pp-cli` printed CLI. It was produced by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press), so treat systemic fixes as upstream Printing Press fixes first. Keep local edits narrow and document why a generated-tree patch belongs here.

## Local Operating Contract

Start by asking the generated CLI for current runtime truth:

```bash
iko-yo-pp-cli doctor --json
iko-yo-pp-cli agent-context --pretty
```

Use runtime discovery instead of relying on a copied command list:

```bash
iko-yo-pp-cli which "family facts" --json
iko-yo-pp-cli trip inspect --help
```

Add `--agent` to command invocations for JSON, compact output, non-interactive defaults, and no color:

```bash
iko-yo-pp-cli trip inspect spots/8220 --agent
```

Before running an unfamiliar command that may mutate remote state, inspect its help and prefer a dry run:

```bash
iko-yo-pp-cli trip inspect --help
iko-yo-pp-cli trip inspect spots/8220 --dry-run --agent
```

When a command requires confirmation, pass `--yes` explicitly only after the target, arguments, and side effects are clear. `--agent` does not imply `--yes`.

## Novel Command Data Sources

Every hand-written novel command must declare its strategy in a Go line comment:

```go
// pp:data-source auto
```

Use exactly one of `auto`, `local`, `live`, or `computed`. Keep `auto` when the command honors `--data-source auto|local|live` by preferring live data with a local fallback; use `local` for local-only reads, `live` for remote-only reads, and `computed` for pure computation from embedded rules. Change a generated scaffold's `auto` default deliberately when its implementation has a narrower source, and keep `cmd.Annotations["pp:data-source"]` on the same command in sync; `--agent` envelopes read that annotation for `meta.source`. Reject incompatible `--data-source` requests with a clear error. TODO stubs still fail dogfood even when annotated.

## Trip scope and saved facts

Source access is read-only, unauthenticated Iko-yo Trip public HTML. Do not bypass blocked core Iko-yo routes or claim the full core catalog is integrated. Prefer the four `trip` commands; the six low-level endpoints are normalized compatibility routes.

`trip discover` reports a bounded, publication-ordered listing window including archived events. Preserve scan coverage and the zero-match note. `trip inspect` returns bounded source facts and canonical handoff URLs. `trip compare` uses supported/excluded/unknown age and amenity evidence, separate qualified fees, and published application intervals. No command guarantees admission, daily operation or seats. `trip cached` reads a partial local fact collection with unchanged observation times.

Successful Trip invocations disable automatic learning and journaling of age/date inputs. Never teach child profiles, contributor identities or private travel histories. Optional learning is for general command patterns only. To disable the framework loop elsewhere, use `--no-learn` or `IKO_YO_NO_LEARN=true`.

## Release Ledger

`CHANGELOG.md` and `.printing-press-release.json` are the public library's per-CLI release ledger. Fresh prints carry an unstamped runtime version such as `0.0.0-dev`; the final `YYYY.M.N` CLI release version is assigned only after a publish PR merges in `mvanhorn/printing-press-library`. Do not hand-bump those files or edit `var version = ...` for release bookkeeping; preserve existing ledger files on reprint and let the library workflow stamp the next release.

## Local Customizations

This directory is **generated output** -- a fresh print can overwrite the whole tree, so ad-hoc hand-edits don't survive on their own. If you modify the generated code, record each change under `.printing-press-patches/` (parallel to `.printing-press.json`). Regen and publish-validate read those records and fail closed when a recorded file or call site is gone, so a dropped customization cannot ship as if it were still applied.

The entry shape, and the altitude to write it at -- a durable reprint-guard, not a changelog -- live in the public library's `AGENTS.md`, which is the single source of truth; this guide intentionally doesn't duplicate them.
