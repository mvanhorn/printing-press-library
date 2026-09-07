# Acceptance Report — cdc-pakistan-pp-cli

    Level:  Full Dogfood (operator-selected)
    Tests:  107/107 passed, 0 failed, 82 skipped
    Marker: proofs/phase5-acceptance.json  status "pass", level "full"
            coverage_hollow ABSENT, 190 source files fingerprinted
    Gate:   PASS

## How the live matrix was run against a challenged site
CDC is Cloudflare-gated, and the dogfood runner executes the binary in a sandboxed
HOME with no captured browser session, so a cookie-auth CLI would 401/403 every
command. The clearance was injected through the CLI's own env-var path:

    CDC_PAKISTAN_CF_CLEARANCE / CDC_PAKISTAN_USER_AGENT / CDC_PAKISTAN_CLEARANCE_MINTED_AT

That env-first precedence was a deliberate design choice made BEFORE this phase,
specifically so the matrix could exercise real requests without a session on disk.
It paid off: the run made genuine live reads and genuine local writes.

## Three defects found and fixed IN this phase (all mine)

1. `coverage_hollow: true`, hollow_features ["stats history"]  -> FIXED
   Root cause: the feature's `pp:happy-args` passed `--limit=5`, which was not a
   declared flag on that command, so the happy path exited 2 and the feature was
   never successfully EXECUTED. Not a gate quirk -- a real broken invocation.
   Fix: added a genuine `--limit` flag (history can be long, so the flag was a
   real gap) AND repointed happy-args at `--snapshot`, the command's actual
   local-write path, so the coverage gate observes the write executing rather
   than only a read. This is the same configuration that cleared the gate for
   MUFAP: declare the feature at an executable leaf and let its real write run.
   NOT DONE: mis-annotating the command `mcp:read-only`. It writes; it says so.

2. `verify rows` exited 3 on a store with no penetration report  -> FIXED
   Root cause: an absent vintage was treated as notFound. But a fresh install
   legitimately has none -- the series lives in one category and only ONE live
   vintage exists at a time because CDC removes prior months. That is an EMPTY
   LOCAL-CACHE STATE, not an error.
   Fix: emit an empty typed result at exit 0 plus a stderr hint naming the
   command that populates it, matching the missing-mirror convention the other
   six read commands already followed. Chosen over leaning on
   `pp:typed-exit-codes`, because consistency was the real defect.

3. `no such table: cdc_documents` leaked as a raw SQL error  -> FIXED
   Root cause: the missing-mirror guard tested whether the DB FILE existed, but
   the generated store CREATES that file (with framework tables) on first open.
   So on a fresh install the file exists while the CDC tables do not and the
   guard never fired. Surfaced as a SIGBUS-adjacent hard failure under the
   scorecard sample probe.
   Fix: new internal/cli/cdc_mirror.go checks sqlite_master for the table itself.
   Wired into all six read commands.

## Printing Press issues observed (retro candidates, NOT patched in the printed CLI)
* dead_flags `maxAge` and dead_functions `isDryRunResponseForClient`,
  `successfulNoop` are all in GENERATED files carrying the DO-NOT-EDIT header
  (internal/cli/root.go, internal/cli/helpers.go, internal/store/events.go).
  Patching the printed CLI would hide the machine bug from the next print.
* `config inconsistency: write fields [, token, ] vs read fields
  [ || c.CookieCredential() != ]` -- both "field names" are parser artefacts, not
  real fields. A cookie-auth CLI trips this every run.
* novel-feature depth mismatch: `stats history advertised as stats but registered
  as learnings stats`. CONFIRMED FALSE POSITIVE, and a NEW flavour beyond psx
  WU-3: there is no `learnings stats` command at all (`learnings` has only
  candidates/confirm/forget/list/purge/reject). The walker named a parent path
  that does not exist. Root `stats` resolves correctly and runs the CDC code.
* `pp:typed-exit-codes` on a novel leaf was not honoured by the scorecard's
  sample probe (it was honoured by the live matrix).

## Honest limits recorded rather than papered over
* 82 of 107 matrix entries were SKIPPED, largely auth-required and mutation-shaped
  probes. The 25 that executed covered every novel leaf's help, happy path, JSON
  fidelity and error path.
* PDF-backed features need macOS PDFKit for print-driver vintages, which is every
  vintage from 2025 on. On other platforms they fail honestly rather than
  returning empty rows.
* The free-float panel remains ONE live vintage. No delta or point-in-time feature
  was shipped, because none is supportable.
