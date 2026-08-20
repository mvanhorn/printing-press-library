# Novel Features Brainstorm — gisis-pp-cli v1

(Full Task subagent output, preserved for retro/dogfood debugging.)

## Customer model

**Persona A — Marta, sanctions compliance analyst at a commodity-trading desk**
- *Today (without this CLI):* When a chartering broker proposes a vessel for a West Africa loading, Marta opens the IMO GISIS web app in one tab, logs in (Cloudflare Turnstile + form auth), pastes the IMO into Ship Particulars, screenshots the page, then pastes name/flag/owner into her firm's KYC template. She also opens Equasis in parallel and a third tab for OFAC SDN search. She has no programmatic way to feed GISIS into her own scripts because there's no API.
- *Weekly ritual:* 5-20 vessel name/IMO checks per week as chartering deals come in; quarterly portfolio re-screens push that to 100+ vessels in a single afternoon.
- *Frustration:* GISIS times out her session every ~30 min; she re-Turnstiles, re-logins, re-types. The Ship Particulars page has no copy-as-JSON button — every field gets transcribed by hand or screenshotted.

**Persona B — Sam, due-diligence engineer building the openclaw-brain Vessel MCP orchestrator**
- *Today (without this CLI):* Sam is one engineer building the Phase 3 orchestrator MCP. To pull authoritative IMO particulars for a vessel he currently has nothing — no GISIS API, no existing CLI, just the web app. He's blocked on writing his own scraper or proceeding without authoritative registry data.
- *Weekly ritual:* Develops the orchestrator MCP daily; integration-tests it against 5-10 known IMOs per session; runs nightly batch tests over a watchlist of ~50 vessels.
- *Frustration:* He needs a tool that (a) returns the same JSON shape for the same IMO every time, (b) doesn't re-hit GISIS for an IMO he asked about an hour ago, and (c) is callable as both a CLI and an MCP tool with identical semantics. Today nothing exists.

**Persona C — Iris, investigative maritime journalist (OSINT, sanctions-circumvention beat)**
- *Today (without this CLI):* Iris gets a tip — a tanker called *Atlantic Pioneer* was photographed STS-transferring off Lomé. She doesn't have the IMO. She finds it via MarineTraffic, then crosses to GISIS for the authoritative flag/owner. She keeps a personal spreadsheet of every IMO she's ever queried, because GISIS doesn't have personal history.
- *Weekly ritual:* 2-3 deep investigations per week, each touching 5-30 vessels. Builds a story-by-story dossier per investigation.
- *Frustration:* Every fresh investigation, she re-queries vessels she already looked up 3 months ago for an unrelated story, because GISIS doesn't let her see her own search history and she's bad at maintaining her spreadsheet. She loses the as-of-date when the owner changed — GISIS shows current state only.

## Candidates (pre-cut)

(16 candidates evaluated; 7 kept inline, 9 reconsidered or killed in Pass 3 — see Survivors / Killed below.)

## Survivors and kills

### Survivors

| # | Feature | Command | Score | Buildability | Long Description |
|---|---------|---------|-------|--------------|------------------|
| 1 | Batch IMO lookup with throttled persistence | `ship batch --imos 9966233,9123456` or `ship batch --file imos.txt` | 9/10 | hand-code | Use this command to resolve many IMO numbers at once with the configured throttle. Do NOT use this command for a single IMO; use 'ship get' instead. |
| 2 | Local cache browse with FTS | `ship list [--flag X] [--owner Y] [--type Z] [--name-like Q] [--limit N]` | 7/10 | hand-code | Use this command to browse vessels you have already fetched. Do NOT use this command to fetch a vessel by IMO from GISIS; use 'ship get' instead. |
| 3 | Watchlist with selective refresh | `ship pin <imo> [--label]` / `ship unpin <imo>` / `ship refresh [--pinned] [--older-than 30d]` | 8/10 | hand-code | none |
| 4 | Cross-snapshot ship history (flag-hop detector) | `ship history <imo>` | 9/10 | hand-code | Use this command to see how a vessel's particulars have changed across the snapshots you have fetched. Do NOT use this command for a single current snapshot; use 'ship get' instead. |
| 5 | Stale-cache report | `ship stale [--older-than 30d] [--pinned]` | 7/10 | hand-code | none |
| 6 | Owner-fleet listing from accumulated cache | `owner fleet "ACME" [--exact\|--like]` | 8/10 | hand-code | Use this command to list cached vessels by owner. Do NOT use this command to fetch a fresh ship by IMO; use 'ship get' instead. Do NOT use this command to filter by flag or type; use 'ship list' instead. |
| 7 | Session liveness ping | `auth ping` | 8/10 | hand-code | none |

### Killed candidates

| Feature | Kill reason | Closest sibling |
|---------|-------------|-----------------|
| Cache-aware get with provenance (--max-age/--data-source) | Framework already provides; double-counts absorbed `ship get`. | absorbed `ship get` |
| Ship export (KYC table / markdown) | Pure formatting; `--json/--csv/--select/--compact + jq` cover it. | absorbed `ship get --json` |
| Flag-hop alerter daemon | Daemon scope creep; reframed as pin+refresh+history on user cron. | survivors #3 + #4 |
| Cross-source dossier | Out of scope — Phase 3 orchestrator owns cross-source. | (lives in orchestrator MCP) |
| LLM ship summary | LLM dependency; pipe absorbed `--json` to user's model of choice. | absorbed `ship get --json` |
| Sanctions cross-check | External service (OFAC); also Phase 3 orchestrator territory. | (lives in orchestrator MCP) |
| Doctor --explain | Enrichment of absorbed `doctor`; fold into implementation. | absorbed `doctor` |
| Ship card (markdown) | Duplicates killed export + absorbed `--json --compact`. | absorbed `ship get --json` |
| Vault-link emitter | Too project-specific; belongs in orchestrator. | (lives in orchestrator MCP) |
| Session keepalive daemon | Reframed to single-shot `auth ping` (survivor #7). | survivor #7 |
