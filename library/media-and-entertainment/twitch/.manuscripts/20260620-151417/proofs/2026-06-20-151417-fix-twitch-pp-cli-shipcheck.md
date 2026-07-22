# Twitch CLI Shipcheck

Phase 4 sweep run on Windows, Go 1.26.4, against the vendored community spec
`spec.json` (OpenAPI 3.0, derived from DmitryScaletta/twitch-api-swagger; the
security scheme was rewritten from the `implicit` flow to `clientCredentials`
with token URL `https://id.twitch.tv/oauth2/token`, and Unicode smart
punctuation in descriptions was normalized to ASCII).

## Leg Results

| Leg | Result | Exit | Elapsed |
|-----|--------|------|---------|
| verify | PASS | 0 | 23.5s |
| validate-narrative | PASS | 0 | 2.9s |
| dogfood | PASS | 0 | 28.8s |
| workflow-verify | PASS | 0 | 0.5s |
| verify-skill | PASS | 0 | 7.8s |
| scorecard | PASS | 0 | 14.9s |

**Verdict: PASS (6/6 legs passed)**

## Verify Detail

100% pass rate (26/26 commands, 0 critical failures) in mock mode. The
data-layer commands (export, import, search, sync, tail, analytics) all pass
3/3. Note: verify must be run with no persisted credentials (`auth logout`);
a stale on-disk token causes the mock injection to be skipped and the probe to
attempt a live call.

## Scorecard Detail (94/100 — Grade A)

Honest weak spot: Insight scored 4/10 — `analytics` ships generic group-by
examples rather than Twitch-specific curated insights. Everything else
(output modes, auth protocol, doctor, agent-native, MCP design, local cache,
breadth, sync correctness, path validity) scored at or near full marks.

## Live Smoke (separate, real API)

Outside the mock-mode shipcheck, the `client_credentials` app-token flow was
exercised end-to-end against the real Twitch Helix API. With `TWITCH_CLIENT_ID`
/ `TWITCH_CLIENT_SECRET` supplied via environment variables (an app registered
at dev.twitch.tv/console), `auth login` minted and persisted an app access
token, and the following returned **200 OK, source: live** with real data:

- `games get-top --json` — top games (Just Chatting, Fortnite, Old School RuneScape, ...)
- `streams get --json` — live streams
- `sync` (default resources) — 349 records across games-top, streams,
  content-classification-labels, chat-emotes-global, eventsub-subscriptions;
  0 warnings, 0 errors
- `search fortnite --json` — matched synced stream data
- `analytics --type streams --group-by game_name --json` — real aggregation
  (Just Chatting 4, Fortnite 2, Overwatch 2, ...)

This confirmed the `Client-Id` header patch, the `client_credentials` mint, and
the full transcend pipeline (sync -> search -> analytics) against the live API.
The application client secret was rotated immediately after, so the formal
`dogfood --live --write-acceptance` run was not captured; see
`proofs/phase5-skip.json`. Credentials are not part of this artifact.

## Ship Recommendation: **ship**

6/6 legs pass, scorecard 94/100 Grade A, live `client_credentials` + read calls
+ full transcend pipeline verified against the real API. The disclosed
follow-up (user-scope sync resources require a user OAuth token) is documented,
not a blocker.
