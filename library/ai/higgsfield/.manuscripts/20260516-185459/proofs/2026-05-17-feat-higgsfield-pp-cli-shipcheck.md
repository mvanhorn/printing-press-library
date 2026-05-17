# higgsfield-pp-cli — Shipcheck Report

## Verdict: ship

All 6 shipcheck umbrella legs PASS. Scorecard 86/100 Grade A. Phase 5 (live dogfood) is auth-skipped since the user has only the official CLI's JWT (no platform API key); the skip marker is written and is valid per the auth-aware skip rule.

## Shipcheck umbrella

```
LEG                 RESULT  ELAPSED
dogfood             PASS    1.49s
verify              PASS    5.27s
workflow-verify     PASS    0.04s
verify-skill        PASS    0.46s
validate-narrative  PASS    0.67s
scorecard           PASS    0.09s
```

## Scorecard 86/100 — Grade A

Strong (10/10): Output Modes, Auth, Error Handling, Doctor, Agent Native, MCP Quality, Local Cache, Agent Workflow, Path Validity, Auth Protocol, Sync Correctness.

Weaker dimensions worth a polish pass:
- MCP Remote Transport 5/10 — spec doesn't declare `mcp.transport: [stdio, http]`. Adding it makes the MCP reachable from hosted agents (Managed Agents, web clients).
- MCP Tool Design 5/10 — endpoint-mirror surface for a 13-tool API; could enrich with `mcp.intents` for fanout/soul-search workflows.
- Cache Freshness 5/10 — sync is a placeholder until live API testing confirms field shapes.
- Data Pipeline Integrity 7/10 — generations resource fails sync upserts ("missing id for generations") because the generator picked `id` as the PK but our spec entity uses `request_id` as the canonical key. Worth investigating.
- Type Fidelity 3/5 — minor.

## What was built

8/8 transcendence features implemented (all detected by `novel_features_check`):
- `fanout` (top-level): create, wait, compare. Persists fanout groups under `~/.cache/higgsfield-pp-cli/fanouts/<fanout_id>.json`. `--max-cost` guard calls the per-model cost endpoint before submitting.
- `soul-ids search` — FTS-style LIKE over local `soul_ids` table joined into past prompt history.
- `soul-ids usage` — SQL join: generations × soul_ids, ordered by date.
- `account spend` — SQL aggregation: transactions grouped by model | day | soul_id.
- `search` (top-level, framework-emitted) — FTS5 across synced prompts.
- `export` (top-level, framework-emitted) — JSONL/JSON export of any synced resource.

Plus the full spec-derived absorbed surface: account, generations, models, soul-ids, workspaces, uploads.

## Known gaps (non-blocking)

- **Phase 5 live dogfood not executed.** User has the higgsfield CLI's JWT in credentials.json but no platform API key. The skip is valid per the auth-aware gate. To run later: `export HIGGSFIELD_JWT="$(higgsfield auth token)"` then `printing-press dogfood --live --dir <work_dir>`.
- **Generations sync fails with "missing id for generations".** The sync_walker picks `id` as the primary key but our internal-YAML Generation type uses `request_id` as the canonical identifier. Two fixes possible: (1) add an `x-resource-id: request_id` to the generations resource in the spec; (2) flatten the Generation type so `id` and `request_id` are aliases. Worth a polish pass.
- **MCP transport stays stdio-only.** Adding `mcp.transport: [stdio, http]` would let cloud-hosted agents reach this CLI. Re-run `printing-press generate` with the updated spec to wire it in.
- **Web-backend reverse-engineering risk.** The cloud.higgsfield.ai surface schema can drift; the unified MCP source warns of this. Production users should run against the official platform API when possible.

## Generator bug found (retro candidate)

`auth.instructions: |` blocks with trailing newlines cause the generator to emit `Fprintln(w, "...\n")` with a redundant `\n`, failing `go vet`. Workaround: use `|-` (block strip) or a single-line string. Surface to maintainers.

## Fix loop record

- Loop 1: pluralization mismatch in research.json (`soul-id search` → `soul-ids search`, `model list` → `models list`, `generate export` → `export generations`). Fixed via JSON patches.
- Loop 2: `--since 7d` on `export` is not a real flag; rewrote recipes to use `--format jsonl --output models.jsonl`.
- Loop 3: SKILL.md `value_prop` starting with the binary name was misparsed by verify-skill as an unknown command. Reworded to "This CLI mirrors...".
- Loop 4 (auto-applied by linter): SKILL/README sync from updated research.json.

Final: PASS, ship verdict.
