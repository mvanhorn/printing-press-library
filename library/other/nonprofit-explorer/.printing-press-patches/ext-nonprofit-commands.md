# Patch: hand-authored analytical commands + publish-hardening edits

## Hand-authored files (markerless; safe across reprints)

- `internal/cli/ext_nonprofit.go` — shared helpers: NTEE letter map + `nteeName()`
  (full-code lookup), `printJSONLive()` (agent envelope `meta.source:"live"`),
  `normalizeEIN`, `fmtUSD`, `pctOf`, response types, `fetchSearch`, `fetchOrg`,
  `resolveEINOrName`, `latestFiling`, `formTypeName`.
- `internal/cli/ext_ntee_table.go` — embedded 633-entry NTEE-CC code table
  (source: Urban Institute NCCS classification table).
- `internal/cli/ext_search.go` — `search <query>` ranked live search with
  `--state/--ntee/--c-code/--limit/--page`; bare invocation (no query, no
  changed flags via `hasChangedLocalFlags`) shows help.
- `internal/cli/ext_org.go` — `org <ein-or-name>` profile + latest 990 (+`einDash`).
- `internal/cli/ext_filings.go` — `filings <ein-or-name>` all filings newest-first.
- `internal/cli/ext_financials.go` — `financials <ein-or-name>` YoY trajectory +
  latest-year revenue composition (contributions/program/investment/other,
  personnel-cost share). Includes `compositionMap`, `printCompositionBlock`,
  `round1`. NOTE: a classical program-expense ratio is NOT computable — the IRS
  extract has no functional-expense program/management split.
- `internal/cli/ext_compare.go` — `compare <ein-or-name> ...` side-by-side latest 990s.
- `internal/cli/ext_people.go` — `people <ein-or-name>` officer compensation and
  staffing aggregates per year (compnsatncurrofcr, othrsalwages, payrolltx,
  profndraising). Limitation documented in-command: extract carries AGGREGATES
  only; individual officer names/titles/comp live in the 990 PDF (Part VII),
  linked per year. 990-PF uses a different extract layout → renders unavailable.

One file per command intentionally — keeps each command reachable and
independently scoreable; do not merge back into one file on reprint.

Zero-match `search` renders "0 results" and exits 0 (ProPublica 404s empty
result sets; ext_search.go maps that to an empty page, mirroring the resolver).

## Name-or-EIN resolution

`org`, `filings`, `financials`, `people`, and each arg of `compare` accept
either an EIN (dash optional) or a nonprofit NAME. `resolveEINOrName()`: a
valid 9-digit EIN is used directly; otherwise the arg is searched via
`/search.json?q=` and the TOP result's EIN is used. Human mode prints
`Resolved "<name>" → EIN <ein> (...)` to STDERR; agent mode attaches a
`resolved` envelope field (never mixed into the payload). Zero matches →
`notFoundErr` (exit 3) + JSON error envelope in agent mode. GOTCHA:
ProPublica's search.json returns HTTP 404 (not 200 + empty array) on zero
results; the resolver maps "HTTP 404" to the clean not-found path.

## Edits to GENERATED files (re-apply on reprint)

1. `internal/cli/root.go` — after the learn commands, six direct
   `rootCmd.AddCommand(newNP*Cmd(flags))` calls register the ext commands so
   the Cobra registration graph (and static reachability analysis) sees them.
   The `novelCommandHooks` loop is retained but unused.
2. `internal/cli/search.go` — generated FTS command renamed `search` →
   `search-local` (Use/Short/examples) so the ranked live search owns the
   `search` verb. No runtime RemoveCommand shim needed.
3. `internal/cli/promoted_organizations.go` — validates the EIN via
   `normalizeEIN` before the request (accepts dashed EINs, rejects garbage
   with exit 2). REASON: ProPublica returns HTTP 200 with an
   "Unknown Organization" stub (ein 0) for malformed EINs, so without local
   validation an invalid argument exits 0 (fails dogfood error_path). Its
   Example: string (and promoted_search-json.go's) use real invocations in
   place of the generator's example-value placeholder.
4. `README.md` — Quick Start "first command" uses real examples (was
   `organizations mock-value`, which now fails EIN validation); added a
   `## Cookbook` section with real recipes; `## Commands` leads with the six
   primary verbs (raw mirrors documented below them); `## Output Formats`
   uses real invocations. Every README bash example is live-executable
   (verified: 22/22 exit 0). SKILL.md's two mock-value examples replaced
   with real ones (canonical-sections still passes).
5. `.printing-press.json` — `novel_features` records the five transcendence
   features (name-or-EIN resolution, financials trajectory+composition,
   compare, NTEE-CC naming, people aggregates).

## Library packaging contract (mirrors library/travel/flight-goat)

- Module path is the canonical library path
  `github.com/mvanhorn/printing-press-library/library/other/nonprofit-explorer`;
  all internal imports and the .goreleaser ldflags -X path use it. Local `go
  build ./...` still works (the module path is a name, not a fetch URL).
- `.gitignore` build-artifact patterns MUST be root-anchored
  (`/nonprofit-explorer-pp-cli` not `nonprofit-explorer-pp-cli`) — the
  unanchored form also matches `cmd/nonprofit-explorer-pp-cli/` and silently
  untracks both entrypoints (this broke library CI once). The packaged/PR
  copy ships without a .gitignore at all, matching reference CLIs.
- `.manuscripts/<run>/` carries `proofs/` (phase5-acceptance.json) AND
  `research/` (brief md, absorb-manifest md, spec yml) per the library
  convention.

## Publish bookkeeping

- Project is its own git repo (`git init` + commits) because `/Users/sean/.git`
  exists and breaks Go VCS stamping for un-repo'd subdirs under $HOME.
- `.manuscripts/20260721-170143/proofs/phase5-acceptance.json` — written by
  `cli-printing-press dogfood --live --level full --write-acceptance ...`
  (status pass, 91/91). Regenerate after behavior changes.
- Mirror manuscript at `~/printing-press/manuscripts/nonprofit-explorer/…`
  clears the publish-validate manuscripts WARN.
