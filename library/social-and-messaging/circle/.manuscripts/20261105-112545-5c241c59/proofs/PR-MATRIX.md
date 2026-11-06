## Test matrix (live, one Circle community)

Every command in the CLI's command tree and every MCP tool was exercised. Reads ran live. Writes ran only on `pp-test-` objects and were re-read to prove they landed. The real spaces (1000967 to 1000972) were never written.

**Live dogfood (`cli-printing-press dogfood --live --level full`, 1,012 rows):** 280 pass, 0 fail, 726 skip, 6 unverified. No hollow features.

Skip reasons, by count:
- 464: example omitted because required inputs (opaque ids) cannot be derived
- 82: no positional argument fixture
- 48: mutating command needs `--allow-destructive` (covered by the write matrix below)
- 39: command does not honour `--dry-run` (framework commands)
- 34: mutating command, dry-run only
- 18: non-id positional (`name` or `query`) without happy-args
- 10: mutating command, error path would call the live API
- 5: destructive at auth
- 4: request body cannot be synthesized
- 4: feature absent for these credentials (Circle returned 404: reviews not enabled)
- 3: export positional mismatch

**Runtime matrix:** all 175 endpoint commands dry-run cleanly; 8 MCP checks pass.

**MCP:** 39 tools listed. `circle_execute` previews without `confirm` and wrote nothing (re-read). With `confirm: true` it created a `pp-test-` topic, read it back, deleted it, and read back a 404. Mirrored write commands preview only, because MCP has no `--yes`.

**Write matrix on generated endpoint commands (2026-11-05):** 61 pass, 1 fail, 50 skip. Every pass re-read the object after the write.
- The 1 fail was a test-fixture artifact: Circle returned a stale profile for a test email that had been created and deleted several times. The same update passed on a fresh email.
- Skips, with reasons:
  - would send a DM (`messages`)
  - changes the real community's settings (`community update`, `chat-preferences`, `settings update`, `tax-setting update`)
  - Circle can only archive access groups, so a test group would stay forever (7 access-group commands; `invitation-links` needs an access group)
  - money or subscription changes (refund, cancel, resume, billing date, payouts)
  - no paywall, coupon, form or workflow exists and the API cannot create one
  - admin tokens get 401 on comment create
  - flagging content notifies moderators
  - affiliate invites send email
  - paywall groups have no delete endpoint
  - image posts need an image space
- The row-level JSON for this run was in a temp folder that was later cleared. The per-command lifecycle proofs for the novel write commands are kept (see below).

**Novel commands, live lifecycle (pp-test- objects, read back each step):**
- `spaces apply`: create, rerun is a no-op, one-field update, read-back.
- `onboard`: tag and member created with no invite email; rerun sees the existing member; space membership added and read back.
- `course push` / `course pull`:
  - 2 sections, 3 lessons, 1 media upload and a reorder.
  - A rerun is a no-op, and pushing a pulled folder is a no-op.
  - A one-field edit pushes only that field.
- `show-tell schedule` / `show-tell recap`:
  - 4 quiet draft events (all notifications off); a rerun marks every date as existing.
  - The recap was saved as a draft post with notifications skipped.
- `seats`, `founding`, `inbox`: live reads, 0 payers, so `founding` returned NO-GO with exit 2 in the release run. After review, NO-GO exits 8 (2 is usage, 3 is not-found); that change is covered by unit tests and was not re-run live.

Hand edit disclosure: the one hand-level change in the release run was a dependency bump of `github.com/yuin/goldmark` from v1.7.13 to v1.7.17 (GO-2026-5320), made with `go get`.
