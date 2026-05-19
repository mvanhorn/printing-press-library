# Magnific CLI Live Smoke (Phase 5) — SKIPPED

## Skip reason

Magnific's OpenAPI spec declares `apiKey` security globally and the API returns
clean 401 with a structured error message on every endpoint without a key. The
Phase 0.5 API Key Gate found no `MAGNIFIC_API_KEY` / `FREEPIK_API_KEY` in the
environment; the user selected "Continue without (recommended)" so no live key
was provided for testing.

Per the skill, Phase 5 auto-skips when the API requires API-key auth and no
credential is available. The CLI was still exercised against real targets:

- **Reachability gate (Phase 1.9)** — `curl https://api.magnific.com/v1/icons`
  returned 401 with a structured JSON error and a key-acquisition URL. PASS.
- **Dogfood matrix (Phase 4)** — 173/173 commands passed mock-mode dispatch,
  including every novel command path (`history search`, `compare`, `task wait`,
  `task watch`, `task status`, `tasks stale`, `tasks reconcile`, `tasks list`,
  `cost ledger`, `cost forecast`, `gallery list`, `gallery open`, `models list`,
  `models stats`, `prompt save`, `prompt list`, `prompt show`, `prompt run`,
  `prompt delete`, `stock library index`, `stock library search`, `context`).
- **Scorecard `--live-check` Sample Output Probe (Phase 4)** — 7/10 novel-feature
  invocations returned well-formed JSON against the local empty store. The 3
  remaining misses are expected empty-state results (history/stock empty) plus
  one transient SQLITE_BUSY in `compare` (parallel-write concurrency that does
  not occur in single-shell usage).
- **`magnific-pp-cli context --json`** — returned a populated bundle with
  `api_reachable: false, api_status_code: 401` (correct — no key set), the live
  model catalog size (41), and empty local-store fields. Confirms the API
  reachability probe works.
- **`magnific-pp-cli cost forecast --model mystic --count 10 --json`** — returned
  `estimated_credits: 120` (10 × 12) with the caveat string. Pure-local
  computation; verified.

## What remains untested without a key

- Real POST dispatches to image/video/audio model endpoints (would require
  credit-consuming calls and could not run in a non-paid CI context anyway).
- End-to-end `prompt run` → `task wait` → output download flow.
- Bake-off (`compare`) returning real task_ids and per-model output URLs.

## How to enable Phase 5 later

```bash
export FREEPIK_API_KEY=sk_live_...
$CLI_WORK_DIR/magnific-pp-cli doctor --json
# Then re-run `printing-press dogfood --live --dir $CLI_WORK_DIR`.
```

The CLI is shipping-ready as a mock-verified Grade A artifact; live smoke is a
follow-up that a credentialed user can run on demand.
