# Security Model

This document describes the security posture for `salesforce-headless-360-pp-cli`. It is written for operators who need to decide whether a generated Customer 360 bundle can be trusted by an agent, a human reviewer, or a Slack audience.

## Scope

The CLI emits signed Customer 360 bundles, verifies bundles, syncs local Salesforce slices, enriches bundles with optional Data Cloud and Slack context, and can post a field-gated summary to Slack. It does not replace Salesforce authorization, identity, audit, or data-classification controls.

The implementation is spread across these primary paths:

| Area | Implementation |
| --- | --- |
| Bundle assembly and manifest signing | `internal/agent/bundle.go` |
| Bundle verification | `internal/agent/verify.go` |
| File-byte attestation | `internal/agent/file_attestation.go` |
| Trust key registration | `internal/trust/register.go` |
| Local keystore | `internal/trust/keystore.go` |
| JWS signing and verification | `internal/trust/jws.go` |
| FLS and compliance filtering | `internal/security/` |
| Slack audience intersection | `internal/agent/audience_intersect.go` |
| Doctor competing-tool detection | `internal/cli/doctor.go` |

## Trust Model

Bundles are signed with device-scoped Ed25519 keys. A device key is derived from a local private key plus device and user identity attributes, then identified by a stable `kid`. The private key stays on the local machine. The public key is registered to the Salesforce org so verifiers can tie a bundle back to the org trust root.

The preferred org registration path is a Salesforce Certificate. Certificates are harder for ordinary metadata workflows to overwrite accidentally and are the strongest current anchor for org-side key discovery. When Certificate APIs are unavailable, the CLI falls back to Custom Metadata Type records under `SF360_Bundle_Key__mdt`.

Multi-device use is native. Each device registers its own key and gets its own `kid`. Rotation retires the old local record and creates a new device key. Verification accepts previously signed bundles until their `exp` claim lapses, but new bundle generation refuses retired keys.

The local keystore is an offline verification cache, not the source of org truth. It lets `agent verify` validate signatures without calling Salesforce. `agent verify --live` re-checks the org key collection when Salesforce auth is available.

## Threat Model

### Compromised Laptop

If a laptop is compromised, the attacker may gain the local private signing key, cached tokens, local SQLite rows, generated bundles, and logs. The design limits blast radius through short bundle expiration, device-specific `kid` values, per-device revocation, local-only defaults, and org-side key registration.

Operators should revoke or retire the compromised device key, rotate Salesforce auth tokens, and invalidate bundles whose `kid` came from that device. The CLI records local trust audit events to support incident review.

### Compromised Admin

A Salesforce admin can change metadata, alter CMDT fallback rows, install or remove Apex, and grant broad object or field permissions. The CLI cannot make a compromised admin safe. It can, however, make trust changes visible: Certificate registration is preferred, CMDT fallback receipts are hash-chained, and `agent verify --live` can detect missing or retired keys.

### Stale Bundle

A signed bundle can become stale even if the signature remains valid. The `exp` claim bounds the validity window. `agent decay` gives agents a separate freshness signal based on activity, opportunities, cases, and chatter. Operators should treat signature validity and business freshness as different gates.

### Audience Leakage Through Inject

Posting CRM context into Slack can expose data to people who could not read it in Salesforce. `agent inject` mitigates that by enumerating channel members, mapping members to Salesforce users, intersecting FLS across the audience, and rendering only fields readable by that full audience. Unknown or external members block by default unless the caller explicitly acknowledges the waiver flag.

### Content-Version Tamper

Files are the largest tamper surface because a bundle may reference Salesforce `ContentVersion` rows. The manifest records SHA-256 values and Salesforce content version IDs. The JWS covers the manifest SHA, so the file hash references are transitively signed. `agent verify --deep` re-fetches ContentVersion bytes and re-hashes them when auth and a byte fetcher are configured.

## CMDT Overwrite Attack

CMDT fallback exists for editions and orgs where Certificate registration is not available. It is not the preferred path because metadata records are easier for admins or deployment tools to overwrite than Certificate records.

To reduce silent overwrite risk, CMDT registrations include a signed receipt and the previous receipt hash. This creates a hash chain across fallback registrations. A verifier can inspect the receipt chain and notice that a new CMDT key was inserted without continuity.

When running `agent verify --live`, operators should also inspect Salesforce Setup Audit Trail for key-registration changes, metadata deployments, and Certificate/CMDT edits around the bundle generation time. The CLI cannot make Setup Audit Trail immutable, but the live-check workflow is designed to force the right review point.

Use Certificate registration whenever the edition supports it. Use CMDT only after accepting that admin-level metadata mutation remains in the trust boundary.

## JWT And Bulk Gating

JWT bearer auth is useful for CI and scheduled agents, but the authenticating integration user may have broader access than the human the bundle represents. Treating integration-user FLS as end-user FLS is unsafe by default.

The CLI blocks JWT bundle generation unless `agent context --run-as-user <UserId>` is provided. That requirement makes the caller state which Salesforce user should define the read boundary.

Bulk and broad SOQL paths have the same concern. They can be fast, but they do not automatically prove user-scoped FLS for every field and row a bundle contains. The Apex companion, `SF360SafeRead`, closes this gap by executing Salesforce-side checks under the intended user boundary before data enters the bundle path.

If the Apex companion is absent, doctor reports a yellow Apex companion row with installation guidance. Use REST/UI API paths or install the companion before relying on JWT or Bulk-style reads for user-facing bundles.

## PKCE Requirement

The OAuth web flow uses PKCE and a loopback callback. PKCE protects the authorization code exchange when a native or local application cannot safely hold a client secret. This follows the model in RFC 8252 for native apps and RFC 7636 for Proof Key for Code Exchange.

The implementation requires a verifier when building the authorization URL and verifies token exchange behavior in `internal/auth/oauth_test.go`. Web auth should not be deployed without PKCE.

## File-Byte Attestation

The bundle signature does not directly sign each file byte. Instead, file bytes are hashed, file references are placed in the manifest, and the JWS claim signs the manifest SHA. That makes the file-byte hashes transitively covered by the signature.

`agent verify --deep` is the stronger mode. It re-fetches the Salesforce ContentVersion bytes and compares their SHA-256 values against the manifest. If the fetcher is not configured, verification returns a warning rather than pretending deep verification happened.

Use `--strict` when an agent will mutate downstream systems based on the bundle. Strict mode combines live key lookup, deep file checks, and expiration failure.

## `aud` Reservation

The bundle JWS has an `aud` claim. In v1 it is `agent-context`. The value is deliberately reserved as a union-typed claim, not as a one-off string. Future values such as `agent-mutation` can represent bundles whose contents authorize a more sensitive action than read-only context.

Consumers should reject unknown `aud` values unless they explicitly support that audience. Do not treat all valid signatures as equivalent authority.

## Competing-Tool Posture

This CLI should not claim to beat Salesforce's own MCP servers. Agentforce MCP and Salesforce DX MCP are often better for direct tool calls, metadata work, or first-party Salesforce workflows.

Doctor checks `AGENTFORCE_MCP`, `SFDX_MCP`, and local MCP registries such as Claude Code, Claude Desktop, and Cursor configs. When it finds Agentforce MCP or DX MCP, it reports the competing-tool row as yellow and says: "you may not need this CLI." That wording is intentional.

Use this CLI when the job is a portable signed bundle, offline verification, file-byte attestation, local-only doctor checks, or Slack audience-safe injection. Use the official MCP servers when the job is direct Salesforce action and their security model already fits.

## Locality And Logs

The CLI has no telemetry. Doctor runs locally. Logs are local. Profiles, cached rows, trust records, feedback, and generated bundles stay on disk unless the caller explicitly configures delivery, webhook output, Slack injection, or another outbound sink.

Locality is not the same as secrecy. Operators still need endpoint protection, disk encryption, token hygiene, shell history care, and review of generated bundle files before sharing them.

## Operator Checklist

1. Run `salesforce-headless-360-pp-cli doctor`.
2. Prefer Certificate-backed `trust register`.
3. Install `SF360SafeRead` before JWT or Bulk-backed bundle workflows.
4. Use `agent context --dry-run` for security review.
5. Use `agent verify --strict` before mutation workflows.
6. Treat CMDT fallback as an admin-trust boundary.
7. Inspect Setup Audit Trail after live trust changes.
8. Keep generated bundles out of unmanaged cloud logs.

## Write Path Security

v1.1 extends the read trust model to agent-authored Salesforce mutations. The important rule is the same as bundles: a valid Salesforce token is not treated as enough authority by itself. A write must pass the correct Salesforce-side access path, the local FLS and CRUD filter, signed intent generation, audit persistence, and replay/concurrency gates before it executes.

### Two Paths, Two Gates

Non-JWT auth modes use the Salesforce UI API write path by default. UI API record updates enforce object access, field-level security, record sharing, and layout-aware update semantics for the authenticated user, so it is the safest no-Apex path for ordinary human-scoped OAuth or `sf` CLI credentials.

JWT mode uses the Apex companion path. JWT auth commonly authenticates as an integration user whose permissions may exceed the human or agent persona represented by the write. For that reason, JWT writes refuse to run unless `--run-as-user <UserId>` is provided and the Apex companions are installed. `SF360SafeWrite.cls` and `SF360SafeUpsert.cls` execute guarded DML with Salesforce user-mode access semantics, then return normalized D9 errors to the CLI.

Direct REST SObject DML is not the default trust path for agent writes because it is too easy to confuse integration-user permissions with end-user FLS. Bulk paths are similarly guarded; v1.1 is single-record by design, and future bulk variants must pass through the same Apex/UI API gate.

### Idempotency

Agents retry after timeouts, network failures, and tool-call uncertainty. Without a durable idempotency key, retrying "create a Task" can create duplicate CRM activity. v1.1 requires a client-supplied `--idempotency-key` for `agent create`, `agent upsert`, and retryable convenience workflows.

The key is written to `SF360_Idempotency_Key__c` and used as an External ID upsert target. Salesforce then treats a retry with the same key as the same logical operation instead of a second create. A safe key strategy is a hash of intent, such as operation + target + normalized field payload + bounded timestamp. Do not use business identifiers, customer names, emails, patient IDs, or case descriptions as idempotency keys; keys can appear in audit logs and should not leak PII or PHI.

### Optimistic Concurrency

`agent update` and workflow verbs that patch known records use optimistic concurrency. The CLI fetches the current `LastModifiedDate`, signs the intended diff against that before-state, and sends an If-Match-style guard through the selected write path. If Salesforce changed the record after the before-state was fetched, the write is rejected as `CONFLICT_STALE_WRITE`.

`--force-stale` is an explicit opt-out for operators who have reviewed the race. When used, provenance records that stale-write protection was bypassed so later audit review can distinguish intentional override from normal conflict protection.

### Plan Mode Threat Model

Plan mode separates proposal from execution. `agent plan ...` builds a signed write plan with `aud=agent-mutation`, an expiration, intended operation, target, diff hash, and execution constraints. `agent sign-plan` appends countersignatures from other trusted keys. `agent execute-plan` verifies the plan signature, expiration, required countersignature count, intended audience, and write gates before mutating Salesforce.

The `aud` value prevents cross-use with read-only bundles: a signature meant for `agent-context` cannot authorize mutation, and a write plan cannot be replayed as a Customer 360 bundle. The expiration bounds stale approvals, and each execution signs a separate write intent with its own `jti`, preventing a countersigned plan from becoming an unbounded reusable permission slip.

### Bulk Gate Rationale

v1.1 write verbs operate on one record at a time. Any path that computes more than one affected record is blocked unless `--confirm-bulk <N>` is present and `N` exactly matches the computed count. This is intentionally redundant: broad writes are the largest operational risk for agent tooling, and the command must prove that the caller saw the count it is about to mutate.

v1.2 bulk variants should reuse the same gate, write-intent signing, idempotency, audit, and FLS path selection rather than adding a separate bulk-only trust model.

### Write D9 Error Codes

| Code | Meaning |
| --- | --- |
| `CONFLICT_STALE_WRITE` | The record changed after the before-state was fetched; rerun after reviewing the current Salesforce value. |
| `IDEMPOTENCY_KEY_REQUIRED` | A create, upsert, or retryable workflow write omitted `--idempotency-key`. |
| `FLS_WRITE_DENIED` | The acting user's object CRUD or field-level write access excluded the requested mutation. |
| `VALIDATION_RULE_REJECTED` | Salesforce rejected the write through a validation rule or equivalent DML validation. |
| `APEX_COMPANION_REQUIRED` | JWT mode attempted a write without the Apex write companion and `--run-as-user` safety path. |
| `BULK_CONFIRMATION_MISMATCH` | `--confirm-bulk N` was missing or did not match the computed affected record count. |
| `PLAN_SIGNATURE_INVALID` | `agent execute-plan` received an expired, tampered, wrong-audience, or insufficiently countersigned plan. |
| `WRITE_INTENT_AUDIT_FAILED` | The write intent audit row could not be persisted in required sync mode; HIPAA deployments block execution. |
