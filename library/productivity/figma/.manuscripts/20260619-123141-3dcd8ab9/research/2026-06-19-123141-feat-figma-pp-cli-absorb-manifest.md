# Figma CLI — Absorb Manifest (reprint v4.2.0 → v4.24.0)

Reuses the prior published manifest's absorbed surface (47 rows, every Figma REST endpoint). Transcendence rows re-validated by the Phase 1.5c.5 subagent: all 8 prior features kept, all `hand-code`, all >= 8/10. C-13 (code-connect status) killed.

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---|---|---|---|
| 1 | Get file (full tree) | DC | `figma-pp-cli files get` | (generated endpoint) files get |
| 2 | Get file by node ids | DC | `(generated endpoint) files nodes` | accepts `1234:5678` and `1234-5678` |
| 3 | Get file metadata only | DC | `(generated endpoint) files meta` | Tier-1-light |
| 4 | List file versions | DC | `(generated endpoint) files versions` | cursor-paginated |
| 5 | Render nodes PNG/SVG/PDF/JPG | DC,G | `(generated endpoint) files render` | kick-render+poll |
| 6 | Get image-fill URLs | DC | `(generated endpoint) files image-fills` | 14-day expiry warn |
| 7 | List comments | DC | `(generated endpoint) comments list` | |
| 8 | Post comment | DC | `(generated endpoint) comments post` | |
| 9 | Delete comment | DC | `(generated endpoint) comments delete` | --confirm |
| 10 | Comment reactions CRUD | DC | `(generated endpoint) reactions` | |
| 11 | List team projects | DC | `(generated endpoint) teams projects` | |
| 12 | List project files | DC | `(generated endpoint) projects files` | |
| 13 | List team components | DC | `(generated endpoint) components team` | |
| 14 | List file components | DC | `(generated endpoint) components file` | |
| 15 | Get component by key | DC | `(generated endpoint) components get` | |
| 16 | Component sets (3) | DC | `(generated endpoint) component-sets` | |
| 17 | Styles (3) | DC | `(generated endpoint) styles` | |
| 18 | Variables: local | DC | `(generated endpoint) variables local` | Enterprise-gated |
| 19 | Variables: published | DC | `(generated endpoint) variables published` | |
| 20 | Variables: write | DC | `(generated endpoint) variables write` | bulk patch |
| 21 | Dev resources CRUD | DC | `(generated endpoint) dev-resources` | |
| 22 | Webhooks v2 CRUD | DC | `(generated endpoint) webhooks` | strips v2 |
| 23 | Webhook request log | DC | `(generated endpoint) webhooks requests` | feeds replay |
| 24 | Activity logs | DC | `(generated endpoint) activity-logs` | OAuth-only |
| 25 | Analytics — components | DC | `(generated endpoint) analytics components` | Enterprise |
| 26 | Analytics — styles | DC | `(generated endpoint) analytics styles` | |
| 27 | Analytics — variables | DC | `(generated endpoint) analytics variables` | |
| 28 | Whoami / me | DC | `(generated endpoint) me` | |
| 29 | oEmbed lookup | DC | `(generated endpoint) oembed` | public |
| 30 | Payments status | DC | `(generated endpoint) payments` | |
| 31 | Developer logs | DC | `(generated endpoint) developer-logs` | |
| 32 | OAuth login flow | DC | `(behavior in figma-pp-cli auth login) OAuth PKCE` | |
| 33 | PAT setup | DC | `(behavior in figma-pp-cli auth set-token) native X-Figma-Token` | validates against /v1/files probe |
| 34 | Doctor probe matrix | DC | `figma-pp-cli doctor` | plan-tier + rate-limit headers |
| 35 | Bulk image download | G | `(behavior in figma-pp-cli files render) imageRef vs gifRef` | |
| 38 | Design tokens export | FM,TS | `(behavior in figma-pp-cli variables local) W3C/CSS/JSON output` | |
| 42 | Sync to local SQLite | NEW | `figma-pp-cli sync` | last_modified cursor |
| 43 | Local SQL queries | NEW | `figma-pp-cli sql` | cross-cutting joins |
| 44 | FTS5 search | NEW | `figma-pp-cli search` | full-text |
| 45 | Output formats | DC | `(behavior in figma-pp-cli files get) --json/--select/--csv` | agent-native |

## Transcendence (only possible with our approach)

| # | Feature | Command | Buildability | Why Only We Can Do This | Long Description |
|---|---------|---------|--------------|------------------------|------------------|
| T1 | Compaction-aware frame extract | frame extract | hand-code | Ports GLips `simplifyRawFigmaObject` (pre-order walk + 5 transformers + style dedup) into a portable Go binary; fuses node tree + in-scope variables + dev-resources; reports simplifiedNodeCount. Score 10/10. | none |
| T2 | Dev-mode resource bundle | dev-mode dump | hand-code | Fuses dev-resource links + variables-in-scope + render permalink + Code Connect for one node into a portable MD/JSON bundle; official Dev Mode MCP is closed-source + Desktop-bound. Score 8/10. | Use this command for the full Dev Mode context of a single node as a self-contained bundle. Do NOT use it for frame-level codegen prompts; use 'frame extract' instead. |
| T3 | Cross-file comments audit | comments-audit | hand-code | Local SQLite aggregate over synced comments across every team file with age + group-by; `figma comments list` is per-file only. Score 9/10. | none |
| T4 | Orphans finder | orphans | hand-code | Local join of team-library publish list with library-analytics usage data → zero-usage entities; analytics UI is per-entity-per-file only. Enterprise-gated. Score 9/10. | none |
| T5 | Tokens diff between file versions | tokens diff | hand-code | Snapshots variables at two file versions with mode-awareness; no surveyed tool diffs Figma-internal versions (Tokens Studio diffs Git). Score 9/10. | none |
| T6 | Deterministic file fingerprint | fingerprint | hand-code | Stable SHA-256 of a file's token+component+style surface; exits non-zero on `--expect` mismatch for CI contract. Score 8/10. | none |
| T7 | Webhook delivery replay | webhooks test | hand-code | Pulls `/v2/webhooks/{id}/requests` and replays stored payloads (original headers) against an arbitrary target; Figma dashboard has no replay button. Score 8/10. | none |
| T8 | Variable usage tracer | variables explain | hand-code | Flat list of every node/component referencing a variable across a file from synced bindings; Figma UI shows references per-node modally only. Score 8/10. | Use this command to see every node binding a specific variable before rename/deprecation. Do NOT use it to list all variables (use 'variables local') or to find zero-usage variables (use 'orphans'). |

8 hand-code transcendence rows. 0 spec-emits. 0 stubs.

### Dropped this reprint (subagent kills, surfaced for override)
- code-connect status (C-13): three-way overlap with orphans + dev-mode dump; no research backing.
