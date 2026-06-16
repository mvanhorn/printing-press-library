# Printing Press Retro: KDP Niche Finder

## Session Stats
- API: kdpnichefinder (kdpnichefinder.com — Laravel/Inertia app, brand front over Artistly)
- Spec source: browser-sniffed (HAR) → hand-authored internal YAML
- Scorecard: 95/100 (Grade A, post-polish)
- Verify pass rate: 100%
- Live dogfood: 71/71 (full, authenticated session)
- Fix loops: 2 shipcheck + live-gate fixes
- Manual code edits: cookie-auth Cookie-header fix, CSRF header for writes, 8 Example fields, export --json, doc wording
- Features built from scratch: 7 novel (rank, drift, dupes, saturation, competitors, keywords, export) + refresh + niche HTML data-page fetcher

## Findings

### 1. Cookie auth: captured session jar is never sent on the live request path (Bug)
- **What happened:** With `auth.type: cookie`, `auth login --chrome` captured the session correctly and `doctor` passed, but every real command 401'd. The generated `internal/client/client.go` builds the HTTP client with a **nil cookie jar** (`newHTTPClient(timeout, nil)`), and the request path's cookie block is **comment-only** — it references a `LoadCookieJar` "sole source of outbound cookies" that does not exist anywhere in the generated module. So the captured cookie string (stored in `Config.AccessToken`) is never sent. Additionally, `Config.AuthHeader()` prepends `Bearer ` to the cookie string, which is wrong for a Cookie header.
- **Scorer correct?** N/A — functional bug, surfaced only in live testing (mock/dry-run/shipcheck all passed).
- **Root cause:** generator auth/client template — cookie/composed auth path never wires the captured jar onto outbound requests.
- **Cross-API check:** Recurs on every cookie/composed-auth CLI. Same root family as the env-var-Bearer miswiring already filed.
- **Frequency:** every cookie/composed-auth CLI.
- **Fallback if unfixed:** Only caught in live testing; runs without creds ship it silently broken. Agent fallback unreliable.
- **Worth a fix?** Yes — high blast radius, low detectability.
- **Durable fix:** For cookie/composed auth, either populate a real `http.CookieJar` in `client.New` from the stored cookies, or set the `Cookie` header from the raw cookie string (no `Bearer` prefix) on every request. (I hand-set the Cookie header from `Config.AccessToken` when `Jar == nil`.)
- **Test:** positive — a cookie-auth CLI with a captured jar sends `Cookie: ...` and a real GET returns 200; negative — a bearer/api_key CLI still sends `Authorization`, not `Cookie`.
- **Evidence:** `client.New` nil jar; doInternal lines ~487-497 comment-only block; live 401 → 200 after setting the header.
- **Related prior retros / issues:** **#2512** (`extends`) — #2512 reports the *env-var* cookie path sends `Bearer` not `Cookie`; this run shows the *`auth login --chrome` captured-jar* path is also broken (jar never attached), so the gap is broader than env-var auth.

### 2. Novel-feature stub commands omit `Example`, failing dogfood help-check (Template gap)
- **What happened:** The generator emitted novel-feature stubs (rank.go, drift.go, …) from `research.json.novel_features` with TODO bodies and **no `Example:` field**. dogfood's help-check requires examples; `drift` failed help-check (cascading skips) until I added examples to all 7.
- **Scorer correct?** Yes — dogfood is right to require examples; the generator should supply them.
- **Root cause:** novel-feature stub template ignores the `example` field that already exists on each `research.json.novel_features[]` entry.
- **Cross-API check:** Every CLI with novel_features.
- **Frequency:** every transcendence-bearing CLI.
- **Fallback if unfixed:** Agent adds examples during Phase 3 — but demonstrably forgets (the implementing subagent added none across all 7), so it surfaces late at dogfood.
- **Worth a fix?** Yes — the example string is already in research.json; populating the stub's `Example:` is free and raises the floor.
- **Durable fix:** In the novel-command stub template, set `Example: "<novel_features[].example>"` when present.
- **Test:** positive — a generated novel stub's `--help` shows an Examples section and passes dogfood help-check; negative — a novel feature with no `example` in research.json still compiles (no empty Example).
- **Evidence:** all 7 stubs lacked Example; drift help-check failed in live dogfood.
- **Related prior retros / issues:** **#2636** (`same`) — facet 3 "Novel / learn commands missing Examples". This run is a second sighting on a different API.

## Prioritized Improvements

### P1 — High priority
| Finding | Title | Component | Frequency | Fallback Reliability | Complexity | Guards |
|---|---|---|---|---|---|---|
| 1 | Cookie auth never sends captured Cookie header | generator | every cookie/composed CLI | low (live-only) | medium | bearer/api_key unaffected |

### P2 — Medium priority
| Finding | Title | Component | Frequency | Fallback Reliability | Complexity | Guards |
|---|---|---|---|---|---|---|
| 2 | Novel stubs omit Example | generator | every novel-bearing CLI | medium | small | skip when no example in research.json |

### Skip
| Finding | Title | Why it didn't make it |
|---|---|---|
| verify-skill flag scanner extracts `--chrome'` | flag tokenizer keeps trailing apostrophe from single-quoted prose | Step B: only 1 API with evidence; narrow; mitigated by using backticks in narrative |
| export CSV vs dogfood json_fidelity | CSV-only command fails when `--json` is appended | per-CLI design; resolved by making export honor --json (a fine per-CLI choice) |

### Dropped at triage
| Candidate | One-liner | Drop reason |
|---|---|---|
| base_url vs SSO login domain | kdpnichefinder.com SSO-redirects to app.artistly.ai with separate cookie jars | API-quirk |
| saturation publisher homogeneity | KDP books are nearly all "Independently published" so publisher-HHI saturates | API-quirk |
| stale response cache during debugging | folders list showed null from a cache entry written while auth was broken | iteration-noise (self-inflicted out-of-order debugging) |

## Work Units

### WU-1: Cookie/composed auth must attach the captured session to outbound requests (from F1)
- **Priority:** P1
- **Component:** generator
- **Goal:** A cookie/composed-auth CLI that captured a session via `auth login --chrome` actually sends it on every request.
- **Target:** generator auth/client templates (`internal/generator/`), client.go template.
- **Acceptance criteria:** positive — cookie-auth CLI GET returns 200 with a captured jar; negative — bearer/api_key CLIs still send `Authorization`.
- **Scope boundary:** Does not cover the Windows manual-cookie path (#2512 facet 2).
- **Dependencies:** Shares root with #2512.
- **Complexity:** medium
- **Disposition:** Comment on #2512 (extends its env-var finding with the captured-jar path).

### WU-2: Populate novel-stub `Example` from research.json (from F2)
- **Priority:** P2
- **Component:** generator
- **Goal:** Generated novel-feature stubs ship with an Example and pass dogfood help-check out of the box.
- **Target:** novel-command stub template.
- **Acceptance criteria:** positive — stub `--help` shows Examples and passes help-check; negative — no `example` → no empty Example line.
- **Scope boundary:** Example text only; not the TODO body.
- **Complexity:** small
- **Disposition:** Comment on #2636 (second sighting of facet 3).

## Anti-patterns
- None notable. The pipeline correctly gated on live testing, which is the only thing that caught the cookie-auth bug.

## What the Printing Press Got Right
- Browser-sniff → internal-spec → generate produced a clean 11-endpoint surface from a thin HAR.
- The novel-feature stub scaffolding (root.go wiring + constructor names + read-only annotations) made hand-implementation fast.
- shipcheck + live dogfood caught real issues (missing examples, export json-fidelity) before publish.
- `cookie` auth captured the right Chrome profile automatically once the session existed.
