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
