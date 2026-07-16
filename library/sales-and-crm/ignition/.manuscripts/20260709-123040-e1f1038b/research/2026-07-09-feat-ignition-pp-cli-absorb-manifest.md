# Ignition CLI Absorb Manifest

No existing CLI/MCP/SDK wraps the accounting Ignition (ignitionapp.com). The only "ignition"
tooling on GitHub targets Inductive Automation's unrelated industrial SCADA product
(WhiskeyHouse/ignition-mcp, ignition-devs/ignition-api, inductiveautomation/*). So there is no
competitor feature set to match — the absorb baseline is the app's own GraphQL surface plus the
two verified internal firm read paths (browser-harness ignitionapp domain skill;
ignition-final-invoice/ignition_gql.py).

## Absorbed (match the app's own read surface)
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | List proposals by status/client | ignition_gql.py pagedQuery(PROPOSAL) | (generated endpoint) search_index proposals | Offline SQLite, --json/--select, agent-native |
| 2 | List invoices | ignition_gql.py pagedQuery(INVOICE) | (generated endpoint) search_index invoices | Offline, filterable, low-token |
| 3 | List billing items | ignition_gql.py pagedQuery(BILLING_ITEM) | (generated endpoint) search_index billing_items | Offline, filterable |
| 4 | Client detail + proposals + forms + tags | HAR clientViewPage/client/clientForms/clientTags | (generated endpoint) clients + forms | One entity graph, --select |
| 5 | Client payment method on file | HAR paymentSettings / client(paymentMethods) | (generated endpoint) payments | Read card-on-file status |
| 6 | Client summary + rejected payments | HAR clientSummaryClient/clientSummaryRejectedPayments | (generated endpoint) clients summary | Collections signal |
| 7 | Proposal view detail | HAR proposalView | (generated endpoint) proposals view | Full proposal read |
| 8 | Practice + current user + revenue | HAR currentPractice/currentUser/billingDetails | (generated endpoint) currents / billings | Account context |
| 9 | Branding theme, forms templates, apps | HAR brandingTheme/formTemplates/appsWithCapability | (generated endpoint) brandings/forms/apps | Config reads |
| 10 | Full-text search across synced entities | framework | (behavior in ignition-pp-cli search) | FTS over proposals+invoices+clients |
| 11 | Arbitrary SQL over local mirror | framework | (behavior in ignition-pp-cli sql) | Composable analytics |

## Transcendence (local-join analytics no single GraphQL call gives you)
| # | Feature | Command | Buildability | Why Only We Can Do This | Long Description |
|---|---------|---------|--------------|-------------------------|------------------|
| 1 | Outstanding-billing rollup | outstanding | hand-code | Requires local join of invoices + billing_items filtered by unpaid status, summed per client | Use to find what's billed-but-unpaid across all clients. Read-only; does not send reminders. |
| 2 | Proposal pipeline by status | pipeline | hand-code | Requires local aggregation of the full PROPOSAL search index by status enum + client | Counts DRAFT/AWAITING_ACCEPTANCE/ACCEPTED/LOST across the book of business. |
| 3 | Per-client billing summary | client-billing | hand-code | Requires joining a client's proposals + invoices + billing items from the local store | One client's full money picture: proposed, invoiced, outstanding. Read-only. |
| 4 | Unbilled work finder | unbilled | hand-code | Requires diffing accepted proposals against existing invoices/billing items locally | Surfaces accepted-but-not-yet-invoiced work — directly serves Corben's "invoice more efficiently" goal. |
| 5 | Rejected-payment watch | rejected-payments | hand-code | Requires scanning clientSummaryRejectedPayments across synced clients | Lists clients with a rejected/failed payment so collections can follow up. Read-only. |

## Stubs
None. All transcendence rows are read-only local-store aggregations, fully buildable.

## Explicitly excluded (send-gate / NEVER)
- StripeAccountSessionCreate mutation (dropped from spec).
- Any proposal activate / "Send via email" / invoice create / charge / mutation. Out of scope by design; these stay behind Corben's send gate + safe_send, never in this CLI.
