# Unify CLI — Phase 5 Acceptance Report

**Level: Full Dogfood**
**Verdict: PASS — 102/102 tests passed (91 skipped per matrix design), 0 failures**

Marker file: `proofs/phase5-acceptance.json`

## Live API exercise

API key used for read-only Phase 5 live testing: `UNIFY_API_KEY` (user-supplied at the API Key Gate). Auth context recorded in the acceptance marker. Workspace inventory at test time:

- 12 objects (4 Unify standard: company / opportunity / person / user + 8 Salesforce-mirrored)
- 20+ attributes per typical object (company has 20 incl. domain, industry, employee_count, opportunities ref)
- Records sampled into the local mirror via watchlist (gladly.com, stripe.com, plaid.com, gong.io)

## Tests passed in the matrix

102 of 102 mandatory tests. Every leaf subcommand exercised:
- `--help` parses (every command)
- `--dry-run` exits 0 (every mutation-shaped command)
- happy-path with realistic args (every read command)
- `--json` produces valid JSON (every command that supports it)
- error-path with `__printing_press_invalid__` returns typed exit code (every command that takes args)
- output-mode fidelity (`--csv`, `--select`, `--compact`, `--quiet` where applicable)

## Fixes applied during Phase 5

Three iteration loops produced four shippable improvements:

1. **`sync` pre-creates per-object record tables.** Previously, commands like `coverage` and `audit-scores` failed against objects whose records had never been fetched (e.g., `salesforce_account` if you only watched `company` domains). Now sync's schema phase calls `EnsureRecordTable` for every object, so downstream queries see an empty table instead of a missing-table error.
2. **Cobra `Example:` strings updated** in `vet.go` and `import_csv.go` to use `/tmp/prospects.csv` and `/tmp/accounts.csv` so the validate-narrative dry-run probes succeed.
3. **`SKILL.md` recipes synced** to match the now-canonical `/tmp/` paths and `--since 1d` form. Two stale recipe blocks were left over from initial generation; both are now aligned.
4. **Pre-Phase-5 fixture staging:** `prospects.csv` and `accounts.csv` are also staged in the CLI's working dir to feed the dogfood test runner's synthesized argument values. Worth flagging that the runner uses positional placeholder filenames regardless of the example string, which means CLIs with required `--file` flags depend on a CWD fixture for dogfood to pass.

## Printing Press issues for retro

Two test-infrastructure issues worth filing:

1. **Dogfood comment-as-args interpreter bug.** A trailing `# full schema + records refresh` comment in my Cobra `Example:` string was parsed as positional args to `sync`. The runner should respect `#` as a comment boundary in the example string (it's the shell convention).
2. **Dogfood synthesizes placeholder filenames** like `accounts.csv` regardless of what the `Example:` shows. CLIs with required `--file` flags depend on a fixture at the dogfood CWD, which the user has to know to pre-stage. Either the synthesized placeholder should use a tempfile or the runner should auto-stage based on flag-usage hints.

## Manual verification across all 9 novel commands

Beyond the dogfood matrix, each novel command was hand-exercised against the live API in `--json` mode:

- `sync` — 12 objects, 2647 attrs, 4367 options, 4 records refreshed
- `search "gladly"` — single hit with FTS5 snippet
- `sql "SELECT ... FROM record_company"` — JSON rows + dotted-path field extraction
- `coverage --left company --right salesforce_account` — clean zero/zero report
- `audit-scores --field employee_count --field revenue --threshold 1` — 2 records scanned, mechanism verified
- `schema snapshot/diff/list` — snapshot id 1 + 2 with `--since 1d` resolution
- `vet --csv /tmp/prospects.csv` — 3-row CSV resolved to typed enrichment (gladly + stripe found, invalid domain reported missing)
- `import-csv --object company --file /tmp/accounts.csv --match-on domain --plan` — per-row update plan with diff
- `trace company <id>` — depth-1 reference walk

## Acceptance threshold

Full Dogfood threshold: every mandatory test passes, no flagship feature broken, no auth/sync failures. All met.
