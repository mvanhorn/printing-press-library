# Drive Plaza CLI agent guide

Use `README.md` for planning workflows and `SKILL.md` for agent invocation. Runtime help is authoritative: `driveplaza-pp-cli <command> --help`; `doctor --json` checks setup. This local build has not been published.

## Domain invariants

- Public NEXCO East HTTP only. Production domain clients use fixed English/Japanese origins; no browser sidecar, credentials, account changes or purchases.
- Quotes keep vehicle, explicit JST departure/arrival time, priority, waypoints and conditional ETC assumptions attached. Parse source summary columns; preserve null unknowns and zero tolls separately. Never calculate discounts locally.
- SA/PA IDs include direction. Active/gray/missing icons mean true/false/null. Source weekday hours do not establish open-now status. Preserve the provider's 2006-03-31 warning for non-East records.
- RSS titles and dates cannot establish current restrictions. Keep `active_restriction:null` and official handoffs.
- Metadata retains canonical source URLs, retrieval time, request count and partial-language warnings. Selection projects the payload while preserving provenance. Real empty sets stay empty.

## Implementation and verification

Domain parsing and public HTTP live in `internal/driveplaza/`; command wiring and output projection live in `internal/cli/driveplaza_commands.go`. The five generated novel constructors delegate to that wiring; `registerNovelCommand` adds roads and conditions without editing generated root code.

Hand-authored commands declare `// pp:data-source live` or `computed`, the matching `pp:data-source` annotation and honest `mcp:read-only` hints. All domain calls honor `boundCtx` and dry-run before I/O. Use the shared adaptive limiter and surface typed 429 errors. Keep output pages, scan effort, HTTP body size and requests bounded independently.

Run focused domain tests after parser changes. `evidence/live_acceptance.py` is the separate read-only live behavioral matrix; sanitized snapshots under `internal/driveplaza/testdata` never substitute for it. `workflow_verify.yaml` tests the main resolve→quote→directional-stop flow. Review evidence and Press receipts before continuing phases.

## Reprint durability

This is Printing Press output. Preserve standalone domain files and command hooks during regeneration. Record local customizations under `.printing-press-patches/`. Keep spec/research-owned descriptions consistent; do not hand-stamp release versions or fabricate verifier/scorecard fields in `.printing-press.json`.

## Local learning

Learning is optional local state, separate from source facts. Use `recall` before discovery when useful and fetch the current provider response after a hit. Confirm candidates only after verifying them; teach structural questions with public IDs, avoiding personal trip details. `--no-learn` or `DRIVEPLAZA_NO_LEARN=true` disables the loop for deterministic runs. No source quote is current merely because it appears in local memory.
