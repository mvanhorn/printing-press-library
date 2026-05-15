# Printing Press Library Lessons

Date: 2026-05-14

Use this file for reusable lessons learned while turning Squarespace into a Printing Press library package. Keep one-off live account evidence in `docs/research/`; keep durable implementation rules here.

## What Belongs Here

- Endpoint maps discovered from browser/CDP, HAR, app bundle, or network traces.
- Auth/session mechanics that are not in the public API docs.
- ID resolution rules, especially when public names must be converted into internal IDs.
- Generator limitations and local patches that should either stay documented or move upstream.
- Review and acceptance lessons from Printing Press verifier, Greptile, and accepted packages.
- Safety rules for future write support.

Do not store raw cookies, tokens, account-specific domain IDs, website IDs, subscription IDs, or billing identifiers here. Redact them or replace them with placeholders like `<domain-id>`.

## Endpoint Map Pattern

For dashboard or undocumented APIs, record endpoints as a table with:

| Field | Meaning |
| --- | --- |
| Surface | Public Commerce API, Account dashboard API, site editor API, billing API, etc. |
| Method/path | Redacted endpoint path. Use placeholders for internal IDs. |
| Required auth | Bearer token, account cookie, CSRF header, or browser-only session. |
| Input resolver | How a caller gets required IDs without hardcoding account-specific values. |
| Output shape | Key fields needed by the CLI or agent. |
| CLI command | Command that uses the endpoint. |
| Verification | Live smoke, mock verifier, unit test, or browser/API comparison. |
| Risk | Read-only, write, billing-affecting, DNS-affecting, destructive. |

Example:

| Surface | Method/path | Required auth | Input resolver | CLI command | Verification | Risk |
| --- | --- | --- | --- | --- | --- | --- |
| Account domains | `GET /api/account/1/domains/byName/<domain-name>` | `SQUARESPACE_ACCOUNT_COOKIE(_FILE)` | User passes `--name`; response supplies `<domain-id>`, `<website-id>`, `<contract-id>` | `account domain get --name` | Browser API and CLI live smoke | Read-only |
| Account DNS | `GET /api/account/1/domains/<domain-id>/custom-record-set` | Account cookie | Resolve `<domain-id>` through `byName` first | `account domain custom-records --name` | Browser API and CLI live smoke | Read-only now; write support would be DNS-affecting |
| Billing terms | `GET /api/account/1/billing/websites/<website-id>/contracts/<contract-id>/validTerms?domainName=<domain-name>` | Account cookie | Resolve website and contract IDs from domain lookup payload | `account domain billing-valid-terms --name` | Browser API and CLI live smoke | Billing read |

## Cookie Session Rules

- Account dashboard commands are runtime-cookie based and must not commit cookies or account IDs.
- Prefer `SQUARESPACE_ACCOUNT_COOKIE_FILE` over a literal env var so secrets do not end up in shell history.
- Always construct account-dashboard requests with context and timeout support.
- Treat login redirects, HTML interstitials, and 401/403 responses as auth/session failures, not empty data.
- Redact token/session/cookie/secret-shaped JSON keys before output.
- Keep dashboard commands read-only until write endpoints have explicit confirmation, dry-run behavior, and rollback notes.

## ID Resolution Rules

The CLI should be domain-name first and internal-ID agnostic:

1. User passes `--name example.com`.
2. CLI calls `GET /api/account/1/domains/byName/example.com`.
3. CLI extracts the current domain ID, website ID, and contract/subscription ID from the live payload.
4. Follow-on commands use those values only in memory.
5. Docs and tests use placeholders, never real account IDs.

This is the rule that keeps the package usable by any account and prevents leaking one user's domain identifiers into the library.

## Printing Press Acceptance Lessons

Accepted packages in `library/commerce` generally include the same top-level shape:

- `manifest.json` with `manifest_version: 0.3`, binary MCP server entry point, user config, license, and platform compatibility.
- `SKILL.md` with install, auth, agent-mode, command, and troubleshooting guidance.
- `README.md`, `LICENSE`, `NOTICE`, `Makefile`, `.goreleaser.yaml`, `.golangci.yml`, `.printing-press.json`, and patch metadata when hand edits exist.
- Generated CLI and MCP entry points under `cmd/`.
- Runtime verification output such as `dogfood-results.json` and/or `workflow-verify-report.json`.
- Focused tests for hand-written or patched behavior.

Verifier signals are useful but not sufficient by themselves. For this package, review also required checking that:

- Account commands resolve IDs dynamically rather than baking in user-specific values.
- Cookie auth is runtime-only and sensitive output is redacted.
- Greptile findings are fixed in code and covered by focused tests.
- The standalone GitHub repo mirrors the package fixes.

## Greptile Findings Converted To Rules

The PR review found useful general rules:

- If a flag is advertised, test the behavior. `tail --follow=false` needed an explicit single-poll return path.
- If a filter flag exists, it must actually filter. `search --type` needed a typed FTS query rather than falling through to global search.
- Every outbound HTTP path should carry context cancellation and timeout behavior, including delivery/webhook helpers outside the main API client.
- When Greptile auto-review does not attach to a new SHA, a manual `@greptileai review` comment from a user account can create the missing review check; the workflow bot's auto-nudge may not be enough.

## Where To Put New Discoveries

- `docs/research/<topic>.md`: raw discovery notes, browser traces, current evidence, live-smoke results, redacted API payload observations.
- `docs/printing-press-library-lessons.md`: durable rules and patterns that should survive regeneration or be reused for the next CLI.
- `.printing-press-patches.json`: short index of intentional generated-code patches.
- `README.md` and `SKILL.md`: user-facing install/auth/usage rules only, not raw research.

Every finding doc should include what was found, how it was found, why it matters, raw redacted evidence, and how to reproduce it.
