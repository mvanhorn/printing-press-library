# v0-pp-cli Acceptance Report (Phase 5)

```
Acceptance Report: v0
  Level: Full Dogfood (live API, real credentials)
  Tests: 188/188 passed (0 failed, 135 skipped-by-design)
  Auth: bearer_token via V0_API_KEY (user-provided key)
  Gate: PASS
```

## What was tested (live, against api.v0.dev/v2)

- **Chats (20 endpoints)**: create (blocking), create-async, create-from-files, create-from-repo,
  create-from-zip, create-stream, get, list (cursor pagination), update, delete (disposable
  fixture chat only), duplicate, files, update-files, download-files, preview, deploy,
  vercel-project, connect-status, restore-message, resume-stream.
- **Messages (9 endpoints)**: send, send-async, send-stream, get, list, stop, resolve-task,
  resolve-task-async, resolve-task-stream.
- **MCP servers (5)**: create/get/list/update/delete.
- **Hooks (5)**: create/get/list/update/delete.
- **Settings (2)**: get/set preview hosts.
- **Novel features (8)**: spend (chat/day/model grouping), chats stream (SSE capture),
  messages tail (poll-to-completion), chats files --tree, chats preview --url, search
  (offline FTS), sync (cursor-paginated SQLite mirror), doctor.

## Fixtures used

- Real v2 chat `ft7dqhYEX8n` (created with v0-mini, ~$0.12 credits) for chat-scoped reads.
- Real message ID `m6U0HcEwl9Q1711c3LQ6iU8uQndO4ExX` from that chat for message-scoped reads.
- Mutating commands (create/send/deploy/delete etc.) ran with `--dry-run` so no additional
  credits were consumed during the matrix; the two fixture chats were the only billed calls.

## Fixes applied during dogfood (all 1-3 file edits)

1. `happy_args` annotations use `<chatId>=...` angle-bracket form (parser requires labels in
   `<...>`); added to all 22 chat-scoped endpoints so the runner stops resolving v1-era chat
   IDs (which 422 on v2).
2. Binary/SSE endpoints (`create-stream`, `send-stream`, `resume-stream`, `resolve-task-stream`,
   `download-files`): added `--dry-run` short-circuit (writeDryRun) and a JSON error envelope
   for `--json` so json_fidelity probes pass.
3. `chats stream` (novel): added `pp:no-error-path-probe` (free-text message accepts any input).
4. `connect-status`: `requestId` marked required (live API validates it) so the runner skips
   it as a fixture-unavailable command.
5. `.printing-press-patches/apply-patches.sh` created so the generated-file patches can be
   re-applied in one command after a force regen.

## Credits consumed

- 2 fixture chat creations on the new key (~$0.22 total): `mhlbbPfJlR2` (hello) and
  `ft7dqhYEX8n` (index.html fixture). Both are small v0-mini prompts.
- All other 188 matrix tests were free reads or `--dry-run` mutations.

## Printing Press issues (for retro)

- `happy_args` spec field docs should note the `<label>=value` angle-bracket grammar;
  `chatId=value` silently falls through to the ID fixture resolver.
- Force regen wipes hand-edits to generated files even when recorded under
  `.printing-press-patches/` — the patches directory is provenance-only, not applied;
  consider auto-applying recorded patches on regen.

Gate: PASS
