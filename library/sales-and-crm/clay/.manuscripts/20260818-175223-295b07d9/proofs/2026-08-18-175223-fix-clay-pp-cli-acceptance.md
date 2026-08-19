# Clay CLI Acceptance Report

```
Level: Full Dogfood (reads only, per operator selection)
Tests: 158/158 passed, 0 failed, 155 skipped
Gate:  PASS
```

## Environment
- Auth: `claysession` browser session imported via `auth login --cookies-file`,
  plus a Public API key in `CLAY_API_KEY`.
- Target: the operator's live Clay workspace and its existing workbook.

## What was exercised
- Help, happy-path, JSON fidelity, and error paths across every leaf subcommand.
- Live reads against the real workspace: workspace metadata, permissions, sources,
  subroutines, workbook overview graph, table details, schema, row counts, and
  per-column run status.
- Public API reads: authenticated user/workspace and search filter fields.
- All 11 novel commands resolve, dry-run cleanly, and emit parseable JSON.

## Writes
None. The operator selected reads-only, so no table or column was created,
modified, or deleted during acceptance.

## Skips
155 checks skipped as `blocked-fixture: required API parameter`. Clay embeds
`{workspaceId}` in every `/v3` path, so generated endpoint commands take it as a
required positional and the matrix has no fixture value. This is coverage lost to
ergonomics, not a defect; novel commands avoid it by accepting `--workspace` or
`CLAY_WORKSPACE_ID`.

## Fixes applied during acceptance
1. Dual-credential auth (3 compounding bugs) — see shipcheck report.
2. `public search-fields` enum (`people|companies`) plus a happy-path fixture.
3. `feedback` command given a missing `Examples:` section.
4. `blueprint` parent declared as a `pp:parent-group` so it is not probed as a leaf.

## Printing Press issues for retro
1. **No extension hook for auth-model overrides.** A two-credential API cannot be
   expressed in the spec, and the required edits sit in generated files that
   `generate --force` reverts.
2. **`feedback` template ships without an `Examples:` section**, which its own live
   matrix then fails.
3. **Capture-time secret redaction misses credentials in header values.** A live
   `Authorization: Basic` credential inside a captured request body was not caught
   by the `"apiKey":`-shaped redaction pattern.
4. **`references/browser-sniff-capture.md` assumes browser-use CLI 2.x.** The
   documented `browser-use open` / `eval` subcommands were removed in 3.0.
