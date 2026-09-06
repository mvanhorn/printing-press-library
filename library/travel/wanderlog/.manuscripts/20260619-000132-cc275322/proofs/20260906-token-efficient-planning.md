# Wanderlog CLI token-efficiency review

Measured with **tiktoken 0.14.0 / o200k_base** on September 6, 2026. This is a static, reproducible synthetic workload and tokenizer proxy—not a SOTA/model-quality evaluation or a guarantee for every model.

| Discovery output | Before tokens | After tokens | Change |
|---|---:|---:|---:|
| Default agent-context --agent | 22,384 | 4,597 | 79.5% smaller |
| agent-context --for-edit --agent | 3,623 | 3,686 | 1.7% larger |
| Targeted schema for plan day | unavailable | 369 | new focused read |

“Before” is the amended local candidate preserved immediately before this token-efficiency pass, not the globally installed release.

The default now returns a slim command inventory; complete schemas are opt-in. The existing edit-focused inventory grew slightly as capabilities were added; it did not become smaller. Counts above are actual preserved/candidate binary stdout, not estimated from byte length.

| Planning workflow | CLI calls | Stdout tokens | Command + stdout tokens |
|---|---:|---:|---:|
| Before: outline + saved route/planning + eight block reads | 10 | 3,595 | 3,867 |
| After: full day response | 1 | 1,682 | 1,717 |
| After: unchanged response using matching prior local state | 1 | 235 | 275 |
| After: one note changed, using matching prior local state | 1 | 307 | 347 |

The complete-day workflow is **55.6% smaller in command+stdout tokens**, with **10→1 CLI calls**. Incremental unchanged/change costs require previously established matching local state; the first full read still costs 1,717 command+stdout tokens.

An illustrative cold workflow, combining default discovery and the day task, totals **26,260→6,707 command+stdout tokens (74.5% smaller)** and **11→3 calls**. The candidate includes an additional targeted plan-day schema request. This is one chosen equivalent workflow, not a claim that it is the optimal old or new interaction.

The synthetic fixture contains four scheduled stops, a temporary closure, visit-duration metadata, missing travel estimates, a complete note with an admission cutoff, a checklist, a date-matched flight booking, and a global note prohibiting rental-car use that day. The retention test passed for all these constraints after the final day response structure settled. Candidate day output includes its command envelope, digest, warnings, and current order; deltas are measured as returned by the actual response helpers.

Scope and limits:

- Before-day outputs use legacy read projections with the former pretty agent serialization. Candidate uses the new day response helpers and compact JSON serialization. No private itinerary fixture or live network replay was used.
- Counts include stdout and shell command text. They exclude tool-call framing, assistant reasoning, system/skill prompts, retries, and latency. CLI invocation counts are not HTTP-request counts.
- Preserving textual/structural constraints is checked automatically; actual model planning accuracy, error rates, and task success need separate evaluations.
- No token reductions were obtained by truncating notes or silently dropping booking/closure constraints.

Reproduction lives in the candidate CLI: `internal/cli/plan_token_benchmark_test.go` generates synthetic artifacts using `WANDERLOG_TOKEN_BENCH_DIR`; `scripts/token-benchmark.py` counts them with an explicitly installed tokenizer. The script documents its temporary-venv setup and does not auto-install packages. Detailed measured numbers are in `token-efficiency.json` alongside this report.

Final verification: all Go tests passed; live acceptance 263 passed, zero failed, 179 skipped/unverified; all 12 publication checks passed. Live fixture reads verified full day output, unchanged deltas, query-mismatch fallback and 0600 state-file permissions. Global installation and publication remain unchanged.
