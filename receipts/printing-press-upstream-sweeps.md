# Printing Press library upstream sweeps

## 2026-06-07 — docs + standalone pycookiecheat + Linear sync cap

- Base private branch: `origin/main`
- Upstream range inspected: `origin/main..upstream/main` as of 2026-06-07
- Candidate commits considered:
  - `e2a071867` docs: stop telling manual submitters to edit generated registry.json (#1079) — ported
  - `b86586dfb` fix(substack): support standalone pycookiecheat (#1077) — ported
  - `f386c0546` chore(skills): regenerate per-app skills [skip ci] — skipped; generated mirror noise without selected source changes
  - `15b48d0eb` fix(linear): lower project sync query complexity — ported
  - `2c2db1093` fix(library): support standalone pycookiecheat in cookie CLIs (#1078) — deferred; broad multi-CLI surface, should be a separate focused PR
  - `e85b96b79` chore(registry): regenerate from library/ [skip ci] — skipped; generated registry noise
  - `935897c0a` feat(tripadvisor): add tripadvisor (#1073) — skipped; new CLI/reprint surface, not a selective maintenance fix
  - `5acd70552` chore(registry): regenerate from library/ [skip ci] — skipped; generated registry noise
  - `32518c58a` chore(library): normalize patch metadata layout — deferred; broad catalog metadata migration, not mixed with runtime fixes
  - `d075d4775` merge for #1080 — skipped directly; merge commit wrapping metadata-layout work
  - `7c024eb5e` chore(skills): regenerate per-app skills [skip ci] — skipped; generated mirror noise
- Validation run:
  - `python3 .github/scripts/verify-skill/verify_skill.py --dir library/media-and-entertainment/substack/` — pass
  - `python3 .github/scripts/verify-skill/verify_skill.py --dir library/project-management/linear/` — pass
  - `(cd library/media-and-entertainment/substack && go test ./...)` — pass
  - `(cd library/project-management/linear && go test ./...)` — pass
  - `git diff --check origin/main...HEAD` — pass
  - `(cd library/media-and-entertainment/substack && go vet ./...)` — pass
  - `(cd library/project-management/linear && go vet ./...)` — pass
- Policy: no public upstream push/PR; branch/PR targets private `cathrynlavery/printing-press-library` only.
