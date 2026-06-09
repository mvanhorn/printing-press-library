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

## 2026-06-08 — verify-skill `--home` allowlist

- Base private branch: `origin/main`
- Upstream range inspected: `origin/main..upstream/main` as of 2026-06-08
- Candidate commits considered:
  - `15b8e8765` chore(registry): regenerate from library/ [skip ci] — skipped; generated registry noise without selected source changes
  - `1a76b03c4` feat(releases): add per-cli release ledger — deferred; broad release-ledger automation/metadata surface, better as its own PR if wanted
  - `60d76d227` ci(releases): guard release ledger automation — deferred with release-ledger batch
  - `63959b91e` fix(releases): compare ledger guard from merge base — deferred with release-ledger batch
  - `ed0d6fc9e` through `0927177e9` x-twitter control-plane/OAuth fixes — skipped for this sweep; coherent x-twitter runtime batch but larger than the selected CI unblock
  - `ce67a81a2` ci(verify-skill): allow home in COMMON_FLAGS for paths-and-state SKILLs — ported
  - `38687d6bf` test(verify-skill): cover home allowlist entry in flag checks — ported
  - `8f023142e` through `e9b5a989e` openart new CLI and follow-up fixes — skipped; new CLI/reprint surface, not a selective maintenance fix
  - `ba648a997`, `c6d82ac98` generated skills/release ledger updates — skipped; generated automation noise
- Validation run:
  - `python3 verify_skill_test.py` from `.github/scripts/verify-skill/` — pass (`Ran 9 tests ... OK`)
  - `python3 .github/scripts/verify-skill/verify_skill.py --dir library/project-management/linear/` — pass (`All checks passed`)
  - `git diff --check origin/main...HEAD` — pass
- Policy: no public upstream push/PR; branch/PR targets private `cathrynlavery/printing-press-library` only.

## 2026-06-09 — x-twitter raw API escape hatch

- Base private branch: `origin/main`
- Upstream range inspected: `origin/main..upstream/main` as of 2026-06-09 preflight (`da8a7ac9e` public HEAD), with no bulk merge/rebase from `mvanhorn/*`.
- Candidate commits considered:
  - `ea3b20f46` feat(x-twitter): add raw API escape hatch — ported; conflicts resolved to preserve the private fork's existing thread/articles capability index while adding upstream `raw`, structured dry-run previews, docs, tests, and patch manifest.
  - `8266a14ff` fix(x-twitter): parse raw headers by colon first — ported.
  - `5e1ec9cd9` fix(x-twitter): disambiguate raw header separators — ported.
  - `ab4bd6fff` fix(x-twitter): tighten raw validation — ported.
  - `b8f294951` fix(x-twitter): clarify bearer token auth — deferred; separate auth-surface cleanup, not mixed into the raw-command PR.
  - `92cbb07bb` feat(x-twitter): add OAuth2 PKCE login command — deferred; larger auth feature needing its own focused validation.
  - OpenArt/1Password/XAI/new CLI and generated registry/skills/release-ledger commits — skipped; catalog/reprint-sized or generated noise, not a narrow daily maintenance port.
  - Flight Goat and Dice FM changes — skipped; unrelated product surfaces for this sweep.
- Validation planned/run on branch `upstream-sweep/20260609-x-twitter-raw`: `git diff --check origin/main...HEAD`, `python3 .github/scripts/verify-skill/verify_skill.py --dir library/social-and-messaging/x-twitter/`, `(cd library/social-and-messaging/x-twitter && go test ./...)`, `(cd library/social-and-messaging/x-twitter && go build ./...)`, and `(cd library/social-and-messaging/x-twitter && go vet ./...)`.
- Policy: no public upstream push/PR; branch/PR targets private `cathrynlavery/printing-press-library` only.
