# Phase 5.5 — Polish — immovlan-pp-cli (run 20260922-135147-d48976cc)

Polish pass (printing-press-polish, mid-pipeline, forked):

```
  Scorecard:   83  → 83  (0)
  Verify:      100% → 100% (45/45)
  gosec (hand-authored): 17 → 0
  Tools-audit: 3 → 0 pending (1 fixed, 2 accepted in generated files)
  Output review: 4 WARN → all addressed
  PII-audit: 0 → 0
```

Fixed: photo dir/file permissions and cleanup error handling in `show.go`; `db.Close` on schema-init failure; `PhotoHash` sha1 → sha256 (photo-hash version bumped to 3 so stored hashes are recomputed); `relisted` orders references by site `created_at` and requires ≥ 7 days between publications for two still-live references (2 new tests); `enrich` explains "field absent, read within --retry-after" instead of "nothing to enrich"; `peb-trap` note points at `--include-unknown`; `split-candidates` truncation note; Shorts of `saved list`, `shortlist add/remove`; README's 35 invocations flag-verified.

Skipped (generator-owned, retro candidates): 5 dead helpers in `helpers.go` and `maxAge` in `root.go`; 32 gosec findings in generated files (`store.go`, `teach.go`, `client.go`, `config.go`, `learn/*`, `platform/*`, MCP main; `migration.go` identical to the immoweb library copy); two `thin-short` findings on generated `list` commands. Environmental: live-check `same-as` needs an immoweb-pp-cli store, `enrich` example needs `--allow-destructive`.

ship_recommendation: **ship** — further polish not recommended.
