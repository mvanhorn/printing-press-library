# Free-form JSON body flags restored after the full reprint (board #637 round 2, 2026-10-30)

Printing Press 4.28.0 now prints an object body property that declares a few sub-properties as one flag per sub-property
(`--metadata-source`, `--custom-fields-website`, ...), and an anyOf/oneOf body as `--body-json` only. On the full public document
that removed flags existing users had, although the API still documents and reads the same properties. Each one is restored by hand
in the shape the pre-reprint print had: a string flag holding JSON, parsed and sent as the whole body key.

| Command | Restored flag | Body key |
|---|---|---|
| activities create-activity | --metadata | metadata |
| automations update | --trigger-conditions | triggerConditions |
| bulk edit-contacts | --updates | updates |
| bulk edit-deals | --updates | updates |
| contacts create | --custom-fields | customFields |
| forms create | --settings | settings |
| integrations connect | --config, --credentials | config, credentials |
| scoring-rules create | --condition | condition |
| emails create-template | --body, --body-html, --category, --channel, --is-active, --name, --subject | the same keys (typed) |

A reprint must keep:
- the flag, its `bodyX` string variable and the block after the generated nested-object block (`// PATCH(reprint-637-freeform-json-flags)`):
  malformed JSON is an error naming the flag; the flag together with one of its generated `--<flag>-*` sub-flags is refused before any
  request (the sub-flags are kept: they are the print's).
- emails create-template: the seven typed flags beside `--body-json`; with any typed flag `--body`, `--channel` and `--name` are required
  (the pre-reprint rule; the blocks/plain shape), `--body-json` alone still sends any documented shape, the two never mix.
- Pinned by `internal/cli/freeform_json_flags_test.go` (`TestFreeformJSONFlagsLandInTheBodyAsBefore`, one row per restored flag, and
  `TestEmailsCreateTemplateTypedFlagsAsBefore`).

Upstream issue (for the board, not filed): keep a whole-object JSON flag next to the sub-property flags when a property is an object.

Round 4: every row of the test now pins the mixing refusal (forms `--settings-consent-link-url`, integrations
`--credentials-api-key`, scoring-rules `--condition-field` added); a row without a sub-flag fails the test.
