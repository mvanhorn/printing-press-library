# Printing Press Retro: Creative Fabrica

## Session Stats
- API: creativefabrica (website-itself; no official API)
- Spec source: browser-sniffed → hand-authored internal YAML (Algolia search surface)
- Scorecard: 64/100 (Grade C — held 1 pt below the 65 floor by structural dims)
- Verify pass rate: 100% (26/26)
- Fix loops: 1 shipcheck
- Manual code edits: ~16 hand-authored files (entire command layer + Algolia client + snapshot store)
- Features built from scratch: 13 absorbed + 7 transcendence (all hand-built over a sibling Algolia client)

## Findings

### 1. The scorer under-credits hand-built CLIs (sibling clients + cobratree command trees) (Scorer bug)
- **What happened:** Two scorer signals fired falsely because both heuristics assume commands are
  spec-emitted endpoints calling the generated `internal/client`, but this CLI (a website-itself
  build) hand-builds every command over a sibling `internal/algolia` client and exposes them via the
  Cobra tree:
  - **Dogfood:** printed `6/7 novel features look reimplemented (no API client call, no store access)`
    for `find`/`deals`/`designer-stats`/`designer-compare`/`tags` — all of which DO call the live API
    via `c.Search(...)` in the sibling `internal/algolia` client (verified: 49/49 live dogfood passed).
    The heuristic only recognizes the generated `internal/client`/`internal/store`.
  - **Scorecard:** reported `MCP: 1 tools (1 public, 0 auth-required)` while the runtime MCP server
    actually exposes **15 tools** (confirmed via `tools/list`: find, deals, designer_stats, etc.). The
    count reads static spec endpoints (the 1 `products.search` passthrough), not the cobratree the MCP
    binary mirrors at runtime.
- **Scorer correct?** No (both are false negatives). The CLI genuinely calls the API and genuinely
  exposes 15 MCP tools.
- **Root cause:** scorer (dogfood `source_client_check`/novel-feature reimplementation heuristic; scorecard
  MCP-tool counter). Both enumerate from the generated `internal/client` + spec endpoints only.
- **Cross-API check:** Recurs on EVERY website-itself CLI and EVERY combo CLI — both are first-class
  SKILL paths whose commands are hand-built over sibling clients (`internal/source/<name>/`,
  `internal/recipes/`, `internal/phgraphql/`, `internal/algolia/`). SKILL Phase 3 principle #10 already
  documents `source_client_check` inspecting these sibling packages, so the machine already KNOWS the
  pattern exists — the reimplementation/MCP-count heuristics just don't consult it.
- **Frequency:** subclass:hand-built-client-CLIs (website-itself + combo) — a documented, first-class subclass.
- **Fallback if not fixed:** the agent must manually recognize and discount these false negatives every
  run (done here), and the false "reimplemented" signal inflates the apparent dead-code/quality gap,
  while the MCP undercount depresses breadth/MCP dimensions — both push the score down on CLIs that are
  actually correct.
- **Worth a fix?** Yes — it's a reproducible false negative on a documented subclass, and it actively
  misleads the score on otherwise-correct CLIs.
- **Inherent or fixable:** Fixable. The MCP counter can enumerate the cobratree (the same source the
  runtime MCP uses) instead of spec endpoints. The reimplementation heuristic can treat a call into any
  `internal/<pkg>` that itself performs outbound HTTP as an API call, the same set `source_client_check`
  already walks.
- **Durable fix:** (scorecard) count MCP tools from the cobratree walk, not static spec endpoints, for
  CLIs whose command tree exceeds the spec-endpoint set; (dogfood) when classifying a novel command as
  "reimplemented / no API call," also accept calls into sibling internal packages that perform outbound
  HTTP — reuse the package set `source_client_check` already enumerates.
- **Test:** positive — a website-itself CLI with N cobratree commands over a sibling client reports N
  MCP tools and 0 "reimplemented" novel features. negative — a spec-emitted CLI still counts spec
  endpoints and still flags a truly stubbed novel command.
- **Evidence:** dogfood JSON `issues[]` and scorecard `gap_report` from this run; runtime `tools/list`
  returned 15 tools vs scorecard's 1.
- **Related prior retros:** None (first retro on this machine).

## Prioritized Improvements

### P2 — Medium priority
| Finding | Title | Component | Frequency | Fallback Reliability | Complexity | Guards |
|---------|-------|-----------|-----------|---------------------|------------|--------|
| 1 | Scorer under-credits hand-built CLIs (sibling clients + cobratree) | scorer | subclass:hand-built-client (website-itself + combo) | agent must manually discount every run | medium | Only change enumeration source; spec-emitted CLIs unaffected |

### Skip
| Finding | Title | Why it didn't make it |
|---------|-------|------------------------|
| S1 | Scorecard structurally caps read-only/no-sync CLIs (vision/workflows/data_pipeline/sync = 0–5) below the 65 floor | Step G: case-against ("Steinberger bar deliberately rewards local-SQLite+sync; a read-only search CLI scoring lower is the bar working as designed") is roughly even with the case-for; cannot name 3 library APIs with evidence (no local library checked out). Real but debatable — let it resurface with stronger cross-API evidence. |

### Dropped at triage
| Candidate | One-liner | Drop reason |
|-----------|-----------|-------------|
| D1 | Generator emits dead `--allow-partial-failure` flag + ~13 unused pagination helpers | printed-CLI / subclass-only (minimal-spec CLIs); most CLIs use the scaffold |
| D2 | root.go `--help` Highlights lists `find` twice (two novel features map to one command) | printed-CLI cosmetic |
| D3 | No first-class generator shape for "single search/RPC endpoint, many logical commands" | unproven-one-off; hand-building over a sibling client is the documented workflow (aggregator-pattern ref) |

## Work Units

### WU-1: Scorer should credit hand-built CLIs (sibling clients + cobratree command trees) (from F1)
- **Priority:** P2
- **Component:** scorer
- **Goal:** Stop the dogfood reimplementation heuristic and the scorecard MCP-tool counter from
  producing false negatives on CLIs whose commands are hand-built over sibling internal clients and
  exposed via the Cobra tree.
- **Target:** dogfood `source_client_check` / novel-feature reimplementation classifier; scorecard MCP-tool counter.
- **Acceptance criteria:**
  - positive test: a website-itself CLI with M cobratree commands over an `internal/<pkg>` sibling client
    that performs outbound HTTP reports M MCP tools and 0 "reimplemented (no API call)" novel features.
  - negative test: a spec-emitted REST CLI still counts spec endpoints for MCP tools and still flags a
    genuinely stubbed novel command (one with no API/store/sibling-client call) as reimplemented.
- **Scope boundary:** Does not change scoring weights or N/A-eligibility of other dimensions; only changes
  the enumeration source for the MCP count and the call-site set the reimplementation heuristic accepts.
- **Dependencies:** none
- **Complexity:** medium

## Anti-patterns
- (none worth filing)

## What the Printing Press Got Right
- `probe-reachability` correctly classified the runtime as `browser_http` (Surf clears Cloudflare),
  and the agent correctly determined the actual DATA api (Algolia) is NOT behind Cloudflare — so the
  printed CLI shipped plain standard HTTP with the browser used only at discovery time. Clean separation
  of discovery transport vs runtime transport.
- The generator pre-scaffolded novel-feature command stubs from research.json, giving a correct
  skeleton to fill in.
- shipcheck + full live dogfood (49/49) + verify-skill + the code-review subagent caught real issues
  (mixed-type JSON fields, HTML entities, filter-injection escaping) before ship.
