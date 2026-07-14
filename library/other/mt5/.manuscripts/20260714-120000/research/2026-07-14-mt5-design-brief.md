# mt5 — Design Brief (hand-build research)

**Printed:** hand-built by ek-labs, packaged for the printing-press-library catalog on 2026-07-14 (run `20260714-120000`). The upstream API surface is the [MQL5 Python integration](https://www.mql5.com/en/docs/python_metatrader5) — the `MetaTrader5` Python package, which has no OpenAPI/GraphQL spec (`spec_format: internal`).

## Problem

MetaTrader 5 is the dominant retail FX/CFD terminal, and its only programmable surfaces are MQL5 (in-terminal) and a Windows-only Python package. Neither is shell-friendly or agent-safe: there is no dry-run concept, no audit trail, and no way to hand an LLM agent a trading capability without also handing it the ability to fat-finger a live account.

## Architecture decisions

1. **Go CLI ↔ Python bridge.** The `MetaTrader5` package only runs in-process in Python on Windows. The CLI spawns an embedded `mt5_bridge.py` (go:embed, SHA-256 content-addressed materialization, atomic rename) and speaks line-delimited JSON-RPC over stdio with per-call timeouts. ~50 ms per round trip.
2. **Local SQLite mirror** (`modernc.org/sqlite`, pure Go). Because the bridge is slow and Windows-only, everything MT5 knows is mirrored into a local store keyed by `account_login`: symbols, ticks, per-timeframe bars, orders, deals, positions, calendar, features, backtests, audit. Analytics (`stats`, `sql`, `replay`, `backtest`) read the mirror in microseconds and work on any OS.
3. **Safety pipeline as the write path, not a wrapper.** Every write goes through: live-mode double gate (`MT5_LIVE=1` env AND `--i-understand-this-is-live`), SHA-256 intent-hash dry-run/confirm (60–120 s bucketed window; hash covers tickets + filter, not live price), config guardrails (max volume, position cap, daily-loss from broker data, kill-switch file), and an append-only audit log (JSONL + DB table). The MCP server re-enters the same cobra pipeline in-process, so no tool can bypass it.
4. **Exit-code contract for agents.** `5` (broker rejected — retry may help) is distinct from `6` (safety gate — change the command). `10` config, `11` terminal unreachable.

## Novel features (validated in the shipped tree)

1. `close all --filter "<SQL predicate>"` — compound write resolved by SQL against the mirror, one hash for the batch (the hero command).
2. The mirror itself + `sql` — arbitrary read-only SQL over your full trading history.
3. Defense-in-depth safety layer (above).
4. `replay --speed 100x` — tick-accurate JSONL replay from the mirror for offline strategy development.
5. `bars export` / `ticks copy` — bulk CSV/JSONL quant export that never touches the bridge.

## Build history

Eleven phases (scaffold → bridge → store/sync → reads → algo stats → safety → config → writes → hero flow → quant → MCP → integration tests), each verified against a JustMarkets demo account, followed by five independent review passes fixing 🔴/🟡 findings (bridge desync, hash determinism, per-symbol lot step, guardrail no-ops, audit-loss on failure, migration locking). The complete log with per-phase evidence is `STATUS.md` at the CLI root — it is the primary research/verification record for this hand-build and is not duplicated here.

## Known limitations

- Live data and writes require a Windows host with MT5 running (mirror-backed commands work everywhere).
- Trade-event streaming (`watch trades --tail`) needs a helper EA — deferred to Phase 2 (`helper/TODO.md`).
- `calendar sync` has no bridge endpoint; operators import via `sql --write`.
- Parquet export deferred; v1 ships CSV/JSONL.
