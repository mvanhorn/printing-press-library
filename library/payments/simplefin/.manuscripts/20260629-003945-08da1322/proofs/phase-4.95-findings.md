# Phase 4.95 Local Code Review — Findings

## Code review (16 hand-authored files): 0 error-severity findings
Reviewer confirmed clean: SQL injection (all placeholders), SQLite drain-first (all loaders close rows before follow-up queries), credential leakage (resolveSimpleFINAccess strips userinfo from BaseURL; access URL never logged/printed), nil/panic, resource leaks, division-by-zero.

### Warnings — FIXED in place
- networth `--at` was a dead flag -> now filters the snapshot trend (`WHERE snapshot_at >= ?`).
- config password-less Basic-auth edge -> token now uses trailing-colon form `user:`.
- reconcile `--fix` could merge coincidentally-identical charges and was non-durable -> now removes ONLY same-account duplicates (cross-account collisions reported as possible transfers, never auto-deleted) + help/output caveat that a later sync can re-introduce removed rows.

### Warning — accepted (low impact)
- export/read paths discard the `ok` from ParseAmount (silent 0 on unparseable). Protocol guarantees decimal strings; left as-is.

## Generator-bug retro candidate (NOT patched — generator-reserved package)
**internal/cliutil/credentials_test.go: 4 failing tests** (TestCredentialsFileWinsWhenLegacyConfigAlsoHasSecrets, TestCorruptCredentialsFallsBackToLegacyConfig, TestCorruptCredentialsFallsBackToEnvCredential, TestEmptyCredentialsFileDoesNotClearLegacyConfig).

Root cause: the generated tests assert `strings.Contains(cfg.AuthHeader(), "<raw-credential>")`. For an `auth.format: "Basic {token}"` spec, the generated `AuthHeader()` base64-encodes the credential (`"Basic " + base64(token+":")`), so the raw value is never a literal substring. The credential-LOADING assertions (`assertConfigCredential`) pass; only the AuthHeader substring check fails. The test template is incompatible with the documented Basic-auth pattern (the Stytch catalog example uses the same `api_key` + `Basic {token}` shape).

Impact on this CLI: none functionally — live auth verified working (doctor + sync + /accounts all succeed against the demo bridge). The failing tests exercise a synthetic non-URL credential path that real SimpleFIN access URLs never reach.

Action: route to /printing-press-retro as a generator test-template bug (credential-precedence tests should decode Basic-auth headers or assert on the loaded field, not substring-match the raw value against a base64-encoded header).
