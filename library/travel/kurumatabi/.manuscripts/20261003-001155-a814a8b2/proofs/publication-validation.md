# Publication validation and live evidence

Printing Press4.32.5 validated the dedicated publication source, using Go1.27.1. Supported regeneration backfilled the existing creator handle from the same-person legacy printer zjsng; no manual creator rewrite or release-version bump was made. Supported creator regeneration preserved all20native code/asset guards before the documented publication fixes below. Local installed binaries/skills and historical proofs were preserved.

## Authoritative validation

| Check | Result | Note |
|---|---|---|
| manifest | PASS |  |
| transcendence | PASS |  |
| phase5 | PASS |  |
| go mod tidy | PASS |  |
| module path | WARN | go.mod module path "kurumatabi-pp-cli" does not start with the canonical library prefix github.com/mvanhorn/printing-press-library/library/<category>/<slug> |
| govulncheck | PASS |  |
| go vet | PASS |  |
| go build | PASS |  |
| --help | PASS |  |
| --version | PASS |  |
| verify-skill | PASS |  |
| patches | PASS |  |
| manuscripts | PASS |  |

The pre-package bare module warning is expected; publish package requires and checks the canonical module github.com/mvanhorn/printing-press-library/library/travel/kurumatabi. Reachable govulncheck, vet/build and help/version checks are scoped to this CLI. Required MCP metadata and the customization index are retained; generated registry/skill mirrors and release accounting are left to library automation.

## Fresh full live gate

Actual command contract: cli-printing-press dogfood --live --level full --timeout 120s --research-dir RESEARCH --write-acceptance phase5-acceptance.json --json. It ran at 2026-10-03T07:42:29.975699Z and returned PASS: 116/116executed checks passed, 0failures, 89explicitly skipped/unverified. The binary minted the adjacent phase5-acceptance.json and its source fingerprint/per-file map; no marker was hand-edited and no skip was inferred. The raw JSON transcript lived in a private temporary directory outside manuscripts and was deleted after inspection.

Nine native park leaves have live happy-path, JSON-fidelity, dry-run and help coverage. The generic runner skips five native negative rows based on positional detection; deterministic focused tests cover actual usage validation. Six BLOCKED_FIXTURE rows concern generated learning confirm/reject candidate IDs, not park workflows. Other skips are runner mutation/dry-run/export/fixture policies. These remain unverified rather than being represented as passes.

All five approved workflows are implemented. Live fit/compare/audit use real public details. Near/match cold-cache samples correctly return an empty observed pool with hints; populated cached ranking/matching was independently verified over29real observations during generation. This does not claim nationwide proximity coverage or source vacancy.

Focused publication fixture/parser/CLI regressions passed uncached after minimization and email/widget-key removal. The preserved native tests also exercise modern/legacy icons, conservative membership/fee/disposal semantics, correct comparison denominators, source pagination, explicit429errors, agent envelopes and bounded SQLite concurrency. The full uncached test suite passed all13tested packages after the publication fixes; packaged build/vet/govuln/skill/validate and repository-specific package verifiers are required before the PR.

## Scoped publication fixes

The source diagnostic path now bounds HTML/JSON response-body reads to4MiB regardless of Content-Length (including Go transparently decoded gzip bodies) and bounds each explicit gzip/deflate decoding layer to the same limit before parsing, caching or status/retry decisions. The generated binary-delivery branch and its streaming timeout remain compatible; Kurumatabi has no verified binary source workflow. Deterministic tests exercise success/429/503 bodies, lying/unknown lengths, body closure, exact-limit acceptance, compressed expansion and chained encodings, plus preserved generic binary bytes.

Agent selection preserves source and coverage metadata even when only metadata is selected; the stable results field becomes[] when no rows were requested. Partial park comparison emits one usable stdout envelope with requested/compared counts and failure IDs, then returns typed exit5. Explicit delivery occurs only after success, so failed comparison buffers are not delivered. The metadata/row/mixed/all-miss selector regressions, actual CLI oversize exit and partial comparison tests passed. Actual22-field complete and partial comparison matrices also reject bare/results-prefixed selector typos with exit2; unrelated nested empty arrays cannot suppress the miss, while known-empty anchored fields remain selectable. MCP wording describes its source build target rather than claiming a prebuilt binary is packaged.

## Public artifact boundary

This tree includes source/spec metadata, minimal contract fixtures and authored research/validation summaries. It excludes raw provider response dumps, runtime databases, private installed/final/publish reports, host filesystem paths, pipeline/receipt logs, cookie/session state, captured credentials and binaries. No booking/provider transaction was performed. The contribution awaits manual review; release version/catalog/skill mirror publication occurs after merge.

## Runtime help

```text
Plan Japanese overnight vehicle stops with explicit fit, facilities and unknowns.

Highlights (not in the official API docs):
  • parks fit   Compare your vehicle with one park’s published dimensions and membership rules.
  • parks compare   Compare park evidence without inventing total prices or unknown facilities.
  • parks near   Rank cached parks by straight-line distance from an explicit waypoint.
  • parks audit   Surface disabled-icon conflicts, stale observations and qualified opening notes.
  • parks match   Screen cached parks into proven, uncertain and ruled-out service matches.

Agent mode: add --agent to any command for JSON output + non-interactive mode.
Health check: run 'kurumatabi-pp-cli doctor' to verify connectivity.
See README.md or the bundled SKILL.md for recipes.

Usage:
  kurumatabi-pp-cli [command]

Available Commands:
  agent-context  Emit structured JSON describing this CLI for agents
  api            Browse API resource interfaces by raw name
  completion     Generate the autocompletion script for the specified shell
  doctor         Check CLI health
  export         Export data to JSONL or JSON for backup, migration, or analysis
  feedback       Record feedback about this CLI (local by default; upstream opt-in)
  help           Help about any command
  learnings      Inspect or forget the local search_learnings table
  parks          Discover and evaluate official Japanese overnight vehicle parks
  playbook       Inspect or amend stored CLI playbooks
  profile        Named sets of flags saved for reuse
  recall         Check prior learnings for a query before running discovery (LLM-fired, pre-discovery)
  source         Inspect low-level public source links; use parks commands for planning.
  teach          Record a query -> resource mapping for future recall (LLM-fired, silent)
  teach-lookup   Install a manual entity-lookup row (kind, canonical, value)
  teach-pattern  Install a manual generalization pattern (query_template, resource_template, entity_kind)
  teach-playbook Record a CLI playbook + free-text notes for a query family
  version        Print version
  which          Find the command that implements a capability
  workflow       Compound workflows that combine multiple API operations

Flags:
      --agent                   Set agent-friendly output defaults (--json --compact --no-input --no-color)
      --audit-dir string        Aggregate the receipt and index under this audit directory
      --client-profile string   Select the tenant-gated client profile (env: PRINTING_PRESS_CLIENT_PROFILE)
      --compact                 Return only key fields (id, name, status, timestamps) for minimal token usage
      --config string           Config file path
      --csv                     Output as CSV (table and array responses)
      --data-source string      Data source for read commands: auto (live with local fallback), live (API only), local (synced data only) (default "auto")
      --deliver string          Route output to a sink: stdout (default), file:<path>, webhook:<url>
      --dry-run                 Show request without sending
  -h, --help                    help for kurumatabi-pp-cli
      --home string             Root directory for config, data, state, and cache files
      --human-friendly          Enable colored output and rich formatting
      --json                    Output as JSON
      --max-age duration        Maximum acceptable age of local-store data before a stderr hint suggests sync; 0 disables (default 30m0s)
      --no-cache                Bypass response cache
      --no-color                Disable colored output
      --no-input                Disable all interactive prompts (for CI/agents)
      --no-learn                Disable the teach/recall learning loop for this invocation
      --plain                   Output as plain tab-separated text
      --profile string          Apply values from a saved run profile; this does not select a client (see 'kurumatabi-pp-cli profile list')
      --quiet                   Bare output, one value per line
      --rate-limit float        Max requests per second (0 to disable; default auto — pace to server rate-limit headers) (default auto)
      --receipt                 Write an atomic private run receipt
      --receipt-file string     Override the run receipt destination
      --select string           Comma-separated fields to include in output (e.g. --select url,text)
      --timeout duration        Request timeout (default 1m0s)
  -v, --version                 version for kurumatabi-pp-cli
      --yes                     Skip confirmation prompts (explicit confirmation for scripts)

Use "kurumatabi-pp-cli [command] --help" for more information about a command.
```

## Current review contracts

All six current-head review gaps were corrected with deterministic behavior tests. The bounded scan cache retains all28observations while returning3rows and preserves richer details. Local/automatic detail reads reject card-only observations. Actual YY bath/onsen icons retain both source observations, with qualified fee uncertainty and explicit generic bath presence. Scoped transactional teaching undo invalidates only affected inferred structural query/resource/venue families, retains manual/unrelated rules and rolls back both changes on a dependency failure. Explicit manual pattern promotion cannot be downgraded by later inference. Verified prefix candidates are literal, case/type scoped and unique; ambiguous matches abstain.

The real compiled companion's partial comparison was exercised through both native MCP mirror and recipe handlers. Each remains a failed tool with one usable partial JSON block, truthful2requested/1compared counts and the failedID, plus a separate UTF-8-safe diagnostic. Oversized evidence is a marked preview. Failure text is bounded at60,000bytes for evidence plus4,000for diagnostics. Full source and canonical package test suites, vet/build, reachable vulnerability checks, skill and publish validation pass for the reviewed source.
