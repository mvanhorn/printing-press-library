# Polish patch — generated-tree fixes (2026-07-17)

Reprint guard: these are hand-edits to generated (`DO NOT EDIT`) files applied
during `/printing-press-polish`. A fresh print overwrites them, so re-apply the
intent (or, better, fix upstream in the Printing Press so the next print emits
them). Each item notes the systemic (generator) root cause.

## 1. Root description brand spelling (`internal/cli/root.go`)
- Changed `rootCmd.Short` and `rootCmd.Long` opening from "Every StarterStory …"
  to "Every Starter Story …" (two words).
- Reason: dogfood description-drift — root.Short must match
  `research.json::narrative.headline` ("Starter Story"). The generator rendered
  root.Short from `spec.yaml::cli_description`, which was misspelled "StarterStory".
- Durable fix (also applied here): `spec.yaml` `cli_description` + `description`
  corrected to "Starter Story", so a reprint renders root.go correctly.
- Systemic: generator should reconcile spec `cli_description` against the
  research narrative headline (retro candidate).

## 2. Dead `--max-age` flag removed (`internal/cli/root.go`)
- Removed the `rootFlags.maxAge` field and the
  `PersistentFlags().DurationVar(&flags.maxAge, "max-age", …)` registration.
- Reason: dogfood dead-flag. The flag's only consumer is the sync-hint feature
  in the generated `sync_hint.go`, which is compile-gated OFF
  (`const syncHintsEnabled = false`). The flag accepted a value that was never
  read. No README/SKILL references, no behavior change.
- Systemic (retro candidate): the generator should NOT emit `--max-age` when
  `syncHintsEnabled == false`. Every CLI printed with the hint feature gated off
  carries this same dead flag.

## 3. Dead `hasChangedLocalFlags` removed (`internal/cli/helpers.go`)
- Removed the unused `hasChangedLocalFlags(cmd)` helper (its only reference was
  its own doc comment). `pflag` import stays used elsewhere in the file.
- Systemic (retro candidate): generator emits this helper unconditionally even
  when nothing calls it.

## Not applied — blocked (needs reprint or tooling fix)
- MCP description overrides for `breakdowns_get`, `businesses_get`, `ideas_get`,
  `tools_get` are written to `mcp-descriptions.json` but could NOT be applied:
  `cli-printing-press mcp-sync` refused with "reprint required (missing
  Client.Config, Client.NoCache)" even though `internal/client/client.go` DOES
  declare both `Config` and `NoCache`. This looks like a false-positive
  mcp-sync compatibility probe (retro candidate). The override file is ready;
  a reprint (or fixed mcp-sync) will apply it and lift MCP Desc Quality from 0/10.
