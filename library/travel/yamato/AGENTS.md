# Yamato Transport CLI agent guide

Use `README.md` and `SKILL.md` for supported public luggage-planning workflows. Source captures, live assertions, measurements and reviews live in `.manuscripts/`.

## Source and domain contract

- Keep the six planning commands: parcel, quote, airports, counters, same-day and products. Read current `--help` for flag semantics.
- `parcel` is computed from a dated local rule snapshot. Other planning commands use first-party public HTTP; normal calculator cookies stay in memory for one invocation. These commands expose no shipment/payment/private tracking/account mutations.
- Preserve dimension and weight categories, product/date-kind distinctions, JST, cash/cashless tariffs, included versus conditional fees, source freshness and unknown acceptance. Estimated dates are not guarantees; counter closing hours are not dispatch deadlines.
- Return image/PDF transcriptions only after checking current bytes against the verified source hash. Restrict same-day claims to the covered Narita/Haneda airport-to-hotel rows and hand off all other routes to the full source.
- Parse the primary submitted quote tables. Expandable comparisons for other products contain different dates and rates.

## Implementation and verification

Hand-written domain code is in `internal/yamato/`; command bodies are in the six named `internal/cli/` files with shared helpers in `yamato_commands.go`. Preserve generated framework wiring and tests. Novel commands declare `pp:data-source` in comments and annotations, use the generated JSON/field-selection helpers, and bound their complete network operation with `boundCtx`.

Run `go test ./internal/yamato ./internal/cli` for domain/command changes, then required Printing Press checks and read-only live cases. Fixtures verify consequential parsing/domain logic; live source checks verify current provider behavior. Domain tests use in-memory transports and need no local listening socket.

No-auth public planning requires no credentials. Local `--no-learn` disables generated invocation learning for deterministic verification. Do not infer future source availability, acceptance or tariff changes from old evidence.

## Generated output and releases

This is Printing Press generated output with retained hand-written domain files. Record generated-tree customizations in `.printing-press-patches/` so regeneration can check that command delegates and domain files survived. Keep `spec.yaml` and research descriptions aligned with the approved manifest; use Press sync when generated descriptions change.

The generated store schema is forward-only. Preserve any existing release ledger; a local build remains unstamped until a separately authorized publication assigns a release. Do not hand-bump runtime release bookkeeping.

GitHub publication is outside this local build. When explicitly authorized, verify the effective personal GitHub login is `zjsng` before writing.
