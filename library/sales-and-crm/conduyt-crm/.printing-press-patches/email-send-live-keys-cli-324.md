# email send / send-bulk: live key names, spec-driven pass-through filter (Go CLI #324, 2026-10-22)

Contract: the live public OpenAPI (https://conduyt.app/openapi.yaml, fetched 2026-10-22). `email send` and `email send-bulk` still sent the
old keys (`body`, `bodyHtml`, `scheduledAt`), so both failed against the API (`to`, `subject`, `html` / `text`, `contactId`, ...).

Spec scope (deliberately narrow): `spec.yaml` and `upstream/openapi.yaml` take the live path items of `/email/send` and `/email/send-bulk`
ONLY, written identically into both, so `TestSpecMatchesUpstreamPublicAPI` stays green. The rest of the live document is NOT taken:
it carries 87 operations this print does not have (and drops 2 callbacks), a wholesale refresh is a separate reprint with its own
review. The test fails on a wholesale refresh until that reprint lands; refresh the two path items or do the full reprint.

Generation: `printing-press` 4.32.1 on the old and new spec.yaml into scratch dirs; the trees differ in exactly `email_send.go`,
`email_send-bulk.go`, `internal/mcp/code_orch.go`, `internal/types/types.go`, `tools-manifest.json`, README/SKILL text. Applied by hand
in the 4.28.0 print's style, because the generator cannot express this pair:
- `/email/send` has an `anyOf` body (html or text), which the generator renders as one `--body-json` flag and no `--to`/`--subject`/
  `--html`; the nearest correct output is the hand-written flags below.
- It also emits `--dry-run` for the `dry_run` query parameter, which would shadow the root `--dry-run` (local request preview).
- Its `types.go` response types (`EmailSendResponse.Data string`) are wrong for a Message object; not taken. Its summary/README text is
  the whole operation description; not taken (summaries stay "Send an email to a contact" / "Send bulk emails").

Flags: `email send` --to --subject --html --text --contact-id --reply-to --send-domain-id (required: to, subject, contact-id, and one of
html/text; checked locally by `cliutil.CheckEmailSendFlags`). `email send-bulk` --contact-ids --subject --text --html --template-id
--subject-edited --reply-to --send-domain-id (required: contact-ids; the subject/body-or-template rule is the server's). A passed flag
is always sent, including an empty one. Not exposed as flags (reachable with --stdin / MCP params): `cc`, `bcc` (rejected by the API
when non-empty), `batchDelayMs` (type undeclared in the live spec), the `Idempotency-Key` header and the `dry_run` / `sandbox` query
parameters (MCP executor binds `dry_run` and `sandbox`; this print has no header binding).

Round 2: `--dry-run` runs the same required-field and limit checks as a real send (no `!flags.dryRun` exception on either command), so
a preview is always a body that would be posted. `CheckEmailSendFlags` also enforces the spec's maxLength / pattern for to, subject,
html, text, contactId, replyTo and sendDomainId; the constants live in `email_send_guards.go` and `TestEmailSendLimitsMatchPublicSpec`
pins every one to `upstream/openapi.yaml`. `/email/send-bulk` declares no maxLength or pattern in the live spec (only `format: uuid`
on ids, which are checked), so it has no further limits to mirror. `--stdin` reads `cmd.InOrStdin()`.

## Hand-fixes a reprint must keep

- `internal/cliutil/email_send_guards.go`: `EmailSendKeys` / `EmailSendBulkKeys` and the checks. The pass-through modes (`--stdin` JSON on
  both commands, and `params` on the MCP executor's `email.send` / `email.send-bulk`) REFUSE a body key the endpoint does not declare,
  with the accepted keys listed and "nothing was sent". Refuse, not warn: the API validates the body as a strict object (a 400 once the
  send-route hardening, #155, lands), so a warning would only delay the same failure and risk sending a mail the caller thinks carried
  the key. `TestEmailSendKeysMatchPublicSpec` pins both lists to `upstream/openapi.yaml`.
- The two commands' flag bodies, the `Long` help and the `CheckEmail*` calls in `email_send.go` / `email_send-bulk.go`; the
  `case "email.send", "email.send-bulk"` guard in `handleCodeOrchExecute`.
- `tools-manifest.json` params for `email_send` / `email_send-bulk` follow the print's style without the generator's "Returns the new ..." suffix.

Version: none (this repo carries no per-change version; `AGENTS.md` says release stamps come from the library workflow).

Tests: `internal/cli/email_send_test.go` (exact JSON bodies for both commands, refusals, --stdin known/unknown keys),
`internal/cliutil/email_send_guards_test.go` (spec parity), `internal/mcp/email_send_guards_test.go` (executor).

## Reprint 637 (2026-10-30)

`email_send.go` and `email_send-bulk.go` are kept as they were (the print still emits `--dry-run`, `--sandbox`, `--batch-delay-ms` and
plain flags). The full document now declares the send-bulk `replyTo` and `sendDomainId` patterns, so `email send-bulk` refuses a bad
`--reply-to` / `--send-domain-id` locally (`CheckEmailSendBulkReplyTo`, `CheckEmailSendBulkSendDomainID`), and the spec readers in
`email_send_guards_test.go` read bounds from the string branch of an `anyOf [string, null]` property.
