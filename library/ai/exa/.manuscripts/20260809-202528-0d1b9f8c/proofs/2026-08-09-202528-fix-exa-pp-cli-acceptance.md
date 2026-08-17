# Acceptance Report: Exa

- **Level:** Full Dogfood (live API, real API key)
- **Tests:** 179/179 passed (100% pass rate)
- **Failures:** none
- **Fixes applied: 3**
  - `entity report` / `webset new` zero-match now returns typed not-found exit 3 with a JSON envelope (was exit 0; error-path matrix expects non-zero for unknown entities/ids)
  - `monitors batch` global `--dry-run` short-circuits to a dry-run envelope when no filter is supplied (the endpoint's own `dry_run` body flag shadowed the CLI global flag, causing a live HTTP 400 "[filter]: Required")
  - JSON early-return reordered after zero-match handling so `--json` still gets the not-found exit code
- **Printing Press issues: 2** (filed in `.printing-press-patches/`)
  - OpenAPI `anyOf` body flattening emits StringVar flags for integer/boolean fields (numResults/moderation/stream) — needed hand coercion to avoid HTTP 400
  - Endpoint body flags that shadow the global `--dry-run` (e.g. `monitors batch`'s `dry_run` body param) defeat the CLI dry-run contract
- **Gate:** PASS
