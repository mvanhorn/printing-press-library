# Next Session Handoff

**Last session:** 2026-05-25_21-44-15
**Branch:** feat/home-goat
**PR:** #855 (OPEN, 7/7 CI green)

## Immediate Priority

**Rename home-goat to reno-goat.** User stated preference, not yet executed. This is a full slug rename across ~124 files:
- Directory: `library/commerce/home-goat/` -> `library/commerce/reno-goat/`
- Binary names: `home-goat-pp-cli` -> `reno-goat-pp-cli`, `home-goat-pp-mcp` -> `reno-goat-pp-mcp`
- Module path: `github.com/mvanhorn/printing-press-library/library/commerce/home-goat` -> `.../reno-goat`
- All internal import paths across ~60 Go files
- `.printing-press.json` fields (`api_name`, `cli_name`)
- Env var prefix: `HOME_GOAT_*` -> `RENO_GOAT_*`
- Cobra `Use:` strings, help text, descriptions throughout
- SKILL.md, README.md, AGENTS.md content
- Branch name: `feat/home-goat` -> `feat/reno-goat`
- PR #855 title: `feat(reno-goat): add reno-goat`

After rename, re-run local validation:
```bash
python3 .github/scripts/verify-skill/verify_skill.py --dir library/commerce/reno-goat/
cd library/commerce/reno-goat/ && go build ./... && go vet ./...
```

## Context

- home-goat is a multi-source home furnishing CLI combining Ferguson, West Elm, Rejuvenation, Article, and Shopify DTC stores
- 7 novel features: Compound Category Search, Price Watch, Project Tracker, Stale Detector, Spec Sheet Export, Brand Cross-Reference, Review Aggregation
- Ferguson and Article sources are stubbed (JWT auth and APQ hash discovery gaps respectively)
- Constructor.io price normalization was patched (camelCase vs snake_case) and recorded in `.printing-press-patches.json`

## Post-Merge Work (not blocking PR)

1. **Ferguson JWT acquisition** -- browser-based anonymous token extraction for plumbing/HVAC source
2. **Article APQ hash discovery** -- reverse-engineer persisted query sha256 hashes from Article frontend bundle
3. **Price watch scraping** -- `watch check` currently HEAD-only; needs per-source response parsing
4. **Review aggregation backends** -- external review site endpoint discovery (Houzz, Consumer Reports)

## Session Learnings to Carry Forward

- Never touch PR state (open/close/merge/edit) without explicit user instruction
- Library CLIs need `github.com/mvanhorn/printing-press-library/library/<cat>/<slug>` module paths, not standalone
- Fork PRs must be based on upstream/main, not origin/main
- `verify_publish_package.py` catches PATCH marker / patches[] mismatches -- always populate patches[]

## Key Files

- CLI root: `library/commerce/home-goat/`
- Manifest: `library/commerce/home-goat/.printing-press.json`
- Patches: `library/commerce/home-goat/.printing-press-patches.json`
- PR: https://github.com/mvanhorn/printing-press-library/pull/855
