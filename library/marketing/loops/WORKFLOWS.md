# Agent workflow opportunities

The local CLI exposes all 68 operations in the official Loops OpenAPI 1.22.2 spec. The [command effect matrix](COMMANDS.md) is the executable API inventory. The compound commands below are possible agent workflows over those operations; the first three are implemented. A human can still use Loops UI features that have no public API endpoint.

| Workflow command | API inputs | Status and effect |
| --- | --- | --- |
| `team verify` | API-key team identity | Implemented; reads data. |
| `audit lifecycle` | Lists, segments, campaigns, workflows, event patterns, transactional templates | Implemented; reads counts. |
| `campaigns preflight` | Campaign detail and email Guardian | Implemented; reads readiness signals. |
| `contacts consent-check` | Contact find and suppression status | Feasible; would read data and return only consent and suppression state. |
| `segments inspect` | Segment definition and lists | Feasible; would read filter structure and target list. No contact enumeration is available. |
| `campaigns draft-plan` | Campaign create/update, email message, Guardian | Feasible; changes data only after explicit approval. |
| `report engagement` | Campaign, transactional, and workflow-node metrics | Feasible; reads aggregate sends, opens, clicks, bounces, and unsubscribes. |
| `events route-check` | Event patterns and workflow graph | Feasible; reads routing metadata. A true event test can trigger email and must use the send gate. |
| `transactional preflight` | Template detail, data variables, draft/published status, Guardian | Feasible; reads data. Preview-send and publication remain separately gated. |
| `themes impact-check` | Theme and affected email message metadata | Feasible; reads data before a theme update that can cascade. |
| `uploads asset-check` | Upload creation/completion metadata | Feasible; creates asset metadata. Uploading bytes to a signed URL is outside the Loops API command set. |

## Pairing with PostHog

Keep PostHog and Loops as separate CLIs with separate credentials and tenant checks. An agent can use the existing PostHog printed CLI to identify a product cohort, then use Loops to inspect communication state and prepare a reviewed audience or campaign. The handoff should be a private, minimal JSON file with approved identity mapping; no contact data belongs in the public CLI package.

Potential cross-CLI commands include `signup triage`, `unused-key queue`, `first-request gap`, `error follow-up candidates`, and `campaign-to-activation report`. They need product event instrumentation and a private identity join. Loops engagement metrics alone cannot prove that a person made a first product request. No Loops API operation grants product credits or identifies anonymous visitors from a work email.

## Boundaries

The Loops API supports contact lookup but no bulk contact listing, so a complete signup queue needs an approved private signup feed. The API exposes segment definitions, not a general segment-membership export. Campaign and transactional metrics report email engagement, not product activation. The CLI does not silently send events, previews, or emails to simulate a workflow.
