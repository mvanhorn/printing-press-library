# Context-pack and crosswalk local command scaffolds

Preserve the implemented `internal/cli/context_pack.go` and
`internal/cli/crosswalk.go` command bodies without changing generated
`root.go`. Both commands are local-only readers over the synced NameThatUI
component mirror (and the style-detail mirror only when `context-pack` is
given `--style`). Keep conservative component/style ambiguity, sync hints,
source URLs, non-null JSON collections, and dry-run without opening SQLite.
Do not replace record-backed terminology or upstream sections with inferred
advice during reprints. Context-pack selectors refer to the packet's
top-level `parts`, `style_signals`, and `cautions` fields. Treat `--framework
web` as the HTML-and-ARIA aggregate while retaining case-insensitive exact
matching for named frameworks.
