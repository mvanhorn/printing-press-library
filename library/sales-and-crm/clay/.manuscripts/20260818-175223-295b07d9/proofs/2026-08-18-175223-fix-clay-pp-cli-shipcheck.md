# Clay CLI Shipcheck

## Final leg results

| Leg | Result | Notes |
|---|---|---|
| verify | PASS | |
| validate-narrative | PASS | strict + full-examples |
| dogfood | PASS | structural |
| workflow-verify | PASS | |
| apify-audit | PASS | |
| verify-skill | PASS | |
| scorecard | HOLD | 81/100 Grade A; `live_api_verification` unverifiable in the scorecard sandbox |

Scorecard total: **81/100, Grade A.** Ship threshold is 65.

## Blockers found and fixed

### 1. Dual-credential auth was fundamentally broken (3 compounding bugs)
Clay serves two APIs from one host with two different credentials:
`/v3` (app API) takes the `claysession` cookie; `/public/v0` takes a raw
`clay-api-key` header. Each rejects the other's credential (verified: cookie to
`/public/v0` returns 403). The generator's single-credential model could not
express this, and every failure mode fired at once:

| Bug | Symptom |
|---|---|
| `clay-api-key` stamped on every request | `/v3` returned 401 on every table command |
| `AuthHeader()` wrapped the key as `Bearer <key>` | `/public/v0` rejected the header format |
| `CookieCredential()` preferred the env key | Cookie jar seeded with the wrong value |
| `UsePersistedCookieJar()` false on `env:` auth | Stored browser session silently discarded |

This was not theoretical: the operator's `.env` exports `CLAY_API_KEY`, so the
default state broke the primary use case entirely.

**Fix:** spec `auth.type` changed from `composed` to `cookie` so the framework owns
only the session; the public-API key is injected for `/public/` paths only, read
from the environment at request time and sent verbatim. Verified live: both APIs
authenticate simultaneously against the real workspace.

Two of the three edits land in generated files with no extension hook, so
`generate --force` reverts them. `scripts/apply-clay-auth-patches.py` re-applies
them idempotently. **This is a machine gap, filed for retro.**

### 2. `--workbookId` did not exist
Narrative claimed a flag the CLI does not ship; real name is `--workbook-id`.
Would have shipped a copy-paste-broken Quick Start. Fixed at source in research.json.

### 3. `--pull` referenced but never implemented
`columns link` documented a flag with no implementation. The lookup action's
field-selection contract was never captured, so implementing it would have meant
inventing behavior. Removed from the docs instead.

### 4. `public search-fields` sent an invalid enum
`source_type` accepts only `people|companies`. Spec now declares the enum and a
`happy_args` fixture.

### 5. `feedback` help had no `Examples:` section
Generator-emitted framework command missing an Example, which failed the live
matrix help check. Patched locally; **template-shape retro candidate**, not a
defect this CLI introduced.

### 6. Redundant `workspaceId` body field
Produced an ugly `--workspace-id-2` flag. Removed from the spec.

## Remaining gaps

- **`auth_protocol` 2/10.** Direct consequence of moving the api-key out of the
  spec's auth model. Deliberate: a higher auth score with a broken CLI is worse
  than a low score with a working one.
- **`live_api_verification` N/A.** The scorecard's own live sampler runs without
  the stored session and uses placeholder ids, so it 401s. Phase 5's live matrix
  covered this substantively: 158 passed, 0 failed against the real workspace.
- **`cache_freshness` 5/10.** Cache freshness was not enabled; Clay tables are
  user-owned working state where a pre-read refresh could mask local edits.
- **Row listing unresolved.** `POST /records` is row creation, not listing.

## Verdict

**ship** — all seven legs pass or are unverifiable-in-sandbox, scorecard is Grade A,
Phase 5 live gate passed with zero failures, and no shipping-scope feature returns
wrong or empty output.
