<!-- slop-gate: off -->
# Polish: dropbox-pp-cli

Run on the working copy while the CLI is on HOLD (phase 18 gate FAIL, runner gap; issues #4952-#4956). Polish ran via printing-press-polish (forked), binary 4.33.2.

## Delta
- Scorecard 80 -> 86; dead_code 0/5 -> 5/5; MCP description quality 7/10 -> 10/10
- Verify (mock) 100% -> 100%; dogfood (static) WARN -> PASS
- gosec hand-written 70 -> 0 (41 remain in generated files)
- tools-audit 4 pending -> 0
- Live matrix: not exercised by polish (temp HOME, no creds)

## Fixes kept
- Removed 10 unused generated helpers from internal/cli/helpers.go (patch record .printing-press-patches/q-polish-dead-helpers.json) and an unused hand-written helper in dropbox_account.go
- Explicit `_ =` on 66 ignored Close() errors; 2 narrow #nosec with reasons (bound-parameter LIKE query, operator-supplied plan path)
- mcp-descriptions.json overrides for 4 tools, copied into tools-manifest.json and internal/mcp/tools.go by hand (mcp-sync on a scratch copy reset destructive hints on 8 tools and truncated 2 descriptions)
- overview: missing-index note no longer hides the quota failure reason; regression test added

## Reverted by orchestrator
- README.md / SKILL.md rewrite from the dogfood README sync: it dropped the creator byline, the client-id prompt note, and the doctor step. Restored to the committed versions.

## Orchestrator follow-ups
- Privacy: renamed two real top-level folder names used as test fixtures (internal/cli/conflicts_test.go, internal/dropbox/names_test.go) to neutral names.
- gofmt clean; go vet ./... exit 0; go test ./... exit 0 (test-polish.log, test-rename.log in the run dir).

## Retro candidates (Printing Press)
- Generated internal/platform/migration.go and cliutil/testenv helper lack the generated-file header; migration.go builds VACUUM INTO by string concatenation.
- mcp-sync drops hand patches in tools.go (destructive hints) and truncates descriptions.
- ~45 generated pagination helpers unused in an all-POST API; dogfood dead-code only checks top-level helpers.
- Dogfood README sync drops the Created-by byline and rewrites hand-authored sections.

ship_recommendation: hold (operator HOLD on phase 18 runner gap)
