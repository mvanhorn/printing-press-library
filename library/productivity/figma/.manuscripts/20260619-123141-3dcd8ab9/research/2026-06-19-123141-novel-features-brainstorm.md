# Figma CLI — Novel Features Brainstorm (reprint, v4.2.0 → v4.24.0)

Subagent: general-purpose (sonnet). Reprint reconciliation (Pass 2(d)) active against prior research.json (2026-05-09). All 8 prior features re-scored against current personas; all kept.

## Customer model

### Persona A — Mia, AI codegen agent driver
- **Today:** feeds raw 8–15 MB `/v1/files/{key}` blobs to an LLM via a brittle custom Python compaction script that breaks on new node types.
- **Weekly ritual:** Monday batch component generation; 30–45 min/sprint debugging bad compaction (invisible frames, unresolved instance overrides, raw variable IDs).
- **Frustration:** no portable, versioned compaction pipeline callable from CI/Makefile/shell. GLips runs only as a Claude Desktop MCP.

### Persona B — Rin, design-system maintainer
- **Today:** tracks token changes by eye in the Figma variables panel; compares against last committed tokens.json; clicks Analytics per-entity to find unused components.
- **Weekly ritual:** Thursday pre-release token scan + commit; monthly half-day zero-usage component audit; biweekly stale-comment triage opening files one at a time.
- **Frustration:** no automated Figma-version token diff with mode-awareness, no team-wide orphan finder, no CI drift gate. Everything analytical requires the browser.

### Persona C — Diego, design ops engineer
- **Today:** quarterly design-hygiene via per-task Python notebooks, each with its own auth/pagination/retry.
- **Weekly ritual:** Tuesday comments notebook for 3 big files → spreadsheet; Thursday icon export; monthly manual library-analytics CSV merge in Excel.
- **Frustration:** notebooks unmaintainable, non-composable; no single cross-file chain "find files → fetch comments → filter by age → group by author → export."

### Persona D — Priya, Figma plugin / design-tools engineer
- **Today:** re-triggers Figma events by hand to test webhook handlers; copies failed-delivery payloads from the dashboard into Postman to re-send.
- **Weekly ritual:** 3–5 manual event re-triggers per webhook ship; 20 min per production failure investigation.
- **Frustration:** `/v2/webhooks/{id}/requests` exists but nothing exposes it with filtering + replay; no deterministic DS-surface drift check in CI.

## Candidates (pre-cut)

13 candidates generated (C-01..C-13). C-01..C-08 map to the 8 prior features (source (d) prior-keep + persona/DeepWiki). C-09 (MCP thin surface) and C-10 (native PAT auth) recognized as spec-level enrichments (table-stakes), excluded from the novel pool. C-11 (analytics heat map), C-12 (branch diff), C-13 (code-connect status) generated fresh from sources (b)/(c).

## Survivors and kills

### Survivors (8, all hand-code, all >= 8/10)

| # | Feature | Command | Score | Buildability | Persona | Long Description |
|---|---------|---------|-------|--------------|---------|------------------|
| 1 | Compaction-aware frame extract | `frame extract` | 10/10 | hand-code | A | none |
| 2 | Dev-mode resource bundle | `dev-mode dump` | 8/10 | hand-code | A | redirect vs frame extract |
| 3 | Cross-file comments audit | `comments-audit` | 9/10 | hand-code | C | none |
| 4 | Orphans finder | `orphans` | 9/10 | hand-code | B | none |
| 5 | Tokens diff between file versions | `tokens diff` | 9/10 | hand-code | B | none |
| 6 | Deterministic file fingerprint | `fingerprint` | 8/10 | hand-code | B/D | none |
| 7 | Webhook delivery replay | `webhooks test` | 8/10 | hand-code | D | none |
| 8 | Variable usage tracer | `variables explain` | 8/10 | hand-code | B | redirect vs variables local / orphans |

### Killed candidates

| Feature | Kill Reason | Closest Surviving Sibling |
|---------|------------|--------------------------|
| C-11 analytics heat map | inverse of orphans; no weekly ritual not already served | orphans |
| C-12 branch diff | scope creep; Rin's diff need served by tokens diff; branch_data is a flag on absorbed files get | tokens diff |
| C-13 code-connect status | 6/10; three-way overlap with orphans + dev-mode dump; no research backing | dev-mode dump |

## Reprint verdicts

All 8 prior features = **Keep** (commands and scope unchanged). `dev-mode dump` and `variables explain` gained Long Description redirects this pass to disambiguate sibling commands.
