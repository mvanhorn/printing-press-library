# Shipcheck — slack-pp-cli reprint

Run `20260801-150754-271fdf29`. Reprint of the published `slack` CLI (press v1.2.1, 2026-04-09) under press v4.29.0.

## Final leg results

| Leg | Result | Notes |
|---|---|---|
| verify | **FAIL** | 100% pass rate (69/69, 0 critical). Sole failure: mock-mode data pipeline, 0 rows. See Known Gaps. |
| validate-narrative | PASS | every README/SKILL command path, flag, and argument shape resolves |
| dogfood | PASS | 0 issues; novel features 7/7 planned = 7/7 found |
| workflow-verify | PASS | |
| apify-audit | PASS | |
| verify-skill | PASS | flag-names, flag-commands, positional-args, shell-var-quotes, unknown-command, canonical-sections |
| scorecard | PASS | **91/100, Grade A** |

Live dogfood (Phase 5, full level): **358/358 passed, 0 failed, 219 skipped.**

## Scorecard movement vs the published v1.2.1 CLI

| Dimension | v1.2.1 | This reprint |
|---|---|---|
| MCP Surface Strategy | 2/10 | **10/10** |
| MCP Tool Design | 5/10 | **10/10** |
| MCP Remote Transport | 5/10 | **10/10** |
| MCP Quality | 7/10 | 8/10 |
| Total | 86/100 | **91/100** |

Remaining sub-10 dimensions: Cache Freshness 5/10 (declined deliberately — Slack's tier rate limits make pre-read auto-refresh a footgun), Insight 7/10, Data Pipeline Integrity 7/10, Agent Workflow 9/10.

## Top blockers found and fixed

1. **Reserved-name collisions** — spec resources `search` and `auth` collide with framework commands; renamed `search_api` / `auth_api`. The v1.2.1 press had no such check, so the published CLI shipped with the latent collision.
2. **Novel-command collision the generator does not check** — the novel feature `recall` collided with the framework learn loop's own `recall`, producing two same-named root commands and breaking 7 framework tests. Moved to `archive recall`.
3. **Auth completely broken** — the secondary OpenAPI's OAuth2 `slackAuth` definition displaced the curated `bearer_token` model; `AuthHeader()` never read `SlackBotToken`, so every request went out unauthenticated (`not_authed`). Stripped security definitions from the contributing spec.
4. **Cross-spec duplication** — 150 tools = 62 + 88 with zero overlap detection. Deduplicated to 91.
5. **`ok:false` store corruption** — regression of a previously-patched defect; an error envelope was stored as a record and counted as a successful sync.
6. **No message-history sync path** — the generated syncer walks flat list endpoints only, so five of seven approved features read data nothing ever wrote. Hand-built `archive sync`.
7. **`auth.revoke` mislabelled read-only** — revoked the operator's live token during the matrix. See the acceptance report's Incident section.

## Known Gaps

**Mock-mode data pipeline reports 0 rows.** `verify` creates 53 domain tables but stores no rows when syncing against the generator's mock server, so the leg reports FAIL despite a 100% (69/69) command pass rate with 0 critical failures.

Evidence this is a harness limitation rather than a product defect:

- The same pipeline was exercised against the live Slack API and stored **29 conversations, 32 members, and 991 messages**.
- All seven novel features were verified against that live data, including a relevance assertion (every `archive recall "deploy"` hit contains "deploy") and a negative control (a nonsense query returns `[]`, not `null`, not unrelated rows).
- Isolated as pre-existing: the failure reproduces identically with every local patch disabled, and both before and after response schemas were added to the spec.

Adding `response`/`response_path` declarations for all seven syncable list endpoints, plus a `types:` block, did not change the mock outcome. Filed as a Printing Press issue; not fixable inside the printed CLI.

The operator was shown this evidence and chose to proceed with the gap documented.

## Verdict

**ship-with-gaps.**

Justification against the two required conditions: (a) the outstanding failure requires a fix in the generator's verify harness, which is outside this CLI and unavailable in-session; (b) it is documented here and in the generated README's Known Gaps section. Every other leg passes, live dogfood is 358/358, and the behavioural correctness the ship threshold demands — no approved feature returning wrong or empty output — was confirmed against real data for all seven.
