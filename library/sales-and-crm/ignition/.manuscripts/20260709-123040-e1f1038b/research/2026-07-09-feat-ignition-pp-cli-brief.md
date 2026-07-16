# Ignition CLI Brief

## API Identity
- Domain: go.ignitionapp.com — Ignition (ignitionapp.com), the proposal/agreement/billing/payments platform for accounting & advisory firms. NOT Inductive Automation's industrial SCADA "Ignition".
- Users: the printing firm (owner + agents). Firm sells proposals, bills clients, collects payments through Ignition.
- Data profile: Apollo GraphQL SPA over a single `/graphql` BFF endpoint. Entities: clients, proposals, invoices, billing items, payments, services, forms, branding, practice/user.

## Reachability Risk
- Low for reads within an authed session. `probe-reachability` classified `standard_http` (0.65).
- Auth model is session-cookie + per-page `X-CSRF-Token` (from `meta[name=csrf-token]`) + an `X-Castle-Request-Token` bot-detection header. GraphQL introspection is DISABLED. The CSRF token is minted per page-load, so pure-HTTP replay can go stale — the existing verified read path (`~/.openclaw/workspace/projects/ignition-final-invoice/ignition_gql.py`) runs the same-origin fetch INSIDE the authed page over CDP for exactly this reason. HTTP-replay reliability is the main caveat; the CLI carries cookie+CSRF via config and documents refresh.

## Top Workflows
1. Invoicing: list every invoice / billing item, see status + amount + client, find unbilled or outstanding items. (Corben's stated goal: invoice clients more efficiently.)
2. Proposal management: list proposals by status (DRAFT / AWAITING_ACCEPTANCE / ACCEPTED / LOST), per client.
3. Client lookup: resolve a client, its proposals, forms, payment method on file, tags.
4. Payments: read payment settings, rejected payments, revenue snapshot.

## Table Stakes
- Read every core entity with `--json` for agent consumption and `--select` to trim deep GraphQL payloads.
- Local SQLite mirror so agents can `sql`/`search` across proposals+invoices+clients offline without re-hitting the CSRF-gated endpoint each time.

## Data Layer
- Primary entities: clients, proposals, invoices, billing_items, payments.
- Sync cursor: none server-side; full paged pulls of the search index (perPage 200).
- FTS/search: proposal/invoice/billing-item name + client name.

## Source Priority
- Single source (ignition). HAR-first (user-provided capture of the authed session).

## Product Thesis
- Name: ignition-pp-cli — agent-native read interface to Ignition proposals, invoices, billing, clients, and payments.
- Why it should exist: Ignition has no public API; agents currently drive it through fragile browser automation. A codified CLI + MCP gives the firm's agents a stable, low-token interface to navigate Ignition and reason about invoicing. READ + DRAFT only by design — no send/charge/mutation commands (Corben's send gate + the "live billing surface" NEVERs). The one mutation in the capture (StripeAccountSessionCreate) was dropped from the spec.

## Build Priorities
1. Data layer + sync + search + sql across clients/proposals/invoices/billing_items/payments.
2. Absorbed read operations (all captured GraphQL queries + verified paged-search operations).
3. Transcendence: local-join analytics that answer invoicing questions a single GraphQL call cannot (unbilled/outstanding rollups, proposal pipeline by status, per-client billing summary).

## Constraints (hard)
- READ-ONLY + DRAFT. No command sends email, activates a proposal, creates/charges an invoice, or mutates client state. Any future write path stays behind Corben's send gate + safe_send, never in this CLI.
- No client PII in generated code, README, or archived manuscripts. HAR response bodies (used for research only) contain real client data; strip before archive.
