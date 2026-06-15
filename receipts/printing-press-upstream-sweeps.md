# Printing Press Library Upstream Sweeps

## 2026-06-05T12:53:40Z — walkingpad docs

- **Private fork:** `cathrynlavery/printing-press-library` (`origin/main`)
- **Public upstream:** `mvanhorn/printing-press-library` (`upstream/main`), push URL disabled
- **Inspected range:** `origin/main..upstream/main` after fetching both remotes
- **Candidates considered:**
  - `69a420206` — `docs(walkingpad): align README and SKILL with canonical catalog shape (#1049)` — selected; narrow published-library docs/skill fix, no generated mirrors or registry in the commit.
  - `3dc063231` — `chore(skills): regenerate per-app skills [skip ci]` — skipped; generated `cli-skills/` mirror, regenerated post-merge by repo automation.
  - `fe7f93dbe` — `feat(x-twitter): reprint x-twitter under Printing Press 4.20.1 (#1051)` — skipped/deferred; broad reprint, not a small upstream sweep candidate.
  - `77cfbda69` — `chore(skills): regenerate per-app skills [skip ci]` — skipped; generated `cli-skills/` mirror.
  - `38981b46a` — `chore(registry): regenerate from library/ [skip ci]` — skipped; generated `registry.json`, regenerated post-merge by repo automation.
- **Ported commits:** `69a420206` cherry-picked as `7528fc2d1` onto `upstream-sweep/20260605-walkingpad-docs`.
- **Validation run:**
  - `python3 .github/scripts/verify-skill/verify_skill.py --dir library/devices/walkingpad/` — PASS
  - `cd library/devices/walkingpad && go test ./...` — PASS
  - `cd library/devices/walkingpad && go build ./...` — PASS
  - `cd library/devices/walkingpad && go vet ./...` — PASS
  - `cd library/devices/walkingpad && $HOME/go/bin/govulncheck ./...` — PASS (`No vulnerabilities found.`)
  - `git diff --check origin/main...HEAD` — PASS
- **PR:** `https://github.com/cathrynlavery/printing-press-library/pull/13`
- **Policy confirmation:** no public push; branch/PR target private fork only.

## 2026-06-06T11:17:51Z — x-twitter articles cookie user/lifecycle fixes

- Repo: `cathrynlavery/printing-press-library` private fork, comparing `origin/main..upstream/main` after fetching `mvanhorn/printing-press-library`.
- Candidate range reviewed: `dfffcde19` through `4455336cd` from public upstream, with private-only X/Twitter auth/composite work preserved.
- Ported:
  - `875fce8db` — `fix(x-twitter): provide user id for articles list (#1068)`; adapted around the private fork's hand-authored cookie-capture flow rather than pulling the broader Chrome profile selector surface.
  - `f8f5f3cbc` — `fix(x-twitter): include articles lifecycle variable (#1069)`.
- Skipped/deferred:
  - New/reprint CLIs (`twelvelabs`, `qbo`, `xero`, `function-health`, `foxnews`, `uk-train-goat`) and generated registry/skills refreshes: reprint/catalog-sized, not a narrow daily sweep patch.
  - `0927a1442` installer default-bin sweep: broad generated docs/installer surface; better as a dedicated npm installer PR if needed.
  - `b931fb9fd` CI auto-tag workflow and `6d2cb5980` patch-manifest normalization: useful, but separate infrastructure/catalog-maintenance surfaces.
  - `c02ac8c09` Chrome profile selection: broad multi-CLI auth update; only the X Articles user-id dependency was adapted here.
- Validation planned/run on branch `upstream-sweep/20260606-x-twitter-articles`: `git diff --check`, `python3 .github/scripts/verify-skill/verify_skill.py --dir library/social-and-messaging/x-twitter/`, `go test ./...`, `go build ./...`, `go vet ./...`, and `govulncheck ./...` from the X/Twitter CLI root.
- Policy: no public push; branch/PR target private fork `cathrynlavery/printing-press-library` only.

## 2026-06-15T17:01:45Z — WaveSpeed CLI 2026.6.2 private sync

- Repo/path: `/Volumes/Cat SSD/Developer/PrintingPress/printing-press-library-private-wavespeed`.
- Private target: `cathrynlavery/printing-press-library-private` (`PRIVATE`, default branch `main`).
- Public upstream inspected: `mvanhorn/printing-press-library` (`upstream/main` at `2a98659ce5ec544408a1f5b27f58aa6240e4e00b`); push URL remained disabled and no push was made to mvanhorn.
- Finding: private `main` (`7785f5a66d397a3f9b7b96e3e0fa981c27b182cf`) was missing WaveSpeed release metadata/version `2026.6.2` and latest WaveSpeed CLI refresh files.
- Mutation: created branch `sync/wavespeed-2026-6-2-private`, synced `library/ai/wavespeed` from `upstream/main`, committed `2361518d6` (`feat(wavespeed): sync 2026.6.2 CLI refresh`), and pushed only to `git@github.com:cathrynlavery/printing-press-library-private.git`.
- PR: `https://github.com/cathrynlavery/printing-press-library-private/pull/17`.
- Verification:
  - `git diff --quiet upstream/main -- library/ai/wavespeed` — PASS (synced WaveSpeed tree matches upstream after the branch changes).
  - `go test ./...` from `library/ai/wavespeed` — PASS.
  - `go run ./cmd/wavespeed-pp-cli --help` — confirmed top-level commands include `brand`, `plan`, `qa`, `pack`, `batch`, `variants`, `compose`, `aspects`, `restyle`, and `library`.

## 2026-06-15T17:17:36Z — PR #17 review follow-up

- PR reviewed: `https://github.com/cathrynlavery/printing-press-library-private/pull/17`.
- Comments inspected: issue comment from `greptile-apps`, latest review, unresolved review threads, and status check rollup.
- Findings addressed:
  - Added `HOME` isolation to `TestRunDownloadFailureReportsPartialSuccesses` so partial-download state writes stay in a temp home.
  - Capped legacy `schemaHelpText` enum display at 20 values.
  - Narrowed `models --capability image-edit` text matching from any `edit` substring to image-specific phrases.
- Verification:
  - `git diff --check` — PASS.
  - `cd library/ai/wavespeed && go test ./...` — PASS.
- Policy confirmation: private branch only; no public/mvanhorn push.
