# Automation preview subjects: dealId, companyId, executionId on dry-run and test-node (2026-10-17)

Contract: app main 59ccd6a7b (live 2026-10-17, 1:10 PM PT). IMPORTANT: the public `public/openapi.yaml` at that commit does NOT
carry the new fields (it has zero `executionId`, the dry-run body is still `contactId` only, and test-node is the generic untyped
object). The authenticated API catalog (`app/api/v1/schema/api-catalog/generated-api-catalog.json`, built from the overrides in
`scripts/generate-api-catalog.ts`) lists the real request bodies, so the field names, types and texts below were taken from the
catalog's `requestBody.fields` for `POST /automations/:id/dry-run` and `POST /automations/:id/test-node`.

Spec (written identically into `spec.yaml` and `upstream/openapi.yaml`; the operation-level drift gate stays green):
- `dryRunAutomation`: body gains `dealId`, `companyId`, `executionId` (uuid) and `contactId` takes the catalog text; the `mode` and
  `source` query parameters (present in the public OpenAPI for this operation, never in this spec) are added.
- `postAutomationsIdTestNode`: the untyped body becomes the typed one: required `nodeId`, plus `contactId`, `dealId`, `companyId`,
  `executionId`. `requestBody.required` is now true.
- NOT taken (not this lane's delta): `triggerNodeId`, `previousStageId`, `previousPipelineId`, `appointmentId`, which the catalog also
  lists for both operations. They stay reachable through `--stdin`.

Generation: `printing-press` 4.32.1 on the old and the new spec.yaml into scratch dirs (gen-old, gen-new); the trees differ in exactly
`internal/cli/automations_dry-run_automation.go`, `internal/cli/automations_test-node_post-automations-id.go`,
`internal/mcp/code_orch.go` (dry-run `QueryParams` mode and source) and `tools-manifest.json` (both tools' params). The generator's own
govulncheck gate ended both runs (rc 3, Go stdlib advisory in the scratch toolchain) after the project was written, as in earlier lanes.
Applied by hand in the 4.28.0 print's style: the 4.32.1 output adds a `pp:requires-input` bare-invocation guard and a "Returns the new
DataEnvelope" manifest suffix that this print does not have, so neither was taken. `.printing-press.json` is untouched (print version
stays 4.28.0); no version bump (this repo carries none).

New flags (dry-run automation and test-node post-automations-id): `--deal-id`, `--company-id`, `--execution-id`, and on dry-run
`--mode` / `--source`; test-node also `--node-id` (required) and `--contact-id`. A passed flag is always sent.

## Hand-fixes a reprint must keep

- The generator cut every flag help at the first "." (it printed "contact.", "(company.", "admin or owner only)." and "/ ..."), losing the
  rule. The help of `--contact-id`, `--deal-id`, `--company-id` and `--execution-id` is rewritten in both command files to state: deal
  REQUIRED on a deal occurrence, company REQUIRED on a company occurrence (otherwise accepted only when it equals the company the
  execution payload carries; the execution does not replace it), execution = admin or owner only, supplies one fact (the company that
  run's payload carried), the deal-scope sentence (runs carrying a deal are hidden from API keys without the deals scope; added in Codex round 2) and the synthetic sample / unknown/absent sentence. Pinned by `TestPreviewSubjectFlagHelpCarriesTheRules`.
- `tools-manifest.json` descriptions of the two tools follow the print's style ("Optional: ... (plus N more)."), without the generator's
  "Returns the new DataEnvelope." suffix.
- The CLI sends the ids as given; the API owns the UUID and occurrence rules (400/404). `internal/mcp/code_orch.go` binds only the dry-run
  query params; the body is passed through as given.

Tests: `internal/cli/automations_preview_subjects_test.go` (each flag alone on both commands, all together with `--source`, omitted
fields not sent, test-node refuses a missing `--node-id` before any request, help text, manifest params).

## Reprint 637 (2026-10-30)

The live document now carries the full dry-run and test-node bodies, so the print adds `--appointment-id`, `--previous-pipeline-id`,
`--previous-stage-id` and `--trigger-node-id` on both commands, and `--mode` / `--source` take the documented enums. The hand-fixes
above were re-applied: every subject flag is sent when passed (`Changed()`), and the new flags' help is written out in full instead of
the generator's cut at the first ".". The `tools-manifest.json` entries are the fresh print's (they now list all eight body fields).
