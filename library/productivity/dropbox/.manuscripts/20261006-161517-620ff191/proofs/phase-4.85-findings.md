<!-- slop-gate: off -->
# Phase 4.85 agentic output review

Status: WARN (3 warnings). Sampled overview, mess, journal live (conflicts/organize need an index; apply/undo/links audit skipped as mutating).

1. overview and mess printed all-zero JSON with exit 0 when no index exists; the "run index" hint was stderr-only. Fix (slice M): index_missing + note in the JSON payload for every local read command.
2. overview reported source live with quota null; its no-index early return skipped the index-independent space-usage call. Fix (slice M): quota fetched first; source labels corrected.
3. Store DB filename keyed on StoreScopeCredential() = "Bearer <access_token>"; Dropbox access tokens expire every 4 hours, so each refresh pointed the CLI at a new empty data-<hash>.db and orphaned the index. Fix (slice M): OAuth configs scope by client id (refresh token fallback); guard test added. Retro candidate (generator): StoreScopeCredential must use a refresh-stable identity for OAuth2 CLIs.

User decision: fix all three before shipping (folded into the user-approved review extension).

Verification (orchestrator, live): isolated --home with copies of the real config/credentials; index --root <test-folder> -> data-259722dec3c4.db; access token in the copy corrupted and expiry set to the past; tree <test-folder> refreshed the token (new token written) and read the same data-259722dec3c4.db with 4 files. overview with no index: index_missing true, note present, live quota returned.
