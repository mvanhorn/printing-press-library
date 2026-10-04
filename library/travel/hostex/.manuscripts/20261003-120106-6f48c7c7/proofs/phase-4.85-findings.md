# Phase 4.85 output review (self-review, read-only live sampling)

Sampled against the live account with a scratch mirror (outputs not stored here; they contain guest data).

- ops-gaps, inbox-sla, oversell-watch, revenue-rollup, stay-brief (unknown code): shapes and counts plausible; empty results are explicit.
- WARN fixed: revenue-rollup emitted float noise (`48373.78999999999`); now rounded to the cent.
- WARN (not fixed, by design): automation-preview reports "store not synced" because `automation` is deliberately excluded from default sync (the endpoint requires a `type` param); the command's local scan is empty until `sync --resources automation` is given the type.
- price-parity requires `--property` (documented in --help); `--days`-only invocation prints usage.
