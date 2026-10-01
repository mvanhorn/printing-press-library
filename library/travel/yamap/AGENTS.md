# YAMAP CLI development

Keep scope public, first-party and read-only. Use `internal/hiking` for domain parsing and bounded source reads; focused commands are wired in `internal/cli/hiking_commands.go`. Generated framework packages are separate from focused workflows. Preserve additive hooks and the internal spec on regeneration.

Read `README.md` for evidence definitions before changing metrics, freshness, map coverage or publisher/contributor classifications. Keep missing values null, Japanese names/source IDs intact, and unknown official closures explicit. Never infer open/safe status from reports or false publisher flags.

For parsing/cache changes, run deterministic `internal/hiking` tests. For API/query changes, run `tools/live_e2e.py` from the canonical project and compare fresh source JSON; fixtures never count as live evidence. All checks use an isolated `YAMAP_HOME` and cache directory. Follow `.press-run.json` receipts when resuming the build. Source edits invalidate Press live acceptance and require a fresh matrix before promotion. Do not publish or alter shared config without explicit authorization.
