# Generator upgrade to 4.35.3

The new-CLI floor rejected `printing_press_version` 4.34.2. A later hand edit that set the field to 4.35.0 without running the generator was not kept.

This refresh used the released Printing Press binary `cli-printing-press 4.35.3` from tag `v4.35.3` (`cli-printing-press_linux_amd64.tar.gz`).

```bash
cli-printing-press generate \
  --spec library/productivity/normattiva/spec.yaml \
  --output /tmp/normattiva-fresh-4353 \
  --name normattiva \
  --category productivity \
  --owner "Luigi Pastore" \
  --research-dir library/productivity/normattiva/.manuscripts/20261028-132341-29de1036/research \
  --validate

cli-printing-press regen-merge library/productivity/normattiva \
  --fresh /tmp/normattiva-fresh-4353 \
  --apply \
  --force
```

`generate --validate` on the fresh tree passed `go mod tidy`, the safe-module checks, `go test ./...`, `govulncheck ./...`, `go vet ./...`, `go build ./...`, `--help`, `version`, and `doctor`.

`regen-merge` synthesized the comparison base with cli-printing-press v4.34.2, then applied the 4.35.3 template. It classified the three P1 fixes (`verifica.go`, `fascicolo.go`, `novita.go`, `cite.go`) and their regression tests as NOVEL and left them in place. Creator stays `luigipastore61063-del` / Luigi Pastore. Patch files still record `base_printing_press_version` 4.34.2.

`generate` wrote fresh `run_id` `20261107-201401` because it could not derive a run from `--research-dir`, and it did not write a new phase 5 proof. Before `regen-merge --apply`, that fresh `run_id` was set to `20261028-132341-29de1036` so the existing research directory remains the manuscript. `printing_press_version` `4.35.3` and `generated_at` `2026-11-07T20:14:01.493993454Z` were copied from the fresh manifest by `regen-merge`.

`phase5-acceptance.json` in this directory is the original print's 184/184 result. It was not re-executed on the regenerated tree.

The decl overlay aliased imports in `root.go`, `local_data.go`, `tools.go`, and `agent_context.go` without changing behavior. Those aliases were removed. `root.go` keeps the novel-command help text and registrations, and its `--deliver` help now names `spacefast` to match the new `deliver_spacefast.go` emission. `go mod tidy` put the merged `go.mod` back to the previous direct and indirect split.
