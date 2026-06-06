# Printing Press upstream sweeps

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
