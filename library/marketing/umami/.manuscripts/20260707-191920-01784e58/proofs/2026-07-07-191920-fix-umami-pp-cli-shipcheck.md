# umami-pp-cli — Shipcheck Report

## Final verdict: **ship** (pending Phases 4.8-5)

## Legs (final run)
| Leg | Result |
|---|---|
| verify | PASS (auto-fix loop, 12.1s) |
| validate-narrative (strict, full-examples) | PASS |
| dogfood | PASS (0 issues; novel 6/6 planned=found) |
| workflow-verify | PASS |
| apify-audit | PASS |
| verify-skill | PASS (after realtime recipe fix) |
| scorecard | PASS — **98/100 Grade A** |

## Scorecard highlights
- 10/10 on: output modes, auth, error handling, terminal UX, README, doctor, agent-native, MCP remote transport/tool design/surface strategy, local cache, cache freshness, breadth, vision, workflows, insight, path validity, auth protocol, data pipeline, sync correctness.
- 8/10 MCP quality; 9/10 agent workflow; 4/5 type fidelity.
- Omitted (N/A): mcp_description_quality, mcp_token_efficiency, live_api_verification.
- MCP surface: 111 tools (Cloudflare pattern: thin search+execute pair, endpoint mirrors hidden), readiness: full.

## Live sample probe
5/6 pass. The remaining "failure" is a probe heuristic: `new-referrers restodom.fr --agent` — --agent implies --compact which strips the echoed `site` field, so the query token doesn't appear verbatim in output. Functionally verified live (returns real first-seen referrers: search.lilo.org, youtube.com...). Not a functional bug.

## Fix loop history (before → after)
- Loop 1: verify-skill FAIL (realtime recipe used `realtime get <id>`; promoted realtime is a leaf `realtime <id>`) → research.json recipe fixed, regen. Also: unregistered generated command `auth set-token` wired by hand (generator bug); v3 report runners need `filters` object always present → hand decorator; snapshot needed own --max-runtime.
- Loop 2: 7/7 PASS.

## Phase 4.7 sync-param-drop gate
Skipped — vendor-authored spec run, no traffic-analysis.json (no browser-sniff phase).

## Behavioral spot checks (live instance, real data)
- portfolio: 18 sites, growth deltas, 0 failures
- seo restodom.fr: 91.8% organic, 88.8% Google share, real entry pages
- movers: real risers/fallers incl. disappeared rows
- coverage: found a genuinely dead duplicate tracker (dindonstudio.fr entry silent 148d)
- watch: 16 sites scanned, 7 real deviations vs 28d weekday baselines
- new-referrers: real first-seen referrers after snapshot history
- pace: correct projections incl. edge case (new site, tiny prior month)
- reports run-breakdown/run-utm: real data with auto-injected filters
- --period decorator on generated commands; --qualified strips 16/20 bounce sessions

## Known gaps
None blocking. `auth login` stores the opaque v3 token; auto-refresh before expiry is impossible client-side (token is server-encrypted, not a decodable JWT) — re-login on 401, documented in troubleshooting.
