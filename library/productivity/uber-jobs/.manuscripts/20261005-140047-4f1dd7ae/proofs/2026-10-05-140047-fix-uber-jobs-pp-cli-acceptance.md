# Acceptance Report: uber-jobs

Run 20261005-140047-4f1dd7ae, Phase 5 (18-dogfood-testing), written 2026-10-05T18:50Z.

```
Acceptance Report: uber-jobs
  Level: Full Dogfood
  Tests: 118/118 passed (run 2; 88 skipped by the runner's design)
  Failures: none in run 2
    - run 1: careers search and careers lookup with --json --dry-run printed
      {"dry_run": true} with no action ("empty dry-run action")
  Fixes applied: 2
    - CLI fix: a describeDryRun hook (internal/cli/uberjobs_hooks.go) names the
      planned request in the generated careers commands' --dry-run JSON
      (TestUJCCareersDryRunJSONNamesTheAction, mutation-checked)
    - CLI fix: sync's pp:happy-args now carries its positional (query=strategy)
  Printing Press issues: 6 (see below)
  Gate: PASS
```

## Runs

| Run | Flags | Matrix | Passed | Failed | Skipped | Requests | Result |
|---|---|---|---|---|---|---|---|
| 1 | `--live --level full` | 105 | 103 | 2 | 105 | 10 | FAIL (dry-run action) |
| 2 | `--live --level full --allow-destructive` | 118 | 118 | 0 | 88 | 10 | PASS |

Run 2 used `--allow-destructive`, which owner rule 11 allows for dogfood --live only. It turns on real happy runs for the four commands annotated `pp:live-happy-path` (save, new, searches, sync), all of which write only to the runner's sandbox store. Error paths for mutating commands stay skipped regardless.

- **Marker:** `proofs/phase5-acceptance.json` (status pass, level full, written by the runner).
- **Results:** `proofs/2026-10-05-140047-dogfood-results.json`; run 1 is kept as `...-results-run1.json` and `phase5-acceptance-run1.json`.

## Owner rule 12 checks

- **Every approved flag and happy_args driven live once before the matrix:** `scratchpad/prestep.py` and `prestep2.py`, 43/43 PASS with row-level checks. For example, every `--sub-team` row is Software Engineering, and every `--work-pattern` row is Regular. Results are in `proofs/phase5-prestep-results.json` and `phase5-prestep2-results.json`.
- **A pass with a non-zero exit:** none found. The runner's results JSON does not record exit codes, so the passing outputs were scanned for error text (0 hits). The pre-step separately checked exit 0 for every happy-args shape.
- **Skips per command:** all 88 are listed in the results. Most are framework or local commands, error paths for mutating commands, or positionals with no fixture.
- **Matrix did not shrink:** 105 tests in run 1, 118 in run 2.
- **One real-data happy run per flagship command:** 8 of 11 come from the matrix: postings, get, facets, save, searches, check, screen, stats. The runner cannot run the other three live, so their real-data runs come from the pre-step. **Owner decision (2026-10-05): accept the pre-step evidence.**
  - `sync`: the runner's hard rule is "sync command requires --dry-run". Pre-step: a full sync (583 postings, facets refreshed) and a keyword sync (399).
  - `new`: its optional `[name]` blocks flag-only happy-args, and with alphabetical order and a fresh sandbox no saved search exists when it runs. Pre-step: a live baseline, `new --all` over 3 searches, and a filtered search.
  - `doctor`: classified as mutating, so it runs only in dry-run mode. Pre-step: a real run printed "API: reachable".

## Traffic (owner TRAFFIC rules)

- **Phase 5 total:** 42 requests (pre-step 22, matrix run 1: 10, matrix run 2: 10), all HTTP 200, with a minimum gap of 3.0 s. There were 0 refusals, and there are no latch files in the real state dir.
- **Today's ledger after Phase 5:** uber.com 108/300, Oracle 3/300.
- **Logging:** every request is in `discovery/uber-request-ledger.tsv` (backups `.bak-pre-phase5-*`).
- **Isolation:** all runs used a scratch or press-scoped HOME, so the owner's real store was never touched.

## Site finding (not a CLI defect; for the README's known limitations)

The careers site's search indexes disagree with each other:
- The unfiltered list reported 583 postings.
- A keyword search returned 6 ids the unfiltered list lacks.
- `contractTypes=Full time` returned 577 results and missed 5 Full-time postings dated August to October. Full time is the only contract type on the site.
- `team=Engineer&subTeam=Software Engineering` missed 2 postings (301346 and 302500) that the unfiltered list labels with exactly that team and sub-team.

The consequences:
- A server-filtered live read can differ slightly from `--data-source local` after a `sync`.
- A full sync can close rows that a keyword sync then reopens.

## Printing Press issues (retro candidates)

1. **Generated read endpoints have no dry-run action** (templates `command_endpoint.go.tmpl` read path and `client.go.tmpl`). With `--dry-run --json`, the generated read endpoints print the client's bare sentinel `{"dry_run": true}`. The same press's live dogfood fails that as "empty dry-run action", so every printed CLI with read endpoints hits it.
2. **Optional positionals block flag-only happy-args** (runner, `resolveCommandPositionals`). A placeholder in square brackets, such as `[name]` or `[query]`, is treated as required, so a command whose happy path takes no positional is skipped as "non-id positional".
3. **No fixture ordering for stateful commands** (runner). The order is alphabetical, every run gets a fresh sandbox, and there is no setup hook. A command that reads state another command creates (here `new` after `save`) can never get a real happy run.
4. **Sync never gets a live happy run** (runner). A sync leaf with `pp:live-happy-path` is skipped as "sync command requires --dry-run" even under `--allow-destructive`.
5. **`doctor` runs only in dry-run mode** (runner). It is classified as mutating, so its happy run never makes the real health check.
6. **Results JSON has no exit codes** (runner). The per-test results omit the exit code, so "a pass with a non-zero exit" cannot be audited from the report. Typed exit codes declared in help count as success.
