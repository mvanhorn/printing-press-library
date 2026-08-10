# Novel-Features Brainstorm — Q-SYS (reprint run 20260810-144721)

Subagent output (sa_20260810_195037_000000000_647e90bc375b), saved for audit.

## Customer model
1. **Dana Reyes — senior AV integrator / quoting engineer**: BOMs from RFP lists; cross-references spec PDF, compatibility matrix, deprecation notices per part; EOL parts slip into quotes.
2. **Marcus Webb — commissioning technician / field installer**: locked-down/air-gapped job sites; no offline docs; version drift (reads 10.0 behavior for a 9.4 system).
3. **Priya Nair — AV system designer**: picks models for new designs; compares candidates; checks UC-platform certifications (Teams/Zoom/Meet) buried in 34 flat Application_Integration pages.

## Survivors (>= 5/10)
| Feature | Command | Score | Buildability |
|---|---|---|---|
| Unified product card | product get CX-Q --agent | 10/10 | hand-code (built) |
| BOM compatibility check | compat check CX-Q TSC-70-G3 --qds 9.4 --agent | 10/10 | hand-code (built) |
| Deprecation sweep | compat deprecated CX-Q CXD-Q --agent | 10/10 | hand-code (built) |
| Connection guidance by model | connect TSC-70-G3 --agent | 10/10 | hand-code (built) |
| BOM sweep (one report per model) | bom verify --qds 9.4 --agent < bom.txt | 9/10 | hand-code (NEW) |
| Version-aware page reads + drift warning | page get <page> --version 9.4 | 9/10 | hand-code (NEW) |
| Extraction coverage report | coverage --agent | 7/10 | hand-code (built) |
| UC platform integration lookup | integrations TSC-70-G3 --agent | 7/10 | hand-code (NEW) |

## Reprint verdicts (prior 7)
- product get: **keep** · compat check: **keep** · compat deprecated: **keep** · connect: **keep** · coverage: **keep**
- product compare: **drop** — "depends" weekly use; covered by per-model product get
- sql: **drop** — escape hatch, no weekly persona ritual; --agent/--select serve agent queries

## Killed candidates (full list in subagent transcript)
product compare (drop), sql (drop), compat since (upgrade-history niche), docs changed (no persona tracks doc diffs), docs tree (thin sitemap wrapper), spec-meets-requirement (LLM dependency), control-pin lookup (out of scope per user), model resolve (dup of product get join).
