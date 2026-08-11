# Polish 2026-08-11: model-key echo, credential-free similar example, x/text bump

Durable intent for reprint. If a regen overwrites these, re-apply the intent
(not necessarily the exact diff).

## 1. Novel commands echo the canonical `model_key`

`history diff`, `formats gaps`, and `similar` accept a `<source>:<id>` model
key but previously emitted only split `source` + `model_id` fields. Agents
correlating batch output (and the live-check token matcher) need the input key
echoed verbatim. Every JSON output map these commands emit now includes
`"model_key": modelKey(source, id)` alongside `source`/`model_id`:

- `internal/cli/history_diff.go` — `diffOneModel` result map and the `--all`
  per-model error map.
- `internal/cli/formats_gaps.go` — not-found map and the main output map.
- `internal/cli/similar.go` — both error maps and the `seed` block.

## 2. `similar` example uses a credential-free source

The CLI advertises `auth_type: none`; the flagship `similar` example used
`thingiverse:763622`, which 401s on a fresh install (Thingiverse needs
`THINGIVERSE_TOKEN`). Changed to `printgoat-pp-cli similar printables:3161
--agent` (Printables needs no auth, and the 3D BENCHY seed demonstrates the
same-model/same-designer exclusions well) in:

- `research.json::novel_features[]` and `novel_features_built[]` (manuscripts
  run `20260721-105814-1da3c401`, both the manuscripts copy and
  `.manuscripts/` archive) — README/SKILL/root-help render from this.
- `internal/cli/similar.go` Cobra `Example` field.

## 3. Dependency floor: `golang.org/x/text >= v0.39.0`

`go.mod` bumped `golang.org/x/text` v0.38.0 → v0.39.0 to clear GO-2026-5970
(infinite loop on invalid input; reachable via `internal/learn/journal.go`),
which failed `publish validate`'s govulncheck gate. Any reprint must not
regress below v0.39.0.
