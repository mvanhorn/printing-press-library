# HIPAA Deployment Guidance

This document is deployment guidance for organizations that may process protected health information with `salesforce-headless-360-pp-cli`. It is not legal advice and does not replace your counsel, compliance team, Salesforce agreement, or security review.

## BAA Disclaimer

This CLI does not ship with a Business Associate Agreement. Customers with PHI must deploy it to controlled infrastructure covered by their own agreements and controls. If Salesforce is the system of record for PHI, confirm your Salesforce BAA and product configuration before exporting data to any bundle, Slack channel, agent, log, or local workstation.

Salesforce publishes information about its Business Associate Addendum and covered services in its trust and compliance materials. Start with Salesforce's own BAA documentation and your signed contract terms before enabling PHI workflows.

## Default Redaction Behavior

Fields whose Salesforce `ComplianceGroup` includes `HIPAA` are redacted by default, alongside other sensitive groups such as `PII`, `GLBA`, and `PCI`. The implementation is in `internal/security/compliance.go`.

Do not use `--include-pii` or `--include-shield-encrypted` in HIPAA workflows unless your compliance owner has approved the specific run, the output path, the audience, and retention.

The redaction model is fail-closed. If compliance metadata cannot be loaded for an object, non-system fields are redacted rather than exported optimistically.

## Audit Sync-Mode

HIPAA mode requires synchronous bundle audit. If a bundle audit cannot be written, bundle emission is blocked. This prevents a signed PHI-bearing artifact from being produced without a corresponding audit trail.

HIPAA mode is enabled by `SF360_HIPAA_MODE=true`, `SF360_HIPAA_MODE=1`, or an install manifest that marks HIPAA mode. The bundle audit implementation writes local pending, failed, and ok states and attempts org-side `SF360_Bundle_Audit__c` records when a Salesforce client is available.

In HIPAA mode, audit failure is a release-blocking condition, not a warning. Operators should monitor local audit health and Salesforce audit object writes before enabling scheduled bundle generation.

## Write Audit Sync-Mode

HIPAA mode also requires synchronous write audit. If `agent update`, `agent create`, `agent upsert`, `agent log-activity`, `agent advance`, `agent close-case`, `agent note`, or `agent execute-plan` cannot write its pending intent audit row, the Salesforce mutation is blocked with `WRITE_INTENT_AUDIT_FAILED`. This is the write-side equivalent of bundle audit sync-mode: no PHI-bearing mutation should occur without a durable forensic record of which key authored it and what payload was intended.

Write audit rows capture the acting user, signing `kid`, target object, target record, operation, intent JWS, execution status, and `FieldDiff__c`. `FieldDiff__c` is the PHI-sensitive field because it records before/after values. Fields hidden by FLS are preserved as `{"redacted":"FLS"}` instead of the original value, so the audit row proves a field was excluded without leaking data the acting user could not read.

Operators should treat both `SF360_Write_Audit__c` and the local `write_audit_local` mirror as regulated records when Salesforce contains PHI. Restrict access, set retention deliberately, and review failed or pending write-audit rows before enabling scheduled agent writes.

## Controlled Infrastructure

Run HIPAA workflows only on controlled infrastructure:

1. Use managed workstations, self-hosted runners, or servers covered by your organization's HIPAA controls.
2. Do not run PHI bundle generation on generic cloud CI runners unless the runner provider, logging path, artifact storage, and Salesforce processing arrangement are covered by a BAA.
3. Do not send generated bundles to cloud logging, hosted traces, crash reporters, or agent transcripts without a BAA and explicit retention policy.
4. Keep the default local-only behavior. Doctor, logs, profiles, keystore records, feedback, SQLite cache rows, and bundles are local unless you configure an outbound sink.
5. Encrypt disks and restrict local filesystem access for users who can generate or verify bundles.
6. Treat Slack injection as PHI disclosure unless every channel member, workspace, retention policy, and app installation is approved for PHI.

## Slack And Agents

`agent inject` re-checks the Slack channel audience before posting and intersects Salesforce FLS across mapped users. That protects field-level audience leakage, but it does not make Slack a HIPAA-approved destination by itself.

Before posting PHI-derived summaries, confirm the Slack workspace, channel membership, app scopes, retention, export controls, and BAA posture. Use ephemeral posts only when your retention policy allows them.

Agents that consume bundles must run in controlled infrastructure. A local bundle passed to a hosted model, hosted coding agent, or cloud trace can become a PHI disclosure.

## Operational Guidance

Run `doctor` before scheduled jobs and block on red rows. Treat yellow rows as risk decisions. Data Cloud and Slack enrichments are optional; disable them if their compliance boundary is not approved.

Use `agent context --dry-run` to inspect field export shape before producing a signed bundle. Use `agent verify --strict` before an agent acts on any previously generated bundle.

Keep generated bundles short-lived. The JWS `exp` claim limits trust duration, but local files still need retention and deletion controls.

## Salesforce References

Review Salesforce's current Business Associate Addendum and covered-services documentation with your account team or compliance portal access. Product coverage and contract terms can change, so rely on Salesforce's official materials for the final HIPAA determination.
