# smartEX CLI agent guidelines

Use runtime help for command/flag truth. `smartex-pp-cli doctor --json` checks installation; `smartex-pp-cli agent-context --pretty` enumerates the runtime command tree. README and SKILL describe the supported planning boundaries.

## Domain invariants

Public read-only planning only. Fare calculations are one-adult dated quotes, not trains or inventory. Child and unsupported discount prices stay null. Paid-member EX Reservation is distinct from smartEX. Curated basic timetables are incomplete; an empty result is not no service. Departure-only rows stay distinct from arrival. Current Japanese08:00 and English14:00 confirmation sources disagree: preserve both and null actual confirmation. Oversized one-year and overnight eligibility differs by official language edition: preserve both rules and unknown actual eligibility/opening. Five ordinary/four Green oversized passengers per operation, with no cross-car grouping. Source page reachability is not re-extraction of policy facts.

## Implementation and checks

Keep consequential domain logic in internal/smartex and literal command/flag bindings in each Cobra constructor so Press static and runtime discovery agree. Hand-written live work uses boundCtx before the client; public HTTP is bounded and adaptively paced, with typed throttling errors. Return each partial fetch failure; never insert zero-valued fares. Use flags.printJSON for output modes and field selection. Preserve valid dry-run JSON without IO.

Source strategies are local, computed or live; mixed commands document their branches and honor --data-source. Generated foundation fixes belong upstream when systemic; API-specific behavior is tracked under .printing-press-patches. Preserve framework packages internal/cliutil and internal/mcp/cobratree. Run consequential domain/CLI tests after changes, then required Press gates. Keep schema/README/SKILL/narrative aligned with actual commands.

## Personal GitHub

Future GitHub writes require effective login zjsng via `gh api user --jq .login`; verify token override presence without printing values and fork owner zjsng. Preserve personal commit identity. This local build does not authorize publication or account/booking mutation.
