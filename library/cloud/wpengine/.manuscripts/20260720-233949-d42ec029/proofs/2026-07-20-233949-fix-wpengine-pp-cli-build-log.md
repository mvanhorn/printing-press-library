Manifest transcendence rows: 7 planned, 0 built. Phase 3 will not pass until all 7 ship.

# Build Log — wpengine-pp-cli

## Phase 3 complete: 7 planned, 7 built.

Built (all hand-code, ~120-230 LoC each):
1. audit certs — local join installs↔ssl_certificates; verified vs real fleet (found certs expired 700-1400 days)
2. audit backups — latest-completed-per-install join; real fleet: 0 stale prod (daily auto-backups)
3. audit versions — distribution + --php-below + --drift w/ site-name resolution from sites table; real: 7 installs on PHP 7.4, 4 sites with prod/staging drift
4. audit domains — unverified/missing_cert/dangling/duplicates; platform domains (*.wpengine.com, *.wpenginepowered.com) excluded from missing_cert by default (--include-platform to include)
5. guard — live backup→poll→purge gate; IsVerifyEnv short-circuit, IsDogfoodEnv poll curtail, name→UUID resolution via mirror, notification email default from /user
6. whois — 4-table local join; verified vs real domain (resolved install/site/account/cert); negative test exit 3
7. audit usage — live limits+summary projection; string-encoded numbers handled via cliutil.ExtractNumber; real finding: account projected over bandwidth limit; fetch_failures + stderr warning on partial failure

Fixes during build: wp_version null in API list responses (distribution shows "unknown" honestly); drift site names resolved from sites table (install payload only carries site id); platform-domain noise filter.

Data-source annotations: local ×5 (reject --data-source live), live ×2 (reject --data-source local). Typed exits verified: usage 2, notfound 3, api 5.
Unit tests: audit_helpers_test.go (versionBelow, certNameCovers, decodeRedirectTargets, parseAPITime — 11 tests inc. wiring smokes, all pass).
