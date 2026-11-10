# Klaviyo CLI reprint on API revision 2026-10-15

Run `20261110-011500-4505b426`. Printing Press 4.35.4.

## Why reprint

- The published CLI was printed by Printing Press 3.5.0 against Klaviyo revision 2026-04-15. Revision 2026-10-15 adds 128
  operations and removes none (`new-operations.txt`): billing usage, the brand kit, campaign audiences and variations,
  per-message scheduling and cloning, flow action inserts and deletes, form update/publish/unpublish, template preview
  sends, template conversion, translations, custom objects and SMS senders.
- The new commands this run adds need those operations: `campaigns test-send` (template preview sends), `send-budget`
  (billing usage), `plan qa-gate --brand` (brand kit), `flows insert-step` (flow actions), `forms publish` /
  `forms unpublish` (form publish jobs).

## Spec

- Source: `https://raw.githubusercontent.com/klaviyo/openapi/1838308847ecf8da756542134d114ea61d8d88d5/openapi/stable.json`.
- Preprocessing, identical to the original print: add `x-prefix: Klaviyo-API-Key` to the `Klaviyo-API-Key` security
  scheme, serialize with two-space indent and a trailing newline. The 2026-04-15 preprocessed spec reproduces its recorded
  checksum with the same steps.

## Generation and merge

- `printing-press generate` produced a fresh tree. `printing-press regen-merge` (dry run, report in proofs) classified 324
  files as body drift and 111 as value drift, almost all generator-version drift from 3.5.0 to 4.35.4; 24 files carried
  hand additions and 48 were hand-written. The 3.5.0 base could not be synthesized for a three-way merge.
- The fresh tree became the base. Every hand edit recorded in git history and in `.printing-press-patches/` was replayed
  onto it (`reprint-reconciliation.md` lists each file, commit and outcome). Hand-written logic moved into `klaviyo_*.go`
  files so later regen-merges classify it as hand-written.

## What changed in the API that the hand-written commands depend on

Checked live on one Klaviyo account at revision 2026-10-15:

- A campaign is a container: `definition{name, builder, archived, send_settings}`. It has no status, audiences, send time
  or channel.
- Campaign messages carry `definition.status` and `definition.send_options.send_time`; campaign audiences carry
  `included` / `excluded` lists and segments; campaign variations carry `details{channel, template_id, subject,
  preview_text, from_email, from_label, reply_to_email}`.
- IDs changed meaning: a campaign ID from 2026-04-15 is a campaign message ID at 2026-10-15, and a 2026-04-15 message ID
  is a variation ID. Reporting rows group by `campaign_id == campaign_message_id`, the message ID.
- `GET /api/campaigns` rejects `filter=equals(messages.channel, ...)` (filterable: archived, builder, created_at, id,
  name, updated_at). `GET /api/campaign-messages` accepts `equals(status,'scheduled')` and `any(id,[...])`, page size up
  to 100.
- The legacy message template, image and assign-template routes take a variation ID.
- `POST /api/campaigns` accepts inline `audiences[]` and `messages[].variations[]`; assigning a template to the variation
  creates a non-reusable copy, as before.
- Recipient estimates are keyed by message ID.

## Design decisions

- One campaign resolver accepts a container ID or a message ID, so IDs users already hold keep working. No fallback to
  revision 2026-04-15.
- `campaigns deploy` creates one unscheduled draft and fails unless the read-back shows draft status and no send time.
- Report-driven commands resolve only the message IDs their report returns, instead of walking campaign history.
- Report windows are calendar days in the timezone the request declares.
- Receipts stay opt-in, as in both earlier versions of this CLI.
