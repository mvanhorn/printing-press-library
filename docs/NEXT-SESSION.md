# Next Session Handoff

**Last session:** 2026-05-25_23-28-33
**Branch:** feat/home-goat (printing-press-library), feat/readme-coverage-update (geo-goat)
**PR:** #855 (home-goat, OPEN, 7/7 CI green)

## How we got here

- 2026-05-25 (session 1): Generated home-goat multi-source home furnishing CLI via printing-press pipeline. PR #855 opened.
- 2026-05-25 (session 2): Integrated ePropertyPlus land bank data as native geo-goat source — landbank command group + transcendence fan-out. Added Linear MCP, removed Vercel plugin. Decision: inline ePropertyPlus API client in geo-goat rather than cross-CLI dependency.

## Immediate Priority

**Rename home-goat to reno-goat.** User stated preference (prior session), not yet executed. Full slug rename across ~124 files:
- Directory: `library/commerce/home-goat/` → `library/commerce/reno-goat/`
- Binary names: `home-goat-pp-cli` → `reno-goat-pp-cli`, `home-goat-pp-mcp` → `reno-goat-pp-mcp`
- Module path: `github.com/mvanhorn/printing-press-library/library/commerce/home-goat` → `.../reno-goat`
- All internal import paths across ~60 Go files
- `.printing-press.json` fields (`api_name`, `cli_name`)
- Env var prefix: `HOME_GOAT_*` → `RENO_GOAT_*`
- Cobra `Use:` strings, help text, descriptions throughout
- SKILL.md, README.md, AGENTS.md content
- Branch name: `feat/home-goat` → `feat/reno-goat`
- PR #855 title: `feat(reno-goat): add reno-goat`

After rename, re-run local validation:
```bash
python3 .github/scripts/verify-skill/verify_skill.py --dir library/commerce/reno-goat/
cd library/commerce/reno-goat/ && go build ./... && go vet ./...
```

## Open state

- **PR #855** (home-goat) — open, 7/7 CI green, pending reno-goat rename
- **PR #823** (multimail) — open
- **geo-goat `feat/readme-coverage-update`** — has landbank integration (commit `6c04e11`), not yet PR'd
- **Linear MCP** — configured (`linear-server`, HTTP transport), needs OAuth on first use
- **TSDR note (MUST PERSIST until resolved):** The TSDR CLI is for looking up specific trademarks by serial/registration number — it queries the TSDR API, not the TESS full-text/design search. Can't do "find all marks with a key and hat" visual search. Use USPTO's TESS search directly.

## Delegated work awaiting supervisor review

(none)

## Recommended first move next session

**Option A:** Execute the reno-goat rename on PR #855:
```bash
cd ~/Documents/GitHub/printing-press-library
# ~124 file rename: home-goat → reno-goat across directory, module paths, imports, env vars, docs
```

**Option B:** PR the geo-goat landbank integration:
```bash
cd ~/printing-press/library/geo-goat
gh pr create --title "feat(geo-goat): integrate ePropertyPlus land bank data" --body "..."
```

## Context (home-goat / reno-goat)

- Multi-source home furnishing CLI combining Ferguson, West Elm, Rejuvenation, Article, and Shopify DTC stores
- 7 novel features: Compound Category Search, Price Watch, Project Tracker, Stale Detector, Spec Sheet Export, Brand Cross-Reference, Review Aggregation
- Ferguson and Article sources are stubbed (JWT auth and APQ hash discovery gaps)
- Constructor.io price normalization patched and recorded in `.printing-press-patches.json`

## Context (geo-goat landbank)

- 6 new files (~1000 lines): registry, helpers, command group (search, instances, detail)
- ePropertyPlus land bank data wired into 3 transcendence commands: site-report, overlay, neighborhood
- 6 known instances: KC, Cleveland, Toledo, Canton, Youngstown, Warren
- ePropertyPlus API: `https://public-{slug}.epropertyplus.com/landmgmtpub/remote/public`, no auth

## Post-Merge Work (not blocking PRs)

1. **Ferguson JWT acquisition** — browser-based anonymous token extraction
2. **Article APQ hash discovery** — reverse-engineer persisted query sha256 hashes
3. **Price watch scraping** — `watch check` currently HEAD-only
4. **Review aggregation backends** — external review site endpoint discovery

## Session Learnings to Carry Forward

- Never touch PR state (open/close/merge/edit) without explicit user instruction
- Library CLIs need `github.com/mvanhorn/printing-press-library/library/<cat>/<slug>` module paths, not standalone
- Fork PRs must be based on upstream/main, not origin/main
- `verify_publish_package.py` catches PATCH marker / patches[] mismatches
- ePropertyPlus API: base URL `https://public-{slug}.epropertyplus.com/landmgmtpub/remote/public`, index at `/property/searchSummaryPublicMapQuery`, detail at `/property/getPublishedProperty?propertyId=<id>`. No auth.
- GitHub API git tree traversal for reading code from detached commits: chain tree→subtree→blob SHA lookups

## Key Files

- home-goat CLI root: `library/commerce/home-goat/`
- home-goat manifest: `library/commerce/home-goat/.printing-press.json`
- home-goat PR: https://github.com/mvanhorn/printing-press-library/pull/855
- geo-goat landbank: `~/printing-press/library/geo-goat/internal/cli/landbank*.go`
- geo-goat repo: `IiInfra/geo-goat`, branch `feat/readme-coverage-update`

## Repo state at handoff

| Repo | Visibility | Branch | HEAD |
|------|-----------|--------|------|
| printing-press-library | PUBLIC | feat/home-goat | `c034569c` |
| geo-goat (IiInfra) | PRIVATE | feat/readme-coverage-update | `6c04e11` |
