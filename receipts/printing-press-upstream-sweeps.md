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
- Validation run on branch `upstream-sweep/20260609-x-twitter-raw`: `git diff --check origin/main...HEAD`, `python3 .github/scripts/verify-skill/verify_skill.py --dir library/social-and-messaging/x-twitter/`, `(cd library/social-and-messaging/x-twitter && go test ./...)`, `(cd library/social-and-messaging/x-twitter && go build ./...)`, `(cd library/social-and-messaging/x-twitter && go vet ./...)`, and `(cd library/social-and-messaging/x-twitter && go run golang.org/x/vuln/cmd/govulncheck@latest ./...)`.
- Policy: no public upstream push/PR; branch/PR targets private `cathrynlavery/printing-press-library` only.

## 2026-06-10 — Flight Goat whole-hour Google Flights times

- Base private branch: `origin/main`
- Upstream range inspected: `origin/main..upstream/main` as of 2026-06-10 preflight (`4d925c2e6` public HEAD), with no bulk merge/rebase from `mvanhorn/*`.
- Candidate commits considered:
  - `43f1f5a58` fix(flight-goat): parse whole-hour departure times instead of defaulting to midnight — ported; narrow data-correctness fix for Google Flights jspb time arrays.
  - `84d0c753c` fix(flight-goat): allow genuine midnight in fabricated-midnight regression test — ported with the parser regression test refinement.
  - `8266a14ff`, `5e1ec9cd9`, `ab4bd6fff`, `b8f294951`, `92cbb07bb`, `9694e97bb` x-twitter raw/auth/OAuth2 fixes — skipped this sweep; the private fork already has prior x-twitter raw work in flight and the remaining auth/OAuth surface should stay separate.
  - `59bab1464`/`75a122f65` 1Password, `dfac5568f` Cloudflare, `8f023142e` OpenArt, `ee8d0dfd7` XAI, and other new-CLI/reprint-sized commits — skipped; not a narrow private-fork maintenance port.
  - Generated registry/skills/release-ledger commits — skipped; automation noise unless paired with selected source changes.
  - `2c2db1093` standalone pycookiecheat multi-CLI support and broad patch-layout/release-ledger changes — deferred; useful, but too broad for the Flight Goat runtime PR.
- Validation run on branch `upstream-sweep/20260610-flight-goat-time`:
  - `git diff --check origin/main...HEAD` — pass.
  - `python3 .github/scripts/verify-skill/verify_skill.py --dir library/travel/flight-goat/` — pass (`All checks passed`).
  - `(cd library/travel/flight-goat && go test ./internal/gflights/...)` — pass.
  - `(cd library/travel/flight-goat && go build ./...)` — pass.
  - `(cd library/travel/flight-goat && go vet ./...)` — pass.
  - `(cd library/travel/flight-goat && go run golang.org/x/vuln/cmd/govulncheck@latest ./...)` — blocked by Go toolchain standard-library findings in local `go1.26.3` (`GO-2026-5039`, `GO-2026-5037`; fixed in Go 1.26.4), unrelated to the touched Flight Goat parser/test code.
- Policy: no public upstream push/PR; branch/PR targets private `cathrynlavery/printing-press-library` only.

## 2026-06-11 — ClawHub workflow pin

- Base private branch: `origin/main`
- Upstream range inspected: `origin/main..upstream/main` as of 2026-06-11 preflight (`39f91b1ce` public HEAD), with no bulk merge/rebase from `mvanhorn/*`.
- Candidate commits considered:
  - `0614c495c` fix(ci): pin clawhub 0.20.0 and drop the removed `--clawscan-note` flag — ported; narrow CI/package-publish compatibility fix for the discovery-skill workflow.
  - `bdbaf0bb2`, `bc8a6f352` Dice FM entity-normalization hardening — deferred; useful runtime hardening, but a broader CLI-specific surface better handled in its own PR.
  - `320263b85`, `92cbb07bb`, `9694e97bb` x-twitter LINK/OAuth2 fixes — deferred; auth/content-model changes should stay separate from the CI workflow fix.
  - `b8969dfe8` and follow-up Masterpark commits — skipped; new CLI/product surface, not a narrow maintenance port.
  - `8f6cfd92d`, `dd32ea19c`, `5266500b0` WaveSpeed/Cloudflare fixes — skipped; already present in private upstream sweep branches/merged public PR context, not selected for this CI-focused PR.
  - Generated registry/skills/release-ledger commits — skipped; generated automation noise unless paired with selected source changes.
- Validation run on branch `upstream-sweep/20260611-clawhub-pin`:
  - `git diff --check origin/main...HEAD` — pass.
  - `python3 - <<'PY' ... yaml.safe_load(...) ... PY` against `.github/workflows/publish-clawhub-discovery-skill.yml` — pass; workflow YAML parses.
  - `grep -n "clawhub@\|clawscan-note" .github/workflows/publish-clawhub-discovery-skill.yml` — pass; all executable ClawHub invocations use `clawhub@0.20.0`, and `--clawscan-note` remains only in explanatory comments.
- Policy: no public upstream push/PR; branch/PR targets private `cathrynlavery/printing-press-library` only.

## 2026-06-12 — EU Tenders descriptive BE notice titles

- Base private branch: `origin/main`
- Upstream range inspected: `origin/main..upstream/main` as of 2026-06-12 preflight (`f1a2fa4c7` public HEAD), with no bulk merge/rebase from `mvanhorn/*`.
- Candidate commits considered:
  - `ad3cfda97` fix(eu-tenders): resolve descriptive titles for BE notices with reference-only title-lot — ported; narrow data-correctness fix for title selection and multilingual scalar handling.
  - `0d9fe0699` fix(eu-tenders): skip empty strings in array branch of ResolveMultilingual — ported; direct review/regression follow-up for the same helper.
  - `0e7f6cb21` fix generated artifact workflow concurrency — deferred; CI workflow hardening, separate surface from the selected CLI runtime fix.
  - `60964575e`/`1abfdbc87` sec-edgar ownership command and review follow-up — deferred; useful feature-sized CLI surface, better as its own focused PR.
  - `7bb58b2d7`/`0fd5ee0a9` Daraz, `4df4db6eb`/`ca4434d23`/`7addd9fd6` Agent Desktop, Rechtspraak, Instagram, Shopping, Azure Cost Admin new-CLI batches — skipped; new CLI/reprint/product surfaces, not daily selective maintenance fixes.
  - Generated registry/skills/release-ledger commits — skipped; automation noise unless paired with selected source changes.
- Validation run on branch `upstream-sweep/20260612-eu-tenders-title-fallback`:
  - `git diff --check origin/main...HEAD` — pass.
  - `python3 .github/scripts/verify-skill/verify_skill.py --dir library/sales-and-crm/eu-tenders/` — pass (`All checks passed`).
  - `(cd library/sales-and-crm/eu-tenders && go test ./...)` — pass.
  - `(cd library/sales-and-crm/eu-tenders && go build ./...)` — pass.
  - `(cd library/sales-and-crm/eu-tenders && go vet ./...)` — pass.
  - `(cd library/sales-and-crm/eu-tenders && go run golang.org/x/vuln/cmd/govulncheck@latest ./...)` — blocked by local Go standard-library findings in `go1.26.3` (`GO-2026-5039`, `GO-2026-5037`; fixed in Go 1.26.4), unrelated to the touched EU Tenders title/multilingual code.
- Policy: no public upstream push/PR; branch/PR targets private `cathrynlavery/printing-press-library` only.

## 2026-06-13 — generated artifact workflow concurrency

- Base private branch: `origin/main`
- Upstream range inspected: `origin/main..upstream/main` as of 2026-06-13 preflight (`2f5f13550` public HEAD), with no bulk merge/rebase from `mvanhorn/*`.
- Candidate commits considered:
  - `0e7f6cb21` fix generated artifact workflow concurrency — ported; narrow CI workflow reliability fix to prevent generated-artifact runs from evicting each other from a shared concurrency group.
  - `ce67a81a2` ci(verify-skill): allow home in COMMON_FLAGS for paths-and-state SKILLs — already present by equivalent patch-id; cherry-pick was empty.
  - `38687d6bf` test(verify-skill): cover home allowlist entry in flag checks — already present by equivalent patch-id; cherry-pick was empty.
  - `0614c495c` fix(ci): pin clawhub 0.20.0 and drop the removed `--clawscan-note` flag — already present by equivalent patch-id; cherry-pick was empty.
  - `update-cli-release-ledger.yml` hunk from `0e7f6cb21` — skipped because the private fork does not currently have that workflow on `origin/main`.
  - New CLI/reprint batches and follow-up generated registry/skills/release-ledger commits (`roadside-america`, `tripadvisor`, `openart`, `autotempest`, `motohunt`, `yeswehack`, `agent-desktop`, etc.) — skipped; not a narrow selective maintenance port.
- Validation run on branch `upstream-sweep/20260613-ci-verifier-fixes`:
  - YAML parse of `.github/workflows/generate-registry.yml`, `.github/workflows/generate-skills.yml`, and `.github/workflows/normalize-patches.yml` with `yaml.safe_load` — pass.
  - `python3 .github/scripts/verify-skill/verify_skill_test.py` — pass (`Ran 9 tests ... OK`).
  - `git diff --check origin/main...HEAD -- .github/workflows/generate-registry.yml .github/workflows/generate-skills.yml .github/workflows/normalize-patches.yml` — pass.
- Policy: no public upstream push/PR; branch/PR targets private `cathrynlavery/printing-press-library` only.
