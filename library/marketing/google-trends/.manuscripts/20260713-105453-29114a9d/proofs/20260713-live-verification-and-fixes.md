# Live Verification & Bug Fixes — google-trends-pp-cli

Written after fixing 3 real bugs found via live testing, to explain why the
scorecard's automated sample-output probe still shows 4/7 pass despite every
flagship command being manually confirmed working. **Read this before treating
a future shipcheck's 4/7 probe result as evidence of a regression.**

## Summary

Two bugs blocked the entire core explore→widget-token chain (the foundation
of `trends interest`, `trends region`, `trends related`, and every novel
command that consumes their cached data). A third, isolated bug affects only
`trends trending`. All three were found and diagnosed via live calls against
the real API, not guessed.

## Bug 1: `req` sent in POST body instead of query string

**File:** `internal/gtrends/explore.go`, `Explore()`

The generated client sent `POST /trends/api/explore` with `hl`/`tz` as query
params and `req` as a `application/x-www-form-urlencoded` body field. A real
browser capture (`discovery/browser-sniff-capture.har`) showed the working
request puts `hl`, `tz`, and `req` **all in the query string**, with an
**empty POST body**. Every live call using the old shape returned HTTP 500.

Fix: build `req` into the `params` map and call `client.PostWithParams(...,
nil)` instead of `PostFormWithParams`.

## Bug 2: XSSI prefix stripping didn't handle the comma variant

**File:** `internal/client/client.go`, `sanitizeJSONResponse()`

Google's anti-JSON-hijacking prefix is `)]}',` (includes a trailing comma)
for widget/picker responses, but `)]}'` (no comma) for `/api/explore`. The
generated `sanitizeJSONResponse` only checked `)]}'\n` and `)]}'` — since
`)]}'` is itself a byte-prefix of `)]}',`, it matched first and stripped only
4 bytes, leaving a stray leading comma. Every widget/picker response then
failed `json.Unmarshal` with `invalid character ',' looking for beginning of
value`.

Fix: check the comma variants (`)]}',\n`, `)]}',`) before the non-comma ones,
longest-first. **This is a bug in the printing-press generator's own
`sanitizeJSONResponse` template, not something specific to this CLI** — it
should be fixed upstream so future-generated CLIs against similarly-XSSI-
protected APIs don't hit the same failure.

## Bug 3: `trends trending` calls the wrong RPC (partially fixed, now a known gap)

**File:** `internal/gtrends/trending.go`

The original implementation called batchexecute rpcid `DqDTgb`, which — live
verification showed — returns the page's **geo-picker dropdown** (a list of
every country, e.g. `["AL","Albania","albania"]`), not trending search terms.
The real trending-terms RPC is `Tnt4U` (also present in the HAR capture,
previously unexamined), which takes an empty `[]` request body — the geo
scoping lives entirely in the session's `f.sid`/`bl` tokens, themselves
scraped from the `/trending?geo=<geo>` page load.

Fixed the rpcid and request shape to match `Tnt4U` exactly, and added the
`Referer`/`X-Same-Domain` headers a real browser sends. Despite matching the
captured request shape and headers, the live server still returns an
explicit `["e", 4, null, null, ...]` **error frame** instead of data — this
is a real, currently-unresolved gap, most likely session/bootstrap state this
CLI isn't replicating (possibly something only a live, JS-executing browser
session establishes).

**Decision:** rather than keep guessing at undocumented protocol internals,
made the parser fail loudly (`ErrTrendingParseFailed`) on an error frame
instead of falling through to a low-confidence heuristic scraper that was
silently emitting garbage (structural tokens like `"di"`, `"af.httprm"`
mistaken for trending terms). Removed `trends trending` from the README
quickstart, replaced it with `trends related`, and documented the gap
explicitly in the Troubleshooting section. This matches the original
research's own pre-flagged contingency for this specific surface
("recommend hand-coding... or shipping as a documented v1 gap").

## Live verification performed (2026-07-13, this session)

All of the below were run against the real `https://trends.google.com` API,
from a **freshly wiped** `~/.local/share/google-trends-pp-cli/` (no cookies,
no cached data) — i.e. the true first-run experience:

| Command | Result |
|---|---|
| `trends interest coffee --geo US` | ✅ real interest-over-time data |
| `trends region coffee --geo US` | ✅ real per-state values |
| `trends related coffee --geo US` | ✅ real related/rising terms |
| `trends geo-gap "electric vehicle" hybrid --geo US` | ✅ real per-region deltas (took ~6s under adaptive rate-limit backoff) |
| `trends history search "electric vehicle"` | ✅ 20 real matches once `trends related` had synced that keyword |
| `trends opportunities "meal prep"` | ✅ real ranked content ideas once `trends related "meal prep"` had synced |
| `trends trending --geo US` | ❌ known gap — see Bug 3 above |
| `trends pickers-geo` / `trends pickers-category` | ✅ (no-auth, always worked) |

`go build`, `go vet ./...`, and `go test ./...` all pass after every fix.

## Why the scorecard's automated sample probe still shows failures

The scorecard's built-in live sample probe (`shipcheck` → `scorecard` leg)
runs each novel command **once**, independently, with fixed sample keywords
("electric vehicle", "meal prep"), against whatever local-store state exists
at that moment — it does not pre-sync the keyword it searches for, and its
per-command timeout does not account for Google's adaptive rate-limit
backoff compounding across a burst of prior calls in the same run. Two of
its three reported failures (Trend History Search, Content Opportunity
Ranking) are **cold local-store artifacts**: those are local-only FTS
searches over data that only exists after a `trends related`/`trends
trending` sync for that exact keyword — the probe searches before syncing.
The third (Geo-Divergence Finder) is a **timeout artifact**: it needs two
live round-trips and can take 5-8s under Google's throttling, longer than
the probe's fixed window, especially right after other live calls in the
same run adjusted the adaptive limiter down.

All three were manually reproduced with adequate timeout and pre-synced data
above and are confirmed correct. This is a probe-design limitation (no
pre-sync step, fixed timeout not scaled to rate-limit state), not a CLI
defect. If reproducing shipcheck later, don't treat 4/7 as a regression
signal for these three specific commands without first checking local-store
state and rate-limit timing.

## `auth_protocol` scorecard dimension (2/10) — why it's left as-is

The scorecard's Domain Correctness → Auth Protocol dimension scores 2/10.
This is not being dismissed as noise: verified there is no generator source
checkout available in this environment to inspect the scoring rule directly
(installed via `go install ...@latest`, no vendored source, no module-cache
copy present), so the exact check couldn't be read. Reasoning from what's
empirically true instead:

- The spec (`auth.type: cookie`) is not wrong, but the live evidence this
  session gathered goes further than the spec states: the explore/widget API
  works anonymously through this CLI's Chrome-TLS-fingerprinted transport
  with **zero cookies present** (confirmed by deleting `cookies.json`
  entirely and re-testing — see Live Verification table above). A cookie
  from `auth login --chrome` is not required for baseline function; it may
  help under sustained heavy-use rate-limiting, but that's an optimization,
  not a protocol.
- Google Trends has no formal auth protocol (no API key, no OAuth, no Basic/
  Bearer header) — the actual gate is an anti-bot TLS/HTTP fingerprint check,
  not a credential scheme. A scorer looking for a recognizable auth
  mechanism (bearer/basic/apikey/oauth2) will correctly find none, because
  none exists for this API. A 2/10 here plausibly reflects that structural
  reality rather than a defect in this CLI.
- Changing the spec's declared `auth.type` and regenerating was considered
  and rejected: it risks re-triggering the exact regen-wiring-loss issue
  already hit twice this session (hand-written `AddCommand` calls in
  `trends.go`), for a scorecard sub-dimension, not a functional bug. Not
  worth the risk given everything else already verified working.

## What must survive future regeneration

`internal/gtrends/explore.go` and `internal/client/client.go` both carry
hand-fixes on top of generated code. `client.go` is marked DO-NOT-EDIT and
would be silently overwritten by a future `generate --force` (the same
regen-gap that has already eaten hand-written `AddCommand` wiring twice this
session — see `trends.go`'s inline comment). If this spec is regenerated
again, **Bug 2's fix in `sanitizeJSONResponse` must be manually reapplied**,
or better, fixed in the printing-press generator template itself so it
doesn't need reapplying.
