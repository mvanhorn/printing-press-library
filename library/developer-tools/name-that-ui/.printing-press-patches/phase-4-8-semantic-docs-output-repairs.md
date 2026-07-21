# Phase 4.8 semantic, docs, and output repairs

Preserve these generated-tree corrections until Printing Press emits them by
default: global `--dry-run` must bypass freshness refresh and NameThatUI
component/style mirror writes; `identify` must normalize local and fresh-live
semantic candidates at top-level `name`, `platform`, `part`, and `source_url`
paths, deriving only canonical NameThatUI ID metadata when no mirror exists.

Keep lexical identification biased toward exact aliases and whole phrases over
isolated generic tokens. The public command tree must not expose generic
create/upsert import behavior or traffic-derived write candidates. Keep doctor
strictly public/no-auth (connectivity, paths, and local cache only), and state
that `doctor --dry-run` is a no-network preview rather than a reachability
test. Sync help/examples must name only the implemented `catalog,styles`
resources; public reference data remains available through its dedicated
reference commands, not `sync`.

For agent output, `ask --agent` must return a compact route plus candidate
summaries with source links and an explicit full-record follow-up, never full
nested records; live identify agent envelopes must set `meta.source=live`.
Inventory examples/selectors must use `files.matches.*`. Lint must ignore prose
uses of low-information single-word APIs such as `List` unless code syntax
makes the API reference clear, including overloaded Cursor/editor and
token/auth prose. Keep `ask --agent` summaries subject-ranked and capped.

Keep sync JSON stdout structured during dry-run previews; send prose mirror
preview diagnostics to stderr. `sync --since` is accepted but has no effect for
the current public catalog/style resources, and archive help must not claim
incremental behavior. Every novel-command Cobra example is a full executable
`name-that-ui-pp-cli <command>` invocation; generated examples use valid
NameThatUI catalog/style resources.
