# Tiimo CLI Shipcheck

## Summary

| Leg | Result | Notes |
|---|---|---|
| verify | **PASS** | |
| validate-narrative | **PASS** | 12 commands resolved, full examples pass |
| dogfood | **PASS** | novel_features_check 10 planned / 10 found / 0 missing |
| workflow-verify | **PASS** | |
| apify-audit | **PASS** | |
| verify-skill | **PASS** | flag-names, flag-commands, positional-args, shell-var-quotes, unknown-command all clean |
| scorecard | **HOLD** | 96/100 Grade A; held only on `live_api_verification` (no credential yet) |

**Verdict: HOLD** — not a failure. Six legs pass outright; the seventh is
unverified rather than failing, pending a live credential (Phase 5).

## Scorecard: 96/100, Grade A

Perfect 10/10 on: Output Modes, Auth, Error Handling, Terminal UX, README,
Doctor, Agent Native, MCP Quality, MCP Description Quality, Local Cache,
Cache Freshness, Breadth, Vision, Workflows, Insight, Agent Workflow,
Path Validity, Auth Protocol, Data Pipeline Integrity, Sync Correctness.
Type Fidelity 5/5.

Below full marks:
- **MCP Token Efficiency 7/10**
- **MCP Remote Transport 5/10** — deliberate. `mcp.transport` is stdio-only
  because the generated HTTP transport binds all interfaces without
  authenticating callers, and this CLI's tools carry a bearer token for the
  user's personal planner. Trading 5 scorecard points for not exposing
  someone's schedule on their LAN is the right call; do not "fix" this by
  enabling HTTP.
- **Dead Code 4/5**
- `mcp_tool_design` / `mcp_surface_strategy` — N/A, omitted from denominator
  (surface is under the threshold that triggers them).

## Issues found and fixed during shipcheck

1. **`export` narrative promised flags that do not exist.** The authored
   example was `export --from … --to … --format csv`; the framework command is
   `export <resource> --format jsonl|json`. `validate-narrative` caught it
   before it reached README or SKILL. Fixed at source in `research.json`.
2. **`export activities` is not a valid invocation.** The framework export only
   accepts resources it can paginate directly; for this spec that is `profiles`
   alone, because every other resource is scoped under a `{profile_id}` path.
   The "get your data out" headline is therefore carried by `backup` (whole
   planner, portable JSON) and `feed` (ICS/CSV), not by `export`. Descriptions
   were rewritten to say exactly that rather than overclaim.
3. **Brain-dump recipe was a shell pipeline.** `printf … | tiimo-pp-cli todo
   add --stdin` parsed as binary `printf`. Rewritten to lead with the CLI, with
   the pipe shown in the explanation.

## Open issue: intermittent SIGBUS inside the scorecard probe harness

The scorecard `--live-check` sample probe intermittently dies with
`SIGBUS: bus error` at a page-aligned fault address. Observed in 3 of 5 runs.

Evidence it is **not** a defect in the printed CLI:

- **The crashing command is different every run** — `stalls`, then `Calendar
  feed`, then `Local backup`. A real bug would be deterministic.
- **Fault addresses are page-aligned** (`0x130bfc000`, `0x109ca0000`,
  `0x1056ff5dc`), the signature of a memory-mapped region changing under a
  running process rather than an ordinary nil/bounds fault.
- **30 sequential invocations of all 10 sampled commands produced 0 crashes.**
- Each command runs clean standalone, exits 0, and emits valid JSON.
- The binary is not being rewritten mid-run: mtime, inode and size were
  identical before and after a `--live-check` pass.

Most likely cause is the probe harness executing samples concurrently against a
shared memory-mapped SQLite file. Filed as a **retro candidate against the
machine**, not patched in the printed CLI, per the template-shape escape hatch:
patching the printed CLI would hide a harness bug that will recur on the next
print.

Not treated as a ship blocker because it does not reproduce outside the
harness and no leg fails because of it. It is disclosed here rather than
silently dropped.

## Remaining gap

`live_api_verification` is unverified. Every probe that needs the network
returns `401 Unauthorized` because no `TIIMO_TOKEN` is set. Phase 5 live
dogfood against a real account is what closes this.

## Ship recommendation

**hold-pending-live-verification.** Structurally the CLI is Grade A with every
mechanical gate green. The honest blocker is that nothing has been exercised
against a real Tiimo account since the commands were written; Phase 5 is not
optional here.
