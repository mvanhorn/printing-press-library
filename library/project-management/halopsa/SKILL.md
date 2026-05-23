---
name: pp-halopsa
description: "Every HaloPSA, HaloITSM and HaloCRM feature, plus a local SQLite store and cross-entity views the API can't return. Trigger phrases: `triage my Halo queue`, `check SLA breaches in HaloPSA`, `who's overloaded in Halo`, `client card for Acme`, `Halo contract burn-down`, `what changed in Halo since this morning`, `use halopsa`, `run halopsa`."
author: "Damien Stevens"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - halopsa-pp-cli
---

# HaloPSA — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `halopsa-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install halopsa --cli-only
   ```
2. Verify: `halopsa-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails before this CLI has a public-library category, install Node or use the category-specific Go fallback after publish.

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

Wraps the full Halo REST API (952 endpoints across tickets, clients, assets, contracts, time, KB, and workflows) with offline-first search, agent-native JSON output, and cross-entity commands like `triage`, `client card`, and `contracts burn` that join tables Halo's UI scatters across five tabs.

## When to Use This CLI

Reach for halopsa-pp-cli when an agent needs to triage, dispatch, or report against a HaloPSA / HaloITSM / HaloCRM tenant without clicking through the web UI. It is the right tool for cross-entity questions ("who's overloaded", "which tickets are about to breach", "which client is burning their contract"), bulk operations (stale-ticket close, batch action posts, batch time entries), and ETLs that previously required hand-rolled scripts. It is NOT the right tool for end-user portal browsing or for tenants you don't have API credentials to.

## Don't reach for this CLI when

- End-user portal browsing (raising a ticket as an end-user, browsing your own tickets). Use Halo's web portal.
- The tenant has no API application configured, or no `HALOPSA_CLIENT_ID` / `HALOPSA_CLIENT_SECRET` available to the agent.
- The task is on a non-Halo PSA (Autotask, ConnectWise, etc.) — this CLI is Halo-specific even though several integration modules surface foreign-system data.
- Third-party integration sub-commands (e.g. `addigy`, `aws`, `connectwise-rmm`) when the corresponding Halo integration module isn't enabled on the tenant — the commands will return empty / 404 because the integration isn't wired upstream.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Cross-entity dispatch views
- **`triage`** — See per-agent open ticket load, stale tickets, and 24-hour SLA-breach count in one table — the dispatcher view Halo's UI scatters across five tabs.

  _Reach for this when an agent asks 'who should I assign this P1 to' or 'where are we bleeding'. One call, one screen._

  ```bash
  halopsa-pp-cli triage --team Support --json
  ```
- **`tickets age-out`** — Find tickets stale in a status for N days, preview them, then bulk-close with a templated action via --apply.

  _Reach for this on the Monday queue cleanse to close 30 stale tickets in one command instead of 30 clicks._

  ```bash
  halopsa-pp-cli tickets age-out --status "Awaiting Customer" --stale-days 14 --action-note "Auto-closing per policy" --apply
  ```
- **`sla breaching`** — List tickets whose targetdate falls in the next N hours, sorted by time-to-breach, with agent + client + current status.

  _Reach for this on Friday afternoon or anytime before a hand-off to pre-empt SLA breaches._

  ```bash
  halopsa-pp-cli sla breaching --within 24h --team Support --json
  ```
- **`agent workload`** — Per-agent: open tickets, tickets touched this week, billable hours logged, oldest open ticket age.

  _Reach for this when rebalancing the queue or asking 'who's overloaded'._

  ```bash
  halopsa-pp-cli agent workload --team Support --json
  ```

### Per-client situational awareness
- **`client card`** — One panel: client + sites + active tickets + open contracts + contract hours remaining + recent KB articles linked to their tickets + asset count.

  _Reach for this on every client call. Open it before answering the phone, paste it into ticket notes._

  ```bash
  halopsa-pp-cli client card "Acme Corp" --json
  ```
- **`asset history`** — Every ticket that touched this asset, chronological, with agent and time logged.

  _Reach for this when a machine keeps coming back to the queue — the pattern lives in the history, not in the latest ticket._

  ```bash
  halopsa-pp-cli asset history LAP-0042 --json
  ```
- **`kbarticle suggest`** — FTS5-rank KB articles against a ticket's summary + details + last action text; print top 5 with snippets.

  _Reach for this mid-call when a known fix probably exists but you don't remember the exact KB title._

  ```bash
  halopsa-pp-cli kbarticle suggest --ticket 12345 --limit 5
  ```

### Local-only analytics
- **`time gaps`** — List tickets the agent touched this week that have zero time logged on them.

  _Reach for this on Friday before submitting the timesheet. Stops 'I know I worked that ticket but where is it' archaeology._

  ```bash
  halopsa-pp-cli time gaps --agent me --week current
  ```
- **`contracts burn`** — Per contract: hours bank, hours consumed this period (sum of billable time on that client's tickets), days remaining, projected overage.

  _Reach for this mid-month before a client conversation — know whether they're tracking over their bank._

  ```bash
  halopsa-pp-cli contracts burn --client "Acme Corp" --month current --json
  ```
- **`rules dump`** — Print every ticket rule and workflow as readable flat text — conditions → actions, one block per rule.

  _Reach for this during quarterly automation audits or when investigating an unexpected routing._

  ```bash
  halopsa-pp-cli rules dump --workflow "New Ticket" > rules-audit.txt
  ```
- **`tickets changed-since`** — Tickets where any action or status change occurred since timestamp, grouped by ticket.

  _Reach for this after a meeting or when coming back from lunch. 'What did I miss' in one call._

  ```bash
  halopsa-pp-cli tickets changed-since 09:00 --mine --json
  ```
- **`standup`** — Per-agent for the window: tickets closed, tickets reopened, time logged, top client.

  _Reach for this the moment before standup. One paste, everyone sees yesterday's progress._

  ```bash
  halopsa-pp-cli standup --team Support --since yesterday
  ```
- **`client overlay`** — Rank all clients by a chosen metric (open tickets, stale, SLA at-risk, hours over bank). Top N out.

  _Reach for this when looking for the next escalation to invest time in. Whichever client is on top is the one to call._

  ```bash
  halopsa-pp-cli client overlay --metric open_tickets --top 10 --json
  ```

## Command Reference

**actions** — Manage actions

- `halopsa-pp-cli actions create` — Create
- `halopsa-pp-cli actions create-reaction` — Create reaction
- `halopsa-pp-cli actions create-review` — Create review
- `halopsa-pp-cli actions delete` — Delete
- `halopsa-pp-cli actions get` — Use this to return a single instance of Actions.<br> 				Requires authentication.
- `halopsa-pp-cli actions list` — Use this to return multiple Actions.<br> 				Requires authentication.

**addigy** — Manage addigy

- `halopsa-pp-cli addigy create` — Create
- `halopsa-pp-cli addigy list` — List

**addigy-details** — Manage addigy details

- `halopsa-pp-cli addigy-details create` — Create
- `halopsa-pp-cli addigy-details delete` — Delete
- `halopsa-pp-cli addigy-details get` — Get
- `halopsa-pp-cli addigy-details list` — List

**address** — Manage address

- `halopsa-pp-cli address create` — Create
- `halopsa-pp-cli address delete` — Delete
- `halopsa-pp-cli address get` — Use this to return a single instance of AddressStore.<br> 				Requires authentication.
- `halopsa-pp-cli address list` — Use this to return multiple AddressStore.<br> 				Requires authentication.

**addressbook** — Manage addressbook

- `halopsa-pp-cli addressbook create` — Create
- `halopsa-pp-cli addressbook delete` — Delete
- `halopsa-pp-cli addressbook get` — Get
- `halopsa-pp-cli addressbook list` — List

**adobe-acrobat-details** — Manage adobe acrobat details

- `halopsa-pp-cli adobe-acrobat-details create` — Create
- `halopsa-pp-cli adobe-acrobat-details delete` — Delete
- `halopsa-pp-cli adobe-acrobat-details get` — Get
- `halopsa-pp-cli adobe-acrobat-details list` — List

**adobe-commerce-details** — Manage adobe commerce details

- `halopsa-pp-cli adobe-commerce-details create` — Create
- `halopsa-pp-cli adobe-commerce-details delete` — Delete
- `halopsa-pp-cli adobe-commerce-details get` — Get
- `halopsa-pp-cli adobe-commerce-details list` — List

**adobe-commerce-integration** — Manage adobe commerce integration

- `halopsa-pp-cli adobe-commerce-integration create` — Create
- `halopsa-pp-cli adobe-commerce-integration list` — List

**agent** — Manage agent

- `halopsa-pp-cli agent create` — Create
- `halopsa-pp-cli agent create-clearcache` — Create clearcache
- `halopsa-pp-cli agent delete` — Delete
- `halopsa-pp-cli agent get` — Use this to return a single instance of Uname.<br> 				Requires authentication.
- `halopsa-pp-cli agent list` — Use this to return multiple Uname.<br> 				Requires authentication.
- `halopsa-pp-cli agent list-me` — List me

**agent-check-in** — Manage agent check in

- `halopsa-pp-cli agent-check-in create` — Create
- `halopsa-pp-cli agent-check-in get` — Use this to return a single instance of AgentCheckIn.<br> 				Requires authentication.
- `halopsa-pp-cli agent-check-in list` — Use this to return multiple AgentCheckIn.<br> 				Requires authentication.

**agent-event-subscription** — Manage agent event subscription

- `halopsa-pp-cli agent-event-subscription create` — Create
- `halopsa-pp-cli agent-event-subscription delete` — Delete
- `halopsa-pp-cli agent-event-subscription get` — Get
- `halopsa-pp-cli agent-event-subscription list` — List

**agent-image** — Manage agent image

- `halopsa-pp-cli agent-image <id>` — Use this to return a single instance of Uname.<br> 				Requires authentication.

**agent-presence-rule** — Manage agent presence rule

- `halopsa-pp-cli agent-presence-rule` — List

**agent-presence-subscription** — Manage agent presence subscription

- `halopsa-pp-cli agent-presence-subscription create` — Create
- `halopsa-pp-cli agent-presence-subscription delete` — Delete
- `halopsa-pp-cli agent-presence-subscription get-uname-presence-subscription` — Get uname presence subscription
- `halopsa-pp-cli agent-presence-subscription list` — List

**aisuggestion** — Manage aisuggestion

- `halopsa-pp-cli aisuggestion create` — Create
- `halopsa-pp-cli aisuggestion delete` — Delete
- `halopsa-pp-cli aisuggestion get` — Get
- `halopsa-pp-cli aisuggestion list` — List

**alemba** — Manage alemba

- `halopsa-pp-cli alemba` — List

**amazon-seller-details** — Manage amazon seller details

- `halopsa-pp-cli amazon-seller-details create` — Create
- `halopsa-pp-cli amazon-seller-details delete` — Delete
- `halopsa-pp-cli amazon-seller-details get` — Get
- `halopsa-pp-cli amazon-seller-details list` — List

**application** — Manage application

- `halopsa-pp-cli application create` — Create
- `halopsa-pp-cli application create-federatedcredentials` — Create federatedcredentials
- `halopsa-pp-cli application delete` — Delete
- `halopsa-pp-cli application get` — Use this to return a single instance of NHD_Identity_Application.<br> 				Requires authentication.
- `halopsa-pp-cli application list` — List

**appointment** — Manage appointment

- `halopsa-pp-cli appointment create` — Create
- `halopsa-pp-cli appointment create-booking` — Create booking
- `halopsa-pp-cli appointment create-generate` — Create generate
- `halopsa-pp-cli appointment delete` — Delete specific Appointment.<br> 				Requires authentication.
- `halopsa-pp-cli appointment get` — Use this to return a single instance of Appointment.<br> 				Requires authentication.
- `halopsa-pp-cli appointment list` — Use this to return multiple Appointment.<br> 				Requires authentication.
- `halopsa-pp-cli appointment list-booking` — List booking

**approval-process** — Manage approval process

- `halopsa-pp-cli approval-process create` — Create
- `halopsa-pp-cli approval-process delete` — Delete
- `halopsa-pp-cli approval-process get` — Use this to return a single instance of ApprovalProcess.<br> 				Requires authentication.
- `halopsa-pp-cli approval-process list` — Use this to return multiple ApprovalProcess.<br> 				Requires authentication.

**approval-process-rule** — Manage approval process rule

- `halopsa-pp-cli approval-process-rule create` — Create
- `halopsa-pp-cli approval-process-rule delete` — Delete
- `halopsa-pp-cli approval-process-rule get` — Use this to return a single instance of ApprovalProcessRule.<br> 				Requires authentication.
- `halopsa-pp-cli approval-process-rule list` — Use this to return multiple ApprovalProcessRule.<br> 				Requires authentication.

**area-azure-tenant** — Manage area azure tenant

- `halopsa-pp-cli area-azure-tenant` — Use this to return multiple AreaAzureTenant.<br> 				Requires authentication.

**area-request-type** — Manage area request type

- `halopsa-pp-cli area-request-type get` — Use this to return a single instance of AreaRequestType.<br> 				Requires authentication.
- `halopsa-pp-cli area-request-type list` — List

**armis** — Manage armis

- `halopsa-pp-cli armis` — List

**armis-details** — Manage armis details

- `halopsa-pp-cli armis-details create` — Create
- `halopsa-pp-cli armis-details delete` — Delete
- `halopsa-pp-cli armis-details get` — Get
- `halopsa-pp-cli armis-details list` — List

**arrow-sphere-details** — Manage arrow sphere details

- `halopsa-pp-cli arrow-sphere-details create` — Create
- `halopsa-pp-cli arrow-sphere-details delete` — Delete
- `halopsa-pp-cli arrow-sphere-details get` — Get
- `halopsa-pp-cli arrow-sphere-details list` — List

**asset** — Manage asset

- `halopsa-pp-cli asset create` — Create
- `halopsa-pp-cli asset delete` — Delete
- `halopsa-pp-cli asset get` — Use this to return a single instance of Device.<br> 				Requires authentication.
- `halopsa-pp-cli asset list` — Use this to return multiple Device.<br> 				Requires authentication.
- `halopsa-pp-cli asset list-getallsoftwareversions` — List getallsoftwareversions
- `halopsa-pp-cli asset list-nexttag` — List nexttag

**asset-change** — Manage asset change

- `halopsa-pp-cli asset-change create` — Create
- `halopsa-pp-cli asset-change list` — Use this to return multiple DeviceChange.<br> 				Requires authentication.

**asset-group** — Manage asset group

- `halopsa-pp-cli asset-group create` — Create
- `halopsa-pp-cli asset-group delete` — Delete
- `halopsa-pp-cli asset-group get` — Use this to return a single instance of Generic.<br> 				Requires authentication.
- `halopsa-pp-cli asset-group list` — Use this to return multiple Generic.<br> 				Requires authentication.

**asset-software** — Manage asset software

- `halopsa-pp-cli asset-software` — Use this to return multiple DeviceApplications.<br> 				Requires authentication.

**asset-type** — Manage asset type

- `halopsa-pp-cli asset-type create` — Create
- `halopsa-pp-cli asset-type delete` — Delete
- `halopsa-pp-cli asset-type get` — Use this to return a single instance of Xtype.<br> 				Requires authentication.
- `halopsa-pp-cli asset-type list` — Use this to return multiple Xtype.<br> 				Requires authentication.

**asset-type-info** — Manage asset type info

- `halopsa-pp-cli asset-type-info` — Use this to return multiple Xtype.<br> 				Requires authentication.

**asset-type-mappings** — Manage asset type mappings

- `halopsa-pp-cli asset-type-mappings get` — Use this to return a single instance of XTypeMapping.<br> 				Requires authentication.
- `halopsa-pp-cli asset-type-mappings list` — List

**att** — Manage att

- `halopsa-pp-cli att` — List

**attachment** — Manage attachment

- `halopsa-pp-cli attachment create` — Create
- `halopsa-pp-cli attachment create-document` — Create document
- `halopsa-pp-cli attachment create-gets3presignedurl` — Create gets3presignedurl
- `halopsa-pp-cli attachment create-image` — Create image
- `halopsa-pp-cli attachment create-presignedurluploadcomplete` — Create presignedurluploadcomplete
- `halopsa-pp-cli attachment delete` — Delete
- `halopsa-pp-cli attachment delete-document` — Delete document
- `halopsa-pp-cli attachment delete-image` — Delete image
- `halopsa-pp-cli attachment get` — Use this to return a single instance of Attachment.<br> 				Requires authentication.
- `halopsa-pp-cli attachment get-document` — Get document
- `halopsa-pp-cli attachment get-image` — Get image
- `halopsa-pp-cli attachment get-nhserver` — Get nhserver
- `halopsa-pp-cli attachment list` — Use this to return multiple Attachment.<br> 				Requires authentication.
- `halopsa-pp-cli attachment list-image` — List image

**audit** — Manage audit

- `halopsa-pp-cli audit create` — Create
- `halopsa-pp-cli audit delete` — Delete
- `halopsa-pp-cli audit get` — Use this to return a single instance of Audit.<br> 				Requires authentication.
- `halopsa-pp-cli audit list` — List

**auth-info** — Manage auth info

- `halopsa-pp-cli auth-info` — List

**automation** — Manage automation

- `halopsa-pp-cli automation create` — Create
- `halopsa-pp-cli automation create-runbookid` — Create runbookid
- `halopsa-pp-cli automation delete` — Delete
- `halopsa-pp-cli automation get` — Get
- `halopsa-pp-cli automation list` — List

**avalara-details** — Manage avalara details

- `halopsa-pp-cli avalara-details create` — Create
- `halopsa-pp-cli avalara-details delete` — Delete
- `halopsa-pp-cli avalara-details get` — Get
- `halopsa-pp-cli avalara-details list` — List

**aws** — Manage aws

- `halopsa-pp-cli aws` — List

**awsdetails** — Manage awsdetails

- `halopsa-pp-cli awsdetails create` — Create
- `halopsa-pp-cli awsdetails delete` — Delete
- `halopsa-pp-cli awsdetails get` — Get
- `halopsa-pp-cli awsdetails list` — List

**azure-delta** — Manage azure delta

- `halopsa-pp-cli azure-delta create` — Create
- `halopsa-pp-cli azure-delta delete` — Delete
- `halopsa-pp-cli azure-delta get` — Get
- `halopsa-pp-cli azure-delta list` — List

**azure-dev-ops-details** — Manage azure dev ops details

- `halopsa-pp-cli azure-dev-ops-details create` — Create
- `halopsa-pp-cli azure-dev-ops-details delete` — Delete
- `halopsa-pp-cli azure-dev-ops-details get` — Use this to return a single instance of AzureDevOpsDetails.<br> 				Requires authentication.
- `halopsa-pp-cli azure-dev-ops-details list` — List

**azure-translate** — Manage azure translate

- `halopsa-pp-cli azure-translate create` — Create
- `halopsa-pp-cli azure-translate list` — List

**azureadconnection** — Manage azureadconnection

- `halopsa-pp-cli azureadconnection create` — Create
- `halopsa-pp-cli azureadconnection delete` — Delete
- `halopsa-pp-cli azureadconnection get` — Use this to return a single instance of AzureADConnection.<br> 				Requires authentication.
- `halopsa-pp-cli azureadconnection list` — Use this to return multiple AzureADConnection.<br> 				Requires authentication.

**azureadmapping** — Manage azureadmapping

- `halopsa-pp-cli azureadmapping` — Use this to return multiple AzureADMapping.<br> 				Requires authentication.

**background-task** — Manage background task

- `halopsa-pp-cli background-task <id>` — Get

**billing-template** — Manage billing template

- `halopsa-pp-cli billing-template create` — Create
- `halopsa-pp-cli billing-template delete` — Delete
- `halopsa-pp-cli billing-template get` — Use this to return a single instance of ContractTemplateHeader.<br> 				Requires authentication.
- `halopsa-pp-cli billing-template list` — List

**booking-type** — Manage booking type

- `halopsa-pp-cli booking-type` — Use this to return multiple BookingType.<br> 				Requires authentication.

**bookmark** — Manage bookmark

- `halopsa-pp-cli bookmark create` — Create
- `halopsa-pp-cli bookmark get` — Get

**budget-type** — Manage budget type

- `halopsa-pp-cli budget-type create` — Create
- `halopsa-pp-cli budget-type delete` — Delete
- `halopsa-pp-cli budget-type get` — Use this to return a single instance of BudgetType.<br> 				Requires authentication.
- `halopsa-pp-cli budget-type list` — Use this to return multiple BudgetType.<br> 				Requires authentication.

**bulk-email** — Manage bulk email

- `halopsa-pp-cli bulk-email get` — Use this to return a single instance of BulkEmail.<br> 				Requires authentication.
- `halopsa-pp-cli bulk-email list` — List

**business-central-details** — Manage business central details

- `halopsa-pp-cli business-central-details create` — Create
- `halopsa-pp-cli business-central-details delete` — Delete
- `halopsa-pp-cli business-central-details get` — Use this to return a single instance of BusinessCentralDetails.<br> 				Requires authentication.
- `halopsa-pp-cli business-central-details list` — Use this to return multiple BusinessCentralDetails.<br> 				Requires authentication.

**cab** — Manage cab

- `halopsa-pp-cli cab create` — Create
- `halopsa-pp-cli cab delete` — Delete
- `halopsa-pp-cli cab get` — Use this to return a single instance of CabHeader.<br> 				Requires authentication.
- `halopsa-pp-cli cab list` — Use this to return multiple CabHeader.<br> 				Requires authentication.

**cabmember** — Manage cabmember

- `halopsa-pp-cli cabmember` — List

**cabrole** — Manage cabrole

- `halopsa-pp-cli cabrole` — List

**call-log** — Manage call log

- `halopsa-pp-cli call-log create` — Create
- `halopsa-pp-cli call-log get` — Use this to return a single instance of CallLog.<br> 				Requires authentication.
- `halopsa-pp-cli call-log list` — Use this to return multiple CallLog.<br> 				Requires authentication.

**call-script** — Manage call script

- `halopsa-pp-cli call-script create` — Create
- `halopsa-pp-cli call-script delete` — Delete
- `halopsa-pp-cli call-script get` — Use this to return a single instance of ScriptHeader.<br> 				Requires authentication.
- `halopsa-pp-cli call-script list` — List

**canned-text** — Manage canned text

- `halopsa-pp-cli canned-text create` — Create
- `halopsa-pp-cli canned-text create-cannedtext` — Create cannedtext
- `halopsa-pp-cli canned-text delete` — Delete
- `halopsa-pp-cli canned-text get` — Use this to return a single instance of CannedText.<br> 				Requires authentication.
- `halopsa-pp-cli canned-text list` — Use this to return multiple CannedText.<br> 				Requires authentication.

**category** — Manage category

- `halopsa-pp-cli category create` — Create
- `halopsa-pp-cli category delete` — Delete
- `halopsa-pp-cli category get` — Use this to return a single instance of CategoryDetail.<br> 				Requires authentication.
- `halopsa-pp-cli category list` — Use this to return multiple CategoryDetail.<br> 				Requires authentication.

**certificate** — Manage certificate

- `halopsa-pp-cli certificate create` — Create
- `halopsa-pp-cli certificate delete` — Delete
- `halopsa-pp-cli certificate get` — Use this to return a single instance of Certificate.<br> 				Requires authentication.
- `halopsa-pp-cli certificate list` — List

**change-calendar** — Manage change calendar

- `halopsa-pp-cli change-calendar` — List

**charge-rate** — Manage charge rate

- `halopsa-pp-cli charge-rate get` — Use this to return a single instance of ChargeRate.<br> 				Requires authentication.
- `halopsa-pp-cli charge-rate list` — Use this to return multiple ChargeRate.<br> 				Requires authentication.

**chat** — Manage chat

- `halopsa-pp-cli chat create` — Create
- `halopsa-pp-cli chat get` — Get
- `halopsa-pp-cli chat list` — Use this to return multiple LiveChatHeader.<br> 				Requires authentication.

**chat-flow** — Manage chat flow

- `halopsa-pp-cli chat-flow` — Create

**chat-matching-data** — Manage chat matching data

- `halopsa-pp-cli chat-matching-data` — Create

**chat-message** — Manage chat message

- `halopsa-pp-cli chat-message create` — Create
- `halopsa-pp-cli chat-message create-chatmessage` — Create chatmessage
- `halopsa-pp-cli chat-message list` — Use this to return multiple LiveChatMsg.<br> 				Requires authentication.

**chat-profile** — Manage chat profile

- `halopsa-pp-cli chat-profile create` — Create
- `halopsa-pp-cli chat-profile delete` — Delete
- `halopsa-pp-cli chat-profile get` — Use this to return a single instance of ChatProfile.<br> 				Requires authentication.
- `halopsa-pp-cli chat-profile list` — Use this to return multiple ChatProfile.<br> 				Requires authentication.

**client** — Manage client

- `halopsa-pp-cli client create` — Create
- `halopsa-pp-cli client create-newaccountsid` — Create newaccountsid
- `halopsa-pp-cli client create-paymentmethodupdate` — Create paymentmethodupdate
- `halopsa-pp-cli client delete` — Delete
- `halopsa-pp-cli client get` — Use this to return a single instance of Area.<br> 				Requires authentication.
- `halopsa-pp-cli client list` — Use this to return multiple Area.<br> 				Requires authentication.
- `halopsa-pp-cli client list-me` — List me

**client-cache** — Manage client cache

- `halopsa-pp-cli client-cache` — List

**client-contract** — Manage client contract

- `halopsa-pp-cli client-contract create` — Create
- `halopsa-pp-cli client-contract create-clientcontract` — Create clientcontract
- `halopsa-pp-cli client-contract create-clientcontract-2` — Create clientcontract 2
- `halopsa-pp-cli client-contract delete` — Delete
- `halopsa-pp-cli client-contract get` — Use this to return a single instance of ContractHeader.<br> 				Requires authentication.
- `halopsa-pp-cli client-contract list` — Use this to return multiple ContractHeader.<br> 				Requires authentication.

**client-prepay** — Manage client prepay

- `halopsa-pp-cli client-prepay create` — Create
- `halopsa-pp-cli client-prepay delete` — Delete
- `halopsa-pp-cli client-prepay get` — Use this to return a single instance of PrepayHistory.<br> 				Requires authentication.
- `halopsa-pp-cli client-prepay list` — Use this to return multiple PrepayHistory.<br> 				Requires authentication.

**config-commit** — Manage config commit

- `halopsa-pp-cli config-commit create` — Create
- `halopsa-pp-cli config-commit delete` — Delete
- `halopsa-pp-cli config-commit get` — Use this to return a single instance of ConfigCommit.<br> 				Requires authentication.
- `halopsa-pp-cli config-commit list` — Use this to return multiple ConfigCommit.<br> 				Requires authentication.

**confirm-closure** — Manage confirm closure

- `halopsa-pp-cli confirm-closure create` — Create
- `halopsa-pp-cli confirm-closure delete` — Delete
- `halopsa-pp-cli confirm-closure get` — Use this to return a single instance of ConfirmClosure.<br> 				Requires authentication.
- `halopsa-pp-cli confirm-closure list` — List

**confluence-details** — Manage confluence details

- `halopsa-pp-cli confluence-details create` — Create
- `halopsa-pp-cli confluence-details delete` — Delete
- `halopsa-pp-cli confluence-details get` — Get
- `halopsa-pp-cli confluence-details list` — List

**connected-instance** — Manage connected instance

- `halopsa-pp-cli connected-instance create` — Create
- `halopsa-pp-cli connected-instance delete` — Delete
- `halopsa-pp-cli connected-instance get` — Use this to return a single instance of ConnectedInstance.<br> 				Requires authentication.
- `halopsa-pp-cli connected-instance list` — List

**consignment** — Manage consignment

- `halopsa-pp-cli consignment create` — Create
- `halopsa-pp-cli consignment delete` — Delete
- `halopsa-pp-cli consignment get` — Use this to return a single instance of ConsignmentHeader.<br> 				Requires authentication.
- `halopsa-pp-cli consignment list` — Use this to return multiple ConsignmentHeader.<br> 				Requires authentication.

**contactgroup** — Manage contactgroup

- `halopsa-pp-cli contactgroup create` — Create
- `halopsa-pp-cli contactgroup delete` — Delete
- `halopsa-pp-cli contactgroup get` — Get
- `halopsa-pp-cli contactgroup list` — List

**contactgroupcontact** — Manage contactgroupcontact

- `halopsa-pp-cli contactgroupcontact create` — Create
- `halopsa-pp-cli contactgroupcontact delete` — Delete
- `halopsa-pp-cli contactgroupcontact get` — Get
- `halopsa-pp-cli contactgroupcontact list` — List

**contract-rule** — Manage contract rule

- `halopsa-pp-cli contract-rule create` — Create
- `halopsa-pp-cli contract-rule delete` — Delete
- `halopsa-pp-cli contract-rule get` — Get
- `halopsa-pp-cli contract-rule list` — List

**contract-schedule** — Manage contract schedule

- `halopsa-pp-cli contract-schedule create` — Create
- `halopsa-pp-cli contract-schedule delete` — Delete
- `halopsa-pp-cli contract-schedule get` — Use this to return a single instance of ContractSchedule.<br> 				Requires authentication.
- `halopsa-pp-cli contract-schedule list` — List

**contract-schedule-plan** — Manage contract schedule plan

- `halopsa-pp-cli contract-schedule-plan create` — Create
- `halopsa-pp-cli contract-schedule-plan delete` — Delete
- `halopsa-pp-cli contract-schedule-plan get` — Use this to return a single instance of ContractSchedulePlan.<br> 				Requires authentication.
- `halopsa-pp-cli contract-schedule-plan list` — List

**cost-centres** — Manage cost centres

- `halopsa-pp-cli cost-centres create` — Create
- `halopsa-pp-cli cost-centres delete` — Delete
- `halopsa-pp-cli cost-centres get` — Use this to return a single instance of Costcentres.<br> 				Requires authentication.
- `halopsa-pp-cli cost-centres list` — List

**criteria-group** — Manage criteria group

- `halopsa-pp-cli criteria-group` — List

**crmnote** — Manage crmnote

- `halopsa-pp-cli crmnote create` — Create
- `halopsa-pp-cli crmnote delete` — Delete
- `halopsa-pp-cli crmnote get` — Use this to return a single instance of AreaNote.<br> 				Requires authentication.
- `halopsa-pp-cli crmnote list` — Use this to return multiple AreaNote.<br> 				Requires authentication.

**cspconsumption-data** — Manage cspconsumption data

- `halopsa-pp-cli cspconsumption-data create` — Create
- `halopsa-pp-cli cspconsumption-data create-cspconsumptiondata` — Create cspconsumptiondata
- `halopsa-pp-cli cspconsumption-data delete` — Delete
- `halopsa-pp-cli cspconsumption-data delete-cspconsumptiondata` — Delete cspconsumptiondata
- `halopsa-pp-cli cspconsumption-data get` — Get
- `halopsa-pp-cli cspconsumption-data list` — List

**cspinvoice** — Manage cspinvoice

- `halopsa-pp-cli cspinvoice create` — Create
- `halopsa-pp-cli cspinvoice delete` — Delete
- `halopsa-pp-cli cspinvoice get` — Get
- `halopsa-pp-cli cspinvoice list` — List

**cspsubscription-pricing** — Manage cspsubscription pricing

- `halopsa-pp-cli cspsubscription-pricing` — Create

**csvtemplate** — Manage csvtemplate

- `halopsa-pp-cli csvtemplate create` — Create
- `halopsa-pp-cli csvtemplate delete` — Delete
- `halopsa-pp-cli csvtemplate get` — Use this to return a single instance of CSVTemplate.<br> 				Requires authentication.
- `halopsa-pp-cli csvtemplate list` — List

**currency** — Manage currency

- `halopsa-pp-cli currency create` — Create
- `halopsa-pp-cli currency delete` — Delete
- `halopsa-pp-cli currency get` — Use this to return a single instance of Currency.<br> 				Requires authentication.
- `halopsa-pp-cli currency list` — List

**custom-button** — Manage custom button

- `halopsa-pp-cli custom-button create` — Create
- `halopsa-pp-cli custom-button delete` — Delete
- `halopsa-pp-cli custom-button get` — Use this to return a single instance of CustomButton.<br> 				Requires authentication.
- `halopsa-pp-cli custom-button list` — Use this to return multiple CustomButton.<br> 				Requires authentication.

**custom-button-audit** — Manage custom button audit

- `halopsa-pp-cli custom-button-audit` — Create

**custom-integration** — Manage custom integration

- `halopsa-pp-cli custom-integration create` — Create
- `halopsa-pp-cli custom-integration delete` — Delete
- `halopsa-pp-cli custom-integration get` — Use this to return a single instance of OutboundIntegration.<br> 				Requires authentication.
- `halopsa-pp-cli custom-integration list` — List

**custom-integration-method** — Manage custom integration method

- `halopsa-pp-cli custom-integration-method create` — Create
- `halopsa-pp-cli custom-integration-method delete` — Delete
- `halopsa-pp-cli custom-integration-method get` — Use this to return a single instance of OutboundIntegrationMethod.<br> 				Requires authentication.
- `halopsa-pp-cli custom-integration-method list` — Use this to return multiple OutboundIntegrationMethod.<br> 				Requires authentication.

**custom-integration-method-value** — Manage custom integration method value

- `halopsa-pp-cli custom-integration-method-value` — List

**custom-integration-repository** — Manage custom integration repository

- `halopsa-pp-cli custom-integration-repository get` — Use this to return a single instance of OutboundIntegration.<br> 				Requires authentication.
- `halopsa-pp-cli custom-integration-repository list` — List

**custom-query** — Manage custom query

- `halopsa-pp-cli custom-query create` — Create
- `halopsa-pp-cli custom-query delete` — Delete
- `halopsa-pp-cli custom-query get` — Get
- `halopsa-pp-cli custom-query list` — List

**custom-table** — Manage custom table

- `halopsa-pp-cli custom-table create` — Create
- `halopsa-pp-cli custom-table delete` — Delete
- `halopsa-pp-cli custom-table get` — Use this to return a single instance of CustomTable.<br> 				Requires authentication.
- `halopsa-pp-cli custom-table list` — Use this to return multiple CustomTable.<br> 				Requires authentication.

**dashboard-links** — Manage dashboard links

- `halopsa-pp-cli dashboard-links create` — Create
- `halopsa-pp-cli dashboard-links delete` — Delete
- `halopsa-pp-cli dashboard-links get` — Use this to return a single instance of DashboardLinks.<br> 				Requires authentication.
- `halopsa-pp-cli dashboard-links list` — Use this to return multiple DashboardLinks.<br> 				Requires authentication.
- `halopsa-pp-cli dashboard-links list-dashboardlinks` — List dashboardlinks

**dashboard-links-repository** — Manage dashboard links repository

- `halopsa-pp-cli dashboard-links-repository get` — Use this to return a single instance of DashboardLinks.<br> 				Requires authentication.
- `halopsa-pp-cli dashboard-links-repository list` — Use this to return multiple DashboardLinks.<br> 				Requires authentication.

**database-lookup** — Manage database lookup

- `halopsa-pp-cli database-lookup create` — Create
- `halopsa-pp-cli database-lookup create-databaselookup` — Create databaselookup
- `halopsa-pp-cli database-lookup delete` — Delete
- `halopsa-pp-cli database-lookup get` — Use this to return a single instance of PartsLookup.<br> 				Requires authentication.
- `halopsa-pp-cli database-lookup list` — Use this to return multiple PartsLookup.<br> 				Requires authentication.

**database-lookup-confirmation** — Manage database lookup confirmation

- `halopsa-pp-cli database-lookup-confirmation create` — Create
- `halopsa-pp-cli database-lookup-confirmation get` — Get

**datto-commerce-details** — Manage datto commerce details

- `halopsa-pp-cli datto-commerce-details create` — Create
- `halopsa-pp-cli datto-commerce-details delete` — Delete
- `halopsa-pp-cli datto-commerce-details get` — Use this to return a single instance of DattoCommerceDetails.<br> 				Requires authentication.
- `halopsa-pp-cli datto-commerce-details list` — Use this to return multiple DattoCommerceDetails.<br> 				Requires authentication.

**datto-rmm-details** — Manage datto rmm details

- `halopsa-pp-cli datto-rmm-details create` — Create
- `halopsa-pp-cli datto-rmm-details delete` — Delete
- `halopsa-pp-cli datto-rmm-details get` — Get
- `halopsa-pp-cli datto-rmm-details list` — List

**device-licence** — Manage device licence

- `halopsa-pp-cli device-licence` — List

**distribution-lists** — Manage distribution lists

- `halopsa-pp-cli distribution-lists create` — Create
- `halopsa-pp-cli distribution-lists delete` — Delete
- `halopsa-pp-cli distribution-lists get` — Get
- `halopsa-pp-cli distribution-lists list` — List

**distribution-lists-log** — Manage distribution lists log

- `halopsa-pp-cli distribution-lists-log create` — Create
- `halopsa-pp-cli distribution-lists-log delete` — Delete
- `halopsa-pp-cli distribution-lists-log get` — Get
- `halopsa-pp-cli distribution-lists-log list` — List

**document-creation** — Manage document creation

- `halopsa-pp-cli document-creation` — Create

**downtime** — Manage downtime

- `halopsa-pp-cli downtime create` — Create
- `halopsa-pp-cli downtime delete` — Delete
- `halopsa-pp-cli downtime get` — Get
- `halopsa-pp-cli downtime list` — List
- `halopsa-pp-cli downtime list-downtimecalendar` — List downtimecalendar

**draft** — Manage draft

- `halopsa-pp-cli draft` — Create

**dynamics365-crmdetails** — Manage dynamics365 crmdetails

- `halopsa-pp-cli dynamics365-crmdetails create` — Create
- `halopsa-pp-cli dynamics365-crmdetails delete` — Delete
- `halopsa-pp-cli dynamics365-crmdetails get` — Get
- `halopsa-pp-cli dynamics365-crmdetails list` — List

**dynatrace-details** — Manage dynatrace details

- `halopsa-pp-cli dynatrace-details create` — Create
- `halopsa-pp-cli dynatrace-details delete` — Delete
- `halopsa-pp-cli dynatrace-details get` — Get
- `halopsa-pp-cli dynatrace-details list` — List

**ecommerce-order** — Manage ecommerce order

- `halopsa-pp-cli ecommerce-order create` — Create
- `halopsa-pp-cli ecommerce-order delete` — Delete
- `halopsa-pp-cli ecommerce-order get` — Get
- `halopsa-pp-cli ecommerce-order list` — List

**email-address-book** — Manage email address book

- `halopsa-pp-cli email-address-book` — Use this to return multiple Users.<br> 				Requires authentication.

**email-rule** — Manage email rule

- `halopsa-pp-cli email-rule create` — Create
- `halopsa-pp-cli email-rule delete` — Delete
- `halopsa-pp-cli email-rule get` — Use this to return a single instance of EmailRule.<br> 				Requires authentication.
- `halopsa-pp-cli email-rule list` — Use this to return multiple EmailRule.<br> 				Requires authentication.

**email-store** — Manage email store

- `halopsa-pp-cli email-store create` — Create
- `halopsa-pp-cli email-store delete` — Delete
- `halopsa-pp-cli email-store get` — Use this to return a single instance of EmailStore.<br> 				Requires authentication.
- `halopsa-pp-cli email-store list` — List

**email-template** — Manage email template

- `halopsa-pp-cli email-template create` — Create
- `halopsa-pp-cli email-template create-emailtemplate` — Create emailtemplate
- `halopsa-pp-cli email-template delete` — Delete
- `halopsa-pp-cli email-template get` — Use this to return a single instance of MessageContent.<br> 				Requires authentication.
- `halopsa-pp-cli email-template list` — Use this to return multiple MessageContent.<br> 				Requires authentication.

**email-template-variable** — Manage email template variable

- `halopsa-pp-cli email-template-variable create` — Create
- `halopsa-pp-cli email-template-variable delete` — Delete
- `halopsa-pp-cli email-template-variable get` — Get
- `halopsa-pp-cli email-template-variable list` — List

**eracent** — Manage eracent

- `halopsa-pp-cli eracent` — List

**eracent-details** — Manage eracent details

- `halopsa-pp-cli eracent-details create` — Create
- `halopsa-pp-cli eracent-details delete` — Delete
- `halopsa-pp-cli eracent-details get` — Get
- `halopsa-pp-cli eracent-details list` — List

**event** — Manage event

- `halopsa-pp-cli event create` — Create
- `halopsa-pp-cli event delete` — Delete
- `halopsa-pp-cli event get` — Get
- `halopsa-pp-cli event list` — List

**event-rule** — Manage event rule

- `halopsa-pp-cli event-rule create` — Create
- `halopsa-pp-cli event-rule delete` — Delete
- `halopsa-pp-cli event-rule get` — Get
- `halopsa-pp-cli event-rule list` — List

**exact-details** — Manage exact details

- `halopsa-pp-cli exact-details create` — Create
- `halopsa-pp-cli exact-details delete` — Delete
- `halopsa-pp-cli exact-details get` — Use this to return a single instance of ExactDetails.<br> 				Requires authentication.
- `halopsa-pp-cli exact-details list` — Use this to return multiple ExactDetails.<br> 				Requires authentication.

**example** — Manage example

- `halopsa-pp-cli example` — List

**expense** — Manage expense

- `halopsa-pp-cli expense create` — Create
- `halopsa-pp-cli expense list` — List

**external-chat-message** — Manage external chat message

- `halopsa-pp-cli external-chat-message create` — Create
- `halopsa-pp-cli external-chat-message delete` — Delete
- `halopsa-pp-cli external-chat-message get` — Get
- `halopsa-pp-cli external-chat-message list` — List

**external-link** — Manage external link

- `halopsa-pp-cli external-link create` — Create
- `halopsa-pp-cli external-link create-externallink` — Create externallink
- `halopsa-pp-cli external-link delete` — Delete
- `halopsa-pp-cli external-link get` — Use this to return a single instance of ExternalLink.<br> 				Requires authentication.
- `halopsa-pp-cli external-link list` — Use this to return multiple ExternalLink.<br> 				Requires authentication.

**facebook-details** — Manage facebook details

- `halopsa-pp-cli facebook-details create` — Create
- `halopsa-pp-cli facebook-details delete` — Delete
- `halopsa-pp-cli facebook-details get` — Use this to return a single instance of FacebookDetails.<br> 				Requires authentication.
- `halopsa-pp-cli facebook-details list` — Use this to return multiple FacebookDetails.<br> 				Requires authentication.

**faqlists** — Manage faqlists

- `halopsa-pp-cli faqlists create` — Create
- `halopsa-pp-cli faqlists delete` — Delete
- `halopsa-pp-cli faqlists get` — Use this to return a single instance of FAQListHead.<br> 				Requires authentication.
- `halopsa-pp-cli faqlists list` — Use this to return multiple FAQListHead.<br> 				Requires authentication.

**fault-view-log** — Manage fault view log

- `halopsa-pp-cli fault-view-log` — List

**faults-forecasting** — Manage faults forecasting

- `halopsa-pp-cli faults-forecasting create` — Create
- `halopsa-pp-cli faults-forecasting get` — Use this to return a single instance of FaultsForecasting.<br> 				Requires authentication.

**features** — Manage features

- `halopsa-pp-cli features create` — Create
- `halopsa-pp-cli features get` — Use this to return a single instance of ModuleSetup.<br> 				Requires authentication.
- `halopsa-pp-cli features list` — Use this to return multiple ModuleSetup.<br> 				Requires authentication.

**feed** — Manage feed

- `halopsa-pp-cli feed` — Use this to return multiple Feed.<br> 				Requires authentication.

**field** — Manage field

- `halopsa-pp-cli field create` — Create
- `halopsa-pp-cli field create-addfieldtoall` — Create addfieldtoall
- `halopsa-pp-cli field delete` — Delete specific Field.<br> 				Requires authentication.
- `halopsa-pp-cli field get` — Use this to return a single instance of Field.<br> 				Requires authentication.
- `halopsa-pp-cli field list` — Use this to return multiple Field.<br> 				Requires authentication.

**field-group** — Manage field group

- `halopsa-pp-cli field-group create` — Create
- `halopsa-pp-cli field-group delete` — Delete
- `halopsa-pp-cli field-group get` — Use this to return a single instance of FieldGroup.<br> 				Requires authentication.
- `halopsa-pp-cli field-group list` — Use this to return multiple FieldGroup.<br> 				Requires authentication.

**field-info** — Manage field info

- `halopsa-pp-cli field-info create` — Create
- `halopsa-pp-cli field-info delete` — Delete
- `halopsa-pp-cli field-info get` — Use this to return a single instance of FieldInfo.<br> 				Requires authentication.
- `halopsa-pp-cli field-info list` — Use this to return multiple FieldInfo.<br> 				Requires authentication.

**forecast-details** — Manage forecast details

- `halopsa-pp-cli forecast-details create` — Create
- `halopsa-pp-cli forecast-details delete` — Delete
- `halopsa-pp-cli forecast-details get` — Get
- `halopsa-pp-cli forecast-details list` — List

**forethought-details** — Manage forethought details

- `halopsa-pp-cli forethought-details create` — Create
- `halopsa-pp-cli forethought-details delete` — Delete
- `halopsa-pp-cli forethought-details get` — Get
- `halopsa-pp-cli forethought-details list` — List

**formattedemail** — Manage formattedemail

- `halopsa-pp-cli formattedemail create` — Create
- `halopsa-pp-cli formattedemail delete` — Delete
- `halopsa-pp-cli formattedemail get` — Use this to return a single instance of formattedemail.<br> 				Requires authentication.
- `halopsa-pp-cli formattedemail list` — List

**fortnox-details** — Manage fortnox details

- `halopsa-pp-cli fortnox-details create` — Create
- `halopsa-pp-cli fortnox-details delete` — Delete
- `halopsa-pp-cli fortnox-details get` — Get
- `halopsa-pp-cli fortnox-details list` — List

**go-to-resolve** — Manage go to resolve

- `halopsa-pp-cli go-to-resolve list` — List
- `halopsa-pp-cli go-to-resolve list-gotoresolve` — List gotoresolve

**google-business-details** — Manage google business details

- `halopsa-pp-cli google-business-details create` — Create
- `halopsa-pp-cli google-business-details delete` — Delete
- `halopsa-pp-cli google-business-details get` — Get
- `halopsa-pp-cli google-business-details list` — List

**gworkspace-details** — Manage gworkspace details

- `halopsa-pp-cli gworkspace-details create` — Create
- `halopsa-pp-cli gworkspace-details delete` — Delete
- `halopsa-pp-cli gworkspace-details get` — Get
- `halopsa-pp-cli gworkspace-details list` — List

**halo-device-info** — Manage halo device info

- `halopsa-pp-cli halo-device-info create` — Create
- `halopsa-pp-cli halo-device-info delete` — Delete
- `halopsa-pp-cli halo-device-info get` — Get

**halo-feedback** — Manage halo feedback

- `halopsa-pp-cli halo-feedback create` — Create
- `halopsa-pp-cli halo-feedback delete` — Delete
- `halopsa-pp-cli halo-feedback get` — Use this to return a single instance of Feedback.<br> 				Requires authentication.
- `halopsa-pp-cli halo-feedback list` — List
- `halopsa-pp-cli halo-feedback list-feedbackmessage` — List feedbackmessage

**halo-field** — Manage halo field

- `halopsa-pp-cli halo-field` — List

**halo-health** — Manage halo health

- `halopsa-pp-cli halo-health list` — List
- `halopsa-pp-cli halo-health list-hashing` — List hashing

**halo-integration** — Manage halo integration

- `halopsa-pp-cli halo-integration create` — Create
- `halopsa-pp-cli halo-integration create-halointegration` — Create halointegration
- `halopsa-pp-cli halo-integration list` — List

**halo-news** — Manage halo news

- `halopsa-pp-cli halo-news create` — Create
- `halopsa-pp-cli halo-news create-halonews` — Create halonews
- `halopsa-pp-cli halo-news delete` — Delete
- `halopsa-pp-cli halo-news get` — Use this to return a single instance of HaloNews.<br> 				Requires authentication.
- `halopsa-pp-cli halo-news list` — List

**halo-search** — Manage halo search

- `halopsa-pp-cli halo-search` — Use this to return multiple Search.<br> 				Requires authentication.

**halo-workflow** — Manage halo workflow

- `halopsa-pp-cli halo-workflow create` — Create
- `halopsa-pp-cli halo-workflow delete` — Delete
- `halopsa-pp-cli halo-workflow get` — Use this to return a single instance of FlowHeader.<br> 				Requires authentication.
- `halopsa-pp-cli halo-workflow list` — Use this to return multiple FlowHeader.<br> 				Requires authentication.

**historical-ticket-volumes** — Manage historical ticket volumes

- `halopsa-pp-cli historical-ticket-volumes create` — Create
- `halopsa-pp-cli historical-ticket-volumes delete` — Delete
- `halopsa-pp-cli historical-ticket-volumes get` — Get
- `halopsa-pp-cli historical-ticket-volumes list` — List

**holiday** — Manage holiday

- `halopsa-pp-cli holiday create` — Create
- `halopsa-pp-cli holiday delete` — Delete
- `halopsa-pp-cli holiday get` — Use this to return a single instance of Holidays.<br> 				Requires authentication.
- `halopsa-pp-cli holiday list` — Use this to return multiple Holidays.<br> 				Requires authentication.

**hopewiser** — Manage hopewiser

- `halopsa-pp-cli hopewiser` — List

**impersonation-request** — Manage impersonation request

- `halopsa-pp-cli impersonation-request` — Create

**import-csv** — Manage import csv

- `halopsa-pp-cli import-csv create` — Create
- `halopsa-pp-cli import-csv delete` — Delete
- `halopsa-pp-cli import-csv get` — Use this to return a single instance of ImportCsv.<br> 				Requires authentication.
- `halopsa-pp-cli import-csv list` — Use this to return multiple ImportCsv.<br> 				Requires authentication.

**incoming-event** — Manage incoming event

- `halopsa-pp-cli incoming-event create` — Create
- `halopsa-pp-cli incoming-event create-incomingevent` — Create incomingevent
- `halopsa-pp-cli incoming-event delete` — Delete
- `halopsa-pp-cli incoming-event get` — Get
- `halopsa-pp-cli incoming-event list` — List

**incoming-webhook** — Manage incoming webhook

- `halopsa-pp-cli incoming-webhook create` — Create
- `halopsa-pp-cli incoming-webhook create-incomingwebhook` — Create incomingwebhook
- `halopsa-pp-cli incoming-webhook delete` — Delete
- `halopsa-pp-cli incoming-webhook get` — Get
- `halopsa-pp-cli incoming-webhook list` — List

**incoming-webhook-attempt** — Manage incoming webhook attempt

- `halopsa-pp-cli incoming-webhook-attempt` — List

**incomingemail** — Manage incomingemail

- `halopsa-pp-cli incomingemail create` — Create
- `halopsa-pp-cli incomingemail create-addtoticket` — Create addtoticket
- `halopsa-pp-cli incomingemail delete` — Delete
- `halopsa-pp-cli incomingemail get` — Use this to return a single instance of IncomingEmail.<br> 				Requires authentication.
- `halopsa-pp-cli incomingemail list` — Use this to return multiple IncomingEmail.<br> 				Requires authentication.

**ingram-micro-details** — Manage ingram micro details

- `halopsa-pp-cli ingram-micro-details create` — Create
- `halopsa-pp-cli ingram-micro-details delete` — Delete
- `halopsa-pp-cli ingram-micro-details get` — Use this to return a single instance of IngramMicroDetails.<br> 				Requires authentication.
- `halopsa-pp-cli ingram-micro-details list` — List

**ingram-micro-reseller** — Manage ingram micro reseller

- `halopsa-pp-cli ingram-micro-reseller list` — List
- `halopsa-pp-cli ingram-micro-reseller list-ingrammicroreseller` — List ingrammicroreseller

**ingram-micro-reseller-details** — Manage ingram micro reseller details

- `halopsa-pp-cli ingram-micro-reseller-details create` — Create
- `halopsa-pp-cli ingram-micro-reseller-details delete` — Delete
- `halopsa-pp-cli ingram-micro-reseller-details get` — Get
- `halopsa-pp-cli ingram-micro-reseller-details list` — List

**instance** — Manage instance

- `halopsa-pp-cli instance create` — Create
- `halopsa-pp-cli instance get` — Get
- `halopsa-pp-cli instance list` — Use this to return multiple Instance.<br> 				Requires authentication.

**instance-info** — Manage instance info

- `halopsa-pp-cli instance-info` — List

**integration-configuration** — Manage integration configuration

- `halopsa-pp-cli integration-configuration create` — Create
- `halopsa-pp-cli integration-configuration get` — Use this to return a single instance of IntegrationConfiguration.<br> 				Requires authentication.
- `halopsa-pp-cli integration-configuration list` — List

**integration-data** — Manage integration data

- `halopsa-pp-cli integration-data create` — Create
- `halopsa-pp-cli integration-data create-integrationdata` — Create integrationdata
- `halopsa-pp-cli integration-data create-integrationdata-10` — Create integrationdata 10
- `halopsa-pp-cli integration-data create-integrationdata-11` — .<br> 				Requires authentication.
- `halopsa-pp-cli integration-data create-integrationdata-12` — Create integrationdata 12
- `halopsa-pp-cli integration-data create-integrationdata-13` — Create integrationdata 13
- `halopsa-pp-cli integration-data create-integrationdata-14` — Create integrationdata 14
- `halopsa-pp-cli integration-data create-integrationdata-15` — Create integrationdata 15
- `halopsa-pp-cli integration-data create-integrationdata-16` — Create integrationdata 16
- `halopsa-pp-cli integration-data create-integrationdata-17` — Create integrationdata 17
- `halopsa-pp-cli integration-data create-integrationdata-18` — Create integrationdata 18
- `halopsa-pp-cli integration-data create-integrationdata-19` — Create integrationdata 19
- `halopsa-pp-cli integration-data create-integrationdata-2` — Create integrationdata 2
- `halopsa-pp-cli integration-data create-integrationdata-20` — Create integrationdata 20
- `halopsa-pp-cli integration-data create-integrationdata-21` — Create integrationdata 21
- `halopsa-pp-cli integration-data create-integrationdata-22` — Create integrationdata 22
- `halopsa-pp-cli integration-data create-integrationdata-23` — Create integrationdata 23
- `halopsa-pp-cli integration-data create-integrationdata-24` — Create integrationdata 24
- `halopsa-pp-cli integration-data create-integrationdata-25` — Create integrationdata 25
- `halopsa-pp-cli integration-data create-integrationdata-26` — Create integrationdata 26
- `halopsa-pp-cli integration-data create-integrationdata-27` — Create integrationdata 27
- `halopsa-pp-cli integration-data create-integrationdata-28` — Create integrationdata 28
- `halopsa-pp-cli integration-data create-integrationdata-29` — Create integrationdata 29
- `halopsa-pp-cli integration-data create-integrationdata-3` — Create integrationdata 3
- `halopsa-pp-cli integration-data create-integrationdata-30` — Create integrationdata 30
- `halopsa-pp-cli integration-data create-integrationdata-31` — Create integrationdata 31
- `halopsa-pp-cli integration-data create-integrationdata-32` — Create integrationdata 32
- `halopsa-pp-cli integration-data create-integrationdata-33` — Create integrationdata 33
- `halopsa-pp-cli integration-data create-integrationdata-34` — Create integrationdata 34
- `halopsa-pp-cli integration-data create-integrationdata-35` — Create integrationdata 35
- `halopsa-pp-cli integration-data create-integrationdata-36` — Create integrationdata 36
- `halopsa-pp-cli integration-data create-integrationdata-37` — Create integrationdata 37
- `halopsa-pp-cli integration-data create-integrationdata-38` — Create integrationdata 38
- `halopsa-pp-cli integration-data create-integrationdata-39` — Create integrationdata 39
- `halopsa-pp-cli integration-data create-integrationdata-4` — Create integrationdata 4
- `halopsa-pp-cli integration-data create-integrationdata-40` — Create integrationdata 40
- `halopsa-pp-cli integration-data create-integrationdata-41` — Create integrationdata 41
- `halopsa-pp-cli integration-data create-integrationdata-42` — Create integrationdata 42
- `halopsa-pp-cli integration-data create-integrationdata-5` — Create integrationdata 5
- `halopsa-pp-cli integration-data create-integrationdata-6` — Create integrationdata 6
- `halopsa-pp-cli integration-data create-integrationdata-7` — Create integrationdata 7
- `halopsa-pp-cli integration-data create-integrationdata-8` — Create integrationdata 8
- `halopsa-pp-cli integration-data create-integrationdata-9` — Create integrationdata 9
- `halopsa-pp-cli integration-data get` — Get
- `halopsa-pp-cli integration-data list` — List
- `halopsa-pp-cli integration-data list-integrationdata` — List integrationdata
- `halopsa-pp-cli integration-data list-integrationdata-10` — .<br> 				Requires authentication.
- `halopsa-pp-cli integration-data list-integrationdata-100` — List integrationdata 100
- `halopsa-pp-cli integration-data list-integrationdata-101` — List integrationdata 101
- `halopsa-pp-cli integration-data list-integrationdata-102` — List integrationdata 102
- `halopsa-pp-cli integration-data list-integrationdata-103` — .<br> 				Requires authentication.
- `halopsa-pp-cli integration-data list-integrationdata-104` — .<br> 				Requires authentication.
- `halopsa-pp-cli integration-data list-integrationdata-105` — List integrationdata 105
- `halopsa-pp-cli integration-data list-integrationdata-106` — List integrationdata 106
- `halopsa-pp-cli integration-data list-integrationdata-107` — List integrationdata 107
- `halopsa-pp-cli integration-data list-integrationdata-108` — List integrationdata 108
- `halopsa-pp-cli integration-data list-integrationdata-109` — List integrationdata 109
- `halopsa-pp-cli integration-data list-integrationdata-11` — List integrationdata 11
- `halopsa-pp-cli integration-data list-integrationdata-110` — List integrationdata 110
- `halopsa-pp-cli integration-data list-integrationdata-111` — List integrationdata 111
- `halopsa-pp-cli integration-data list-integrationdata-112` — List integrationdata 112
- `halopsa-pp-cli integration-data list-integrationdata-12` — .<br> 				Requires authentication.
- `halopsa-pp-cli integration-data list-integrationdata-13` — List integrationdata 13
- `halopsa-pp-cli integration-data list-integrationdata-14` — .<br> 				Requires authentication.
- `halopsa-pp-cli integration-data list-integrationdata-15` — .<br> 				Requires authentication.
- `halopsa-pp-cli integration-data list-integrationdata-16` — List integrationdata 16
- `halopsa-pp-cli integration-data list-integrationdata-17` — List integrationdata 17
- `halopsa-pp-cli integration-data list-integrationdata-18` — .<br> 				Requires authentication.
- `halopsa-pp-cli integration-data list-integrationdata-19` — .<br> 				Requires authentication.
- `halopsa-pp-cli integration-data list-integrationdata-2` — .<br> 				Requires authentication.
- `halopsa-pp-cli integration-data list-integrationdata-20` — List integrationdata 20
- `halopsa-pp-cli integration-data list-integrationdata-21` — .<br> 				Requires authentication.
- `halopsa-pp-cli integration-data list-integrationdata-22` — .<br> 				Requires authentication.
- `halopsa-pp-cli integration-data list-integrationdata-23` — .<br> 				Requires authentication.
- `halopsa-pp-cli integration-data list-integrationdata-24` — .<br> 				Requires authentication.
- `halopsa-pp-cli integration-data list-integrationdata-25` — .<br> 				Requires authentication.
- `halopsa-pp-cli integration-data list-integrationdata-26` — List integrationdata 26
- `halopsa-pp-cli integration-data list-integrationdata-27` — .<br> 				Requires authentication.
- `halopsa-pp-cli integration-data list-integrationdata-28` — List integrationdata 28
- `halopsa-pp-cli integration-data list-integrationdata-29` — .<br> 				Requires authentication.
- `halopsa-pp-cli integration-data list-integrationdata-3` — .<br> 				Requires authentication.
- `halopsa-pp-cli integration-data list-integrationdata-30` — .<br> 				Requires authentication.
- `halopsa-pp-cli integration-data list-integrationdata-31` — List integrationdata 31
- `halopsa-pp-cli integration-data list-integrationdata-32` — .<br> 				Requires authentication.
- `halopsa-pp-cli integration-data list-integrationdata-33` — .<br> 				Requires authentication.
- `halopsa-pp-cli integration-data list-integrationdata-34` — List integrationdata 34
- `halopsa-pp-cli integration-data list-integrationdata-35` — .<br> 				Requires authentication.
- `halopsa-pp-cli integration-data list-integrationdata-36` — List integrationdata 36
- `halopsa-pp-cli integration-data list-integrationdata-37` — .<br> 				Requires authentication.
- `halopsa-pp-cli integration-data list-integrationdata-38` — .<br> 				Requires authentication.
- `halopsa-pp-cli integration-data list-integrationdata-39` — .<br> 				Requires authentication.
- `halopsa-pp-cli integration-data list-integrationdata-4` — .<br> 				Requires authentication.
- `halopsa-pp-cli integration-data list-integrationdata-40` — .<br> 				Requires authentication.
- `halopsa-pp-cli integration-data list-integrationdata-41` — .<br> 				Requires authentication.
- `halopsa-pp-cli integration-data list-integrationdata-42` — .<br> 				Requires authentication.
- `halopsa-pp-cli integration-data list-integrationdata-43` — List integrationdata 43
- `halopsa-pp-cli integration-data list-integrationdata-44` — .<br> 				Requires authentication.
- `halopsa-pp-cli integration-data list-integrationdata-45` — List integrationdata 45
- `halopsa-pp-cli integration-data list-integrationdata-46` — .<br> 				Requires authentication.
- `halopsa-pp-cli integration-data list-integrationdata-47` — .<br> 				Requires authentication.
- `halopsa-pp-cli integration-data list-integrationdata-48` — .<br> 				Requires authentication.
- `halopsa-pp-cli integration-data list-integrationdata-49` — .<br> 				Requires authentication.
- `halopsa-pp-cli integration-data list-integrationdata-5` — .<br> 				Requires authentication.
- `halopsa-pp-cli integration-data list-integrationdata-50` — .<br> 				Requires authentication.
- `halopsa-pp-cli integration-data list-integrationdata-51` — .<br> 				Requires authentication.
- `halopsa-pp-cli integration-data list-integrationdata-52` — .<br> 				Requires authentication.
- `halopsa-pp-cli integration-data list-integrationdata-53` — .<br> 				Requires authentication.
- `halopsa-pp-cli integration-data list-integrationdata-54` — .<br> 				Requires authentication.
- `halopsa-pp-cli integration-data list-integrationdata-55` — .<br> 				Requires authentication.
- `halopsa-pp-cli integration-data list-integrationdata-56` — .<br> 				Requires authentication.
- `halopsa-pp-cli integration-data list-integrationdata-57` — List integrationdata 57
- `halopsa-pp-cli integration-data list-integrationdata-58` — List integrationdata 58
- `halopsa-pp-cli integration-data list-integrationdata-59` — List integrationdata 59
- `halopsa-pp-cli integration-data list-integrationdata-6` — .<br> 				Requires authentication.
- `halopsa-pp-cli integration-data list-integrationdata-60` — .<br> 				Requires authentication.
- `halopsa-pp-cli integration-data list-integrationdata-61` — List integrationdata 61
- `halopsa-pp-cli integration-data list-integrationdata-62` — List integrationdata 62
- `halopsa-pp-cli integration-data list-integrationdata-63` — .<br> 				Requires authentication.
- `halopsa-pp-cli integration-data list-integrationdata-64` — .<br> 				Requires authentication.
- `halopsa-pp-cli integration-data list-integrationdata-65` — .<br> 				Requires authentication.
- `halopsa-pp-cli integration-data list-integrationdata-66` — .<br> 				Requires authentication.
- `halopsa-pp-cli integration-data list-integrationdata-67` — .<br> 				Requires authentication.
- `halopsa-pp-cli integration-data list-integrationdata-68` — .<br> 				Requires authentication.
- `halopsa-pp-cli integration-data list-integrationdata-69` — .<br> 				Requires authentication.
- `halopsa-pp-cli integration-data list-integrationdata-7` — List integrationdata 7
- `halopsa-pp-cli integration-data list-integrationdata-70` — .<br> 				Requires authentication.
- `halopsa-pp-cli integration-data list-integrationdata-71` — List integrationdata 71
- `halopsa-pp-cli integration-data list-integrationdata-72` — .<br> 				Requires authentication.
- `halopsa-pp-cli integration-data list-integrationdata-73` — .<br> 				Requires authentication.
- `halopsa-pp-cli integration-data list-integrationdata-74` — .<br> 				Requires authentication.
- `halopsa-pp-cli integration-data list-integrationdata-75` — .<br> 				Requires authentication.
- `halopsa-pp-cli integration-data list-integrationdata-76` — List integrationdata 76
- `halopsa-pp-cli integration-data list-integrationdata-77` — .<br> 				Requires authentication.
- `halopsa-pp-cli integration-data list-integrationdata-78` — .<br> 				Requires authentication.
- `halopsa-pp-cli integration-data list-integrationdata-79` — List integrationdata 79
- `halopsa-pp-cli integration-data list-integrationdata-8` — .<br> 				Requires authentication.
- `halopsa-pp-cli integration-data list-integrationdata-80` — .<br> 				Requires authentication.
- `halopsa-pp-cli integration-data list-integrationdata-81` — .<br> 				Requires authentication.
- `halopsa-pp-cli integration-data list-integrationdata-82` — List integrationdata 82
- `halopsa-pp-cli integration-data list-integrationdata-83` — List integrationdata 83
- `halopsa-pp-cli integration-data list-integrationdata-84` — List integrationdata 84
- `halopsa-pp-cli integration-data list-integrationdata-85` — .<br> 				Requires authentication.
- `halopsa-pp-cli integration-data list-integrationdata-86` — List integrationdata 86
- `halopsa-pp-cli integration-data list-integrationdata-87` — .<br> 				Requires authentication.
- `halopsa-pp-cli integration-data list-integrationdata-88` — .<br> 				Requires authentication.
- `halopsa-pp-cli integration-data list-integrationdata-89` — List integrationdata 89
- `halopsa-pp-cli integration-data list-integrationdata-9` — .<br> 				Requires authentication.
- `halopsa-pp-cli integration-data list-integrationdata-90` — .<br> 				Requires authentication.
- `halopsa-pp-cli integration-data list-integrationdata-91` — List integrationdata 91
- `halopsa-pp-cli integration-data list-integrationdata-92` — List integrationdata 92
- `halopsa-pp-cli integration-data list-integrationdata-93` — .<br> 				Requires authentication.
- `halopsa-pp-cli integration-data list-integrationdata-94` — List integrationdata 94
- `halopsa-pp-cli integration-data list-integrationdata-95` — List integrationdata 95
- `halopsa-pp-cli integration-data list-integrationdata-96` — .<br> 				Requires authentication.
- `halopsa-pp-cli integration-data list-integrationdata-97` — .<br> 				Requires authentication.
- `halopsa-pp-cli integration-data list-integrationdata-98` — List integrationdata 98
- `halopsa-pp-cli integration-data list-integrationdata-99` — List integrationdata 99

**integration-delta** — Manage integration delta

- `halopsa-pp-cli integration-delta create` — Create
- `halopsa-pp-cli integration-delta delete` — Delete
- `halopsa-pp-cli integration-delta get` — Get
- `halopsa-pp-cli integration-delta list` — List

**integration-error** — Manage integration error

- `halopsa-pp-cli integration-error create` — Create
- `halopsa-pp-cli integration-error delete` — Delete
- `halopsa-pp-cli integration-error get` — Use this to return a single instance of IntegrationError.<br> 				Requires authentication.
- `halopsa-pp-cli integration-error list` — Use this to return multiple IntegrationError.<br> 				Requires authentication.

**integration-export** — Manage integration export

- `halopsa-pp-cli integration-export create` — Create
- `halopsa-pp-cli integration-export delete` — Delete
- `halopsa-pp-cli integration-export list` — Use this to return multiple IntegrationExport.<br> 				Requires authentication.

**integration-field-data** — Manage integration field data

- `halopsa-pp-cli integration-field-data create` — Create
- `halopsa-pp-cli integration-field-data delete` — Delete
- `halopsa-pp-cli integration-field-data get` — Get
- `halopsa-pp-cli integration-field-data list` — List

**integration-field-mapping** — Manage integration field mapping

- `halopsa-pp-cli integration-field-mapping` — Use this to return multiple IntegrationFieldMapping.<br> 				Requires authentication.

**integration-look-up** — Manage integration look up

- `halopsa-pp-cli integration-look-up create` — Create
- `halopsa-pp-cli integration-look-up list` — List

**integration-request** — Manage integration request

- `halopsa-pp-cli integration-request create` — Create
- `halopsa-pp-cli integration-request delete` — Delete
- `halopsa-pp-cli integration-request get` — Use this to return a single instance of IntegrationRequest.<br> 				Requires authentication.
- `halopsa-pp-cli integration-request list` — Use this to return multiple IntegrationRequest.<br> 				Requires authentication.

**integration-runbook-variable-group** — Manage integration runbook variable group

- `halopsa-pp-cli integration-runbook-variable-group get` — Use this to return a single instance of IntegrationRunbookVariableGroup.<br> 				Requires authentication.
- `halopsa-pp-cli integration-runbook-variable-group list` — Use this to return multiple IntegrationRunbookVariableGroup.<br> 				Requires authentication.

**integration-site-mapping** — Manage integration site mapping

- `halopsa-pp-cli integration-site-mapping` — Use this to return multiple IntegrationSiteMapping.<br> 				Requires authentication.

**integrator-log** — Manage integrator log

- `halopsa-pp-cli integrator-log` — Use this to return multiple IntegratorLog.<br> 				Requires authentication.

**integrator-schedule** — Manage integrator schedule

- `halopsa-pp-cli integrator-schedule` — Use this to return multiple IntegratorSchedule.<br> 				Requires authentication.

**integrator-trace** — Manage integrator trace

- `halopsa-pp-cli integrator-trace get` — Get
- `halopsa-pp-cli integrator-trace list` — List

**invoice** — Manage invoice

- `halopsa-pp-cli invoice create` — Create
- `halopsa-pp-cli invoice create-pdf` — Create pdf
- `halopsa-pp-cli invoice create-updatelines` — Create updatelines
- `halopsa-pp-cli invoice create-view` — Create view
- `halopsa-pp-cli invoice delete` — Delete specific InvoiceHeader.<br> 				Requires authentication.
- `halopsa-pp-cli invoice get` — Use this to return a single instance of InvoiceHeader.<br> 				Requires authentication.
- `halopsa-pp-cli invoice list` — Use this to return multiple InvoiceHeader.<br> 				Requires authentication.
- `halopsa-pp-cli invoice list-lines` — List lines

**invoice-change** — Manage invoice change

- `halopsa-pp-cli invoice-change create` — Create
- `halopsa-pp-cli invoice-change list` — Use this to return multiple InvoiceChange.<br> 				Requires authentication.

**invoice-detail-pro-rata** — Manage invoice detail pro rata

- `halopsa-pp-cli invoice-detail-pro-rata` — List

**invoice-payment** — Manage invoice payment

- `halopsa-pp-cli invoice-payment create` — Create
- `halopsa-pp-cli invoice-payment delete` — Delete
- `halopsa-pp-cli invoice-payment get` — Use this to return a single instance of InvoicePayment.<br> 				Requires authentication.
- `halopsa-pp-cli invoice-payment list` — Use this to return multiple InvoicePayment.<br> 				Requires authentication.

**islonline** — Manage islonline

- `halopsa-pp-cli islonline create` — Create
- `halopsa-pp-cli islonline list` — List

**item** — Manage item

- `halopsa-pp-cli item create` — Create
- `halopsa-pp-cli item create-newaccountsid` — Create newaccountsid
- `halopsa-pp-cli item delete` — Delete
- `halopsa-pp-cli item get` — Use this to return a single instance of Item.<br> 				Requires authentication.
- `halopsa-pp-cli item list` — Use this to return multiple Item.<br> 				Requires authentication.

**item-accounts-link** — Manage item accounts link

- `halopsa-pp-cli item-accounts-link create` — Create
- `halopsa-pp-cli item-accounts-link create-itemaccountslink` — Create itemaccountslink
- `halopsa-pp-cli item-accounts-link delete` — Delete
- `halopsa-pp-cli item-accounts-link get` — Get
- `halopsa-pp-cli item-accounts-link list` — List

**item-group** — Manage item group

- `halopsa-pp-cli item-group create` — Create
- `halopsa-pp-cli item-group delete` — Delete
- `halopsa-pp-cli item-group get` — Use this to return a single instance of ItemGroup.<br> 				Requires authentication.
- `halopsa-pp-cli item-group list` — List

**item-stock** — Manage item stock

- `halopsa-pp-cli item-stock create` — Create
- `halopsa-pp-cli item-stock delete` — Delete
- `halopsa-pp-cli item-stock get` — Use this to return a single instance of ItemStock.<br> 				Requires authentication.
- `halopsa-pp-cli item-stock list` — Use this to return multiple ItemStock.<br> 				Requires authentication.

**item-stock-history** — Manage item stock history

- `halopsa-pp-cli item-stock-history get` — Get
- `halopsa-pp-cli item-stock-history list` — Use this to return multiple ItemStockHistory.<br> 				Requires authentication.

**itemsupplier** — Manage itemsupplier

- `halopsa-pp-cli itemsupplier create` — Create
- `halopsa-pp-cli itemsupplier delete` — Delete
- `halopsa-pp-cli itemsupplier get` — Use this to return a single instance of ItemSupplier.<br> 				Requires authentication.
- `halopsa-pp-cli itemsupplier list` — List

**jamf-details** — Manage jamf details

- `halopsa-pp-cli jamf-details create` — Create
- `halopsa-pp-cli jamf-details delete` — Delete
- `halopsa-pp-cli jamf-details get` — Get
- `halopsa-pp-cli jamf-details list` — List

**jira-details** — Manage jira details

- `halopsa-pp-cli jira-details create` — Create
- `halopsa-pp-cli jira-details delete` — Delete
- `halopsa-pp-cli jira-details get` — Get
- `halopsa-pp-cli jira-details list` — List

**journey** — Manage journey

- `halopsa-pp-cli journey create` — Create
- `halopsa-pp-cli journey delete` — Delete
- `halopsa-pp-cli journey get` — Use this to return a single instance of Journey.<br> 				Requires authentication.
- `halopsa-pp-cli journey list` — List

**kandji** — Manage kandji

- `halopsa-pp-cli kandji` — List

**kandji-details** — Manage kandji details

- `halopsa-pp-cli kandji-details create` — Create
- `halopsa-pp-cli kandji-details delete` — Delete
- `halopsa-pp-cli kandji-details get` — Get
- `halopsa-pp-cli kandji-details list` — List

**kaseya-vsax** — Manage kaseya vsax

- `halopsa-pp-cli kaseya-vsax create` — Create
- `halopsa-pp-cli kaseya-vsax delete` — Delete
- `halopsa-pp-cli kaseya-vsax list` — List

**kaseya-vsaxdetails** — Manage kaseya vsaxdetails

- `halopsa-pp-cli kaseya-vsaxdetails create` — Create
- `halopsa-pp-cli kaseya-vsaxdetails delete` — Delete
- `halopsa-pp-cli kaseya-vsaxdetails get` — Get
- `halopsa-pp-cli kaseya-vsaxdetails list` — List

**kashflow-details** — Manage kashflow details

- `halopsa-pp-cli kashflow-details create` — Create
- `halopsa-pp-cli kashflow-details delete` — Delete
- `halopsa-pp-cli kashflow-details get` — Use this to return a single instance of KashflowDetails.<br> 				Requires authentication.
- `halopsa-pp-cli kashflow-details list` — Use this to return multiple KashflowDetails.<br> 				Requires authentication.

**kbarticle** — Manage kbarticle

- `halopsa-pp-cli kbarticle create` — Create
- `halopsa-pp-cli kbarticle create-vote` — Create vote
- `halopsa-pp-cli kbarticle delete` — Delete
- `halopsa-pp-cli kbarticle get` — Use this to return a single instance of KBEntry.<br> 				Requires authentication.
- `halopsa-pp-cli kbarticle list` — Use this to return multiple KBEntry.<br> 				Requires authentication.

**kbarticle-anon** — Manage kbarticle anon

- `halopsa-pp-cli kbarticle-anon get` — Get
- `halopsa-pp-cli kbarticle-anon list` — List

**key-vault** — Manage key vault

- `halopsa-pp-cli key-vault create` — Create
- `halopsa-pp-cli key-vault delete` — Delete
- `halopsa-pp-cli key-vault get` — Get
- `halopsa-pp-cli key-vault list` — List

**languages** — Manage languages

- `halopsa-pp-cli languages create` — Create
- `halopsa-pp-cli languages delete` — Delete
- `halopsa-pp-cli languages get` — Use this to return a single instance of LanguagePack.<br> 				Requires authentication.
- `halopsa-pp-cli languages list` — Use this to return multiple LanguagePack.<br> 				Requires authentication.

**lap-safe** — Manage lap safe

- `halopsa-pp-cli lap-safe list` — List
- `halopsa-pp-cli lap-safe list-lapsafe` — List lapsafe
- `halopsa-pp-cli lap-safe list-lapsafe-2` — List lapsafe 2

**ldapconnection** — Manage ldapconnection

- `halopsa-pp-cli ldapconnection create` — Create
- `halopsa-pp-cli ldapconnection delete` — Delete
- `halopsa-pp-cli ldapconnection get` — Use this to return a single instance of LDAPConnection.<br> 				Requires authentication.
- `halopsa-pp-cli ldapconnection list` — Use this to return multiple LDAPConnection.<br> 				Requires authentication.

**licence-change** — Manage licence change

- `halopsa-pp-cli licence-change` — Use this to return multiple LicenceChange.<br> 				Requires authentication.

**license-info** — Manage license info

- `halopsa-pp-cli license-info create` — Create
- `halopsa-pp-cli license-info list` — Use this to return multiple LicenceInfo.<br> 				Requires authentication.
- `halopsa-pp-cli license-info list-licenseinfo` — List licenseinfo

**login-token** — Manage login token

- `halopsa-pp-cli login-token` — Create

**lookup** — Manage lookup

- `halopsa-pp-cli lookup create` — Create
- `halopsa-pp-cli lookup create-clearcache` — Create clearcache
- `halopsa-pp-cli lookup delete` — Delete
- `halopsa-pp-cli lookup get` — Use this to return a single instance of Lookup.<br> 				Requires authentication.
- `halopsa-pp-cli lookup list` — Use this to return multiple Lookup.<br> 				Requires authentication.

**mail** — Manage mail

- `halopsa-pp-cli mail create` — Create
- `halopsa-pp-cli mail create-integrator` — Create integrator
- `halopsa-pp-cli mail create-integrator-2` — Create integrator 2
- `halopsa-pp-cli mail create-integrator-3` — Create integrator 3
- `halopsa-pp-cli mail create-integrator-4` — Create integrator 4
- `halopsa-pp-cli mail create-processmail` — Create processmail

**mail-campaign** — Manage mail campaign

- `halopsa-pp-cli mail-campaign create` — Create
- `halopsa-pp-cli mail-campaign delete` — Delete
- `halopsa-pp-cli mail-campaign get` — Get
- `halopsa-pp-cli mail-campaign list` — List

**mail-campaign-email** — Manage mail campaign email

- `halopsa-pp-cli mail-campaign-email create` — Create
- `halopsa-pp-cli mail-campaign-email delete` — Delete
- `halopsa-pp-cli mail-campaign-email get` — Get
- `halopsa-pp-cli mail-campaign-email list` — List

**mail-campaign-log** — Manage mail campaign log

- `halopsa-pp-cli mail-campaign-log get` — Get
- `halopsa-pp-cli mail-campaign-log list` — List

**mailbox** — Manage mailbox

- `halopsa-pp-cli mailbox create` — Create
- `halopsa-pp-cli mailbox delete` — Delete
- `halopsa-pp-cli mailbox get` — Use this to return a single instance of Mailbox.<br> 				Requires authentication.
- `halopsa-pp-cli mailbox list` — Use this to return multiple Mailbox.<br> 				Requires authentication.

**mailbox-credential** — Manage mailbox credential

- `halopsa-pp-cli mailbox-credential create` — Create
- `halopsa-pp-cli mailbox-credential delete` — Delete
- `halopsa-pp-cli mailbox-credential get` — Get
- `halopsa-pp-cli mailbox-credential list` — List

**mailchimp** — Manage mailchimp

- `halopsa-pp-cli mailchimp` — List

**manage-engine** — Manage manage engine

- `halopsa-pp-cli manage-engine` — List

**manage-engine-details** — Manage manage engine details

- `halopsa-pp-cli manage-engine-details create` — Create
- `halopsa-pp-cli manage-engine-details delete` — Delete
- `halopsa-pp-cli manage-engine-details get` — Get
- `halopsa-pp-cli manage-engine-details list` — List

**marketing-unsubscribe** — Manage marketing unsubscribe

- `halopsa-pp-cli marketing-unsubscribe create` — Create
- `halopsa-pp-cli marketing-unsubscribe delete` — Delete
- `halopsa-pp-cli marketing-unsubscribe get` — Get
- `halopsa-pp-cli marketing-unsubscribe list` — List

**mattermost-channel-details** — Manage mattermost channel details

- `halopsa-pp-cli mattermost-channel-details` — List

**mattermost-details** — Manage mattermost details

- `halopsa-pp-cli mattermost-details create` — Create
- `halopsa-pp-cli mattermost-details delete` — Delete
- `halopsa-pp-cli mattermost-details get` — Get
- `halopsa-pp-cli mattermost-details list` — List

**mcp** — Manage mcp

- `halopsa-pp-cli mcp create` — Create
- `halopsa-pp-cli mcp delete` — Delete
- `halopsa-pp-cli mcp list` — List

**meter-reading** — Manage meter reading

- `halopsa-pp-cli meter-reading create` — Create
- `halopsa-pp-cli meter-reading get` — Use this to return a single instance of DeviceMeterReading.<br> 				Requires authentication.
- `halopsa-pp-cli meter-reading list` — Use this to return multiple DeviceMeterReading.<br> 				Requires authentication.

**microsoft-subscription-mapping** — Manage microsoft subscription mapping

- `halopsa-pp-cli microsoft-subscription-mapping create` — Create
- `halopsa-pp-cli microsoft-subscription-mapping delete` — Delete
- `halopsa-pp-cli microsoft-subscription-mapping get` — Get
- `halopsa-pp-cli microsoft-subscription-mapping list` — List

**microsoft-teams** — Manage microsoft teams

- `halopsa-pp-cli microsoft-teams` — List

**microsoft-teams-mapping** — Manage microsoft teams mapping

- `halopsa-pp-cli microsoft-teams-mapping create` — Create
- `halopsa-pp-cli microsoft-teams-mapping delete` — Delete
- `halopsa-pp-cli microsoft-teams-mapping get` — Get
- `halopsa-pp-cli microsoft-teams-mapping list` — List

**mo** — Manage mo

- `halopsa-pp-cli mo create` — Create
- `halopsa-pp-cli mo delete` — Delete
- `halopsa-pp-cli mo get` — Get
- `halopsa-pp-cli mo list` — List
- `halopsa-pp-cli mo list-b` — List b
- `halopsa-pp-cli mo list-r` — List r

**myobdetails** — Manage myobdetails

- `halopsa-pp-cli myobdetails create` — Create
- `halopsa-pp-cli myobdetails delete` — Delete
- `halopsa-pp-cli myobdetails get` — Get
- `halopsa-pp-cli myobdetails list` — List

**ncentral-details** — Manage ncentral details

- `halopsa-pp-cli ncentral-details create` — Create
- `halopsa-pp-cli ncentral-details delete` — Delete
- `halopsa-pp-cli ncentral-details get` — Use this to return a single instance of NCentralDetails.<br> 				Requires authentication.
- `halopsa-pp-cli ncentral-details list` — Use this to return multiple NCentralDetails.<br> 				Requires authentication.

**nhserverconfig** — Manage nhserverconfig

- `halopsa-pp-cli nhserverconfig create` — Create
- `halopsa-pp-cli nhserverconfig delete` — Delete
- `halopsa-pp-cli nhserverconfig get` — Use this to return a single instance of NHServerConfig.<br> 				Requires authentication.
- `halopsa-pp-cli nhserverconfig list` — List

**notification** — Manage notification

- `halopsa-pp-cli notification create` — Create
- `halopsa-pp-cli notification delete` — Delete
- `halopsa-pp-cli notification get` — Use this to return a single instance of UnameNotification.<br> 				Requires authentication.
- `halopsa-pp-cli notification list` — Use this to return multiple UnameNotification.<br> 				Requires authentication.

**notification-log** — Manage notification log

- `halopsa-pp-cli notification-log` — List

**notification-message** — Manage notification message

- `halopsa-pp-cli notification-message create` — Create
- `halopsa-pp-cli notification-message delete` — Delete
- `halopsa-pp-cli notification-message get` — Use this to return a single instance of NotificationContent.<br> 				Requires authentication.
- `halopsa-pp-cli notification-message list` — List

**notifications** — Manage notifications

- `halopsa-pp-cli notifications create` — Create
- `halopsa-pp-cli notifications create-process` — Create process
- `halopsa-pp-cli notifications delete` — Delete
- `halopsa-pp-cli notifications get` — Use this to return a single instance of EscMsg.<br> 				Requires authentication.
- `halopsa-pp-cli notifications list` — Use this to return multiple EscMsg.<br> 				Requires authentication.

**object-mapping-profile** — Manage object mapping profile

- `halopsa-pp-cli object-mapping-profile` — List

**online-status** — Manage online status

- `halopsa-pp-cli online-status create` — Create
- `halopsa-pp-cli online-status list` — List

**opportunities** — Manage opportunities

- `halopsa-pp-cli opportunities create` — Create
- `halopsa-pp-cli opportunities create-view` — Create view
- `halopsa-pp-cli opportunities delete` — Delete specific Faults.<br> 				Requires authentication.
- `halopsa-pp-cli opportunities get` — Use this to return a single instance of Faults.<br> 				Requires authentication.
- `halopsa-pp-cli opportunities list` — Use this to return multiple Faults.<br> 				Requires authentication.

**order-line** — Manage order line

- `halopsa-pp-cli order-line` — List

**organisation** — Manage organisation

- `halopsa-pp-cli organisation create` — Create
- `halopsa-pp-cli organisation delete` — Delete
- `halopsa-pp-cli organisation get` — Use this to return a single instance of Organisation.<br> 				Requires authentication.
- `halopsa-pp-cli organisation list` — List

**outcome** — Manage outcome

- `halopsa-pp-cli outcome create` — Create
- `halopsa-pp-cli outcome delete` — Delete
- `halopsa-pp-cli outcome get` — Use this to return a single instance of TOutcome.<br> 				Requires authentication.
- `halopsa-pp-cli outcome list` — Use this to return multiple TOutcome.<br> 				Requires authentication.

**outgoing** — Manage outgoing

- `halopsa-pp-cli outgoing create` — Create
- `halopsa-pp-cli outgoing delete` — Delete
- `halopsa-pp-cli outgoing get` — Use this to return a single instance of Outgoing.<br> 				Requires authentication.
- `halopsa-pp-cli outgoing list` — Use this to return multiple Outgoing.<br> 				Requires authentication.

**outgoing-attempt** — Manage outgoing attempt

- `halopsa-pp-cli outgoing-attempt get` — Use this to return a single instance of OutgoingAttempt.<br> 				Requires authentication.
- `halopsa-pp-cli outgoing-attempt list` — Use this to return multiple OutgoingAttempt.<br> 				Requires authentication.

**outgoingemail** — Manage outgoingemail

- `halopsa-pp-cli outgoingemail create` — Create
- `halopsa-pp-cli outgoingemail delete` — Delete
- `halopsa-pp-cli outgoingemail list` — Use this to return multiple Outgoingemail.<br> 				Requires authentication.

**pagerdutymapping** — Manage pagerdutymapping

- `halopsa-pp-cli pagerdutymapping` — Use this to return multiple PagerDutyMapping.<br> 				Requires authentication.

**password-field** — Manage password field

- `halopsa-pp-cli password-field create` — Create
- `halopsa-pp-cli password-field get` — Use this to return a single instance of AuditPasswordField.<br> 				Requires authentication.
- `halopsa-pp-cli password-field list` — List

**pax8-details** — Manage pax8 details

- `halopsa-pp-cli pax8-details create` — Create
- `halopsa-pp-cli pax8-details delete` — Delete
- `halopsa-pp-cli pax8-details get` — Get
- `halopsa-pp-cli pax8-details list` — List

**pdf-template** — Manage pdf template

- `halopsa-pp-cli pdf-template create` — Create
- `halopsa-pp-cli pdf-template delete` — Delete
- `halopsa-pp-cli pdf-template get` — Use this to return a single instance of PdfTemplate.<br> 				Requires authentication.
- `halopsa-pp-cli pdf-template list` — Use this to return multiple PdfTemplate.<br> 				Requires authentication.

**pdf-template-repository** — Manage pdf template repository

- `halopsa-pp-cli pdf-template-repository get` — Use this to return a single instance of PdfTemplate.<br> 				Requires authentication.
- `halopsa-pp-cli pdf-template-repository list` — Use this to return multiple PdfTemplate.<br> 				Requires authentication.

**popup-note** — Manage popup note

- `halopsa-pp-cli popup-note create` — Create
- `halopsa-pp-cli popup-note list` — Use this to return multiple AreaPopup.<br> 				Requires authentication.

**power-shell-script** — Manage power shell script

- `halopsa-pp-cli power-shell-script create` — Create
- `halopsa-pp-cli power-shell-script delete` — Delete
- `halopsa-pp-cli power-shell-script get` — Use this to return a single instance of PowerShellScript.<br> 				Requires authentication.
- `halopsa-pp-cli power-shell-script list` — Use this to return multiple PowerShellScript.<br> 				Requires authentication.

**power-shell-script-criteria** — Manage power shell script criteria

- `halopsa-pp-cli power-shell-script-criteria create` — Create
- `halopsa-pp-cli power-shell-script-criteria delete` — Delete
- `halopsa-pp-cli power-shell-script-criteria get` — Use this to return a single instance of PowerShellScriptCriteria.<br> 				Requires authentication.
- `halopsa-pp-cli power-shell-script-criteria list` — Use this to return multiple PowerShellScriptCriteria.<br> 				Requires authentication.

**power-shell-script-processing** — Manage power shell script processing

- `halopsa-pp-cli power-shell-script-processing create` — Create
- `halopsa-pp-cli power-shell-script-processing delete` — Delete
- `halopsa-pp-cli power-shell-script-processing get` — Use this to return a single instance of PowerShellScriptProcessing.<br> 				Requires authentication.
- `halopsa-pp-cli power-shell-script-processing list` — Use this to return multiple PowerShellScriptProcessing.<br> 				Requires authentication.

**priority** — Manage priority

- `halopsa-pp-cli priority create` — Create
- `halopsa-pp-cli priority delete` — Delete
- `halopsa-pp-cli priority get` — Use this to return a single instance of Policy.<br> 				Requires authentication.
- `halopsa-pp-cli priority list` — Use this to return multiple Policy.<br> 				Requires authentication.

**product** — Manage product

- `halopsa-pp-cli product create` — Create
- `halopsa-pp-cli product delete` — Delete
- `halopsa-pp-cli product get` — Use this to return a single instance of ReleaseProduct.<br> 				Requires authentication.
- `halopsa-pp-cli product list` — Use this to return multiple ReleaseProduct.<br> 				Requires authentication.

**product-branch** — Manage product branch

- `halopsa-pp-cli product-branch` — Use this to return multiple ReleaseBranch.<br> 				Requires authentication.

**product-component** — Manage product component

- `halopsa-pp-cli product-component create` — Create
- `halopsa-pp-cli product-component delete` — Delete
- `halopsa-pp-cli product-component get` — Use this to return a single instance of ReleaseComponent.<br> 				Requires authentication.
- `halopsa-pp-cli product-component list` — Use this to return multiple ReleaseComponent.<br> 				Requires authentication.

**project-setup-lines** — Manage project setup lines

- `halopsa-pp-cli project-setup-lines` — Create

**projects** — Manage projects

- `halopsa-pp-cli projects create` — Create
- `halopsa-pp-cli projects create-view` — Create view
- `halopsa-pp-cli projects delete` — Delete specific Faults.<br> 				Requires authentication.
- `halopsa-pp-cli projects get` — Use this to return a single instance of Faults.<br> 				Requires authentication.
- `halopsa-pp-cli projects list` — Use this to return multiple Faults.<br> 				Requires authentication.

**prtgdetails** — Manage prtgdetails

- `halopsa-pp-cli prtgdetails create` — Create
- `halopsa-pp-cli prtgdetails delete` — Delete
- `halopsa-pp-cli prtgdetails get` — Get
- `halopsa-pp-cli prtgdetails list` — List

**publish-profiles** — Manage publish profiles

- `halopsa-pp-cli publish-profiles create` — Create
- `halopsa-pp-cli publish-profiles delete` — Delete
- `halopsa-pp-cli publish-profiles get` — Get
- `halopsa-pp-cli publish-profiles list` — List

**purchase-order** — Manage purchase order

- `halopsa-pp-cli purchase-order create` — Create
- `halopsa-pp-cli purchase-order create-purchaseorder` — Create purchaseorder
- `halopsa-pp-cli purchase-order create-purchaseorder-2` — Create purchaseorder 2
- `halopsa-pp-cli purchase-order delete` — Delete
- `halopsa-pp-cli purchase-order get` — Use this to return a single instance of SupplierOrderHeader.<br> 				Requires authentication.
- `halopsa-pp-cli purchase-order list` — Use this to return multiple SupplierOrderHeader.<br> 				Requires authentication.

**qualification** — Manage qualification

- `halopsa-pp-cli qualification create` — Create
- `halopsa-pp-cli qualification delete` — Delete
- `halopsa-pp-cli qualification get` — Use this to return a single instance of Qualification.<br> 				Requires authentication.
- `halopsa-pp-cli qualification list` — Use this to return multiple Qualification.<br> 				Requires authentication.

**quick-books-details** — Manage quick books details

- `halopsa-pp-cli quick-books-details create` — Create
- `halopsa-pp-cli quick-books-details delete` — Delete
- `halopsa-pp-cli quick-books-details get` — Use this to return a single instance of QuickBooksDetails.<br> 				Requires authentication.
- `halopsa-pp-cli quick-books-details list` — Use this to return multiple QuickBooksDetails.<br> 				Requires authentication.

**quotation** — Manage quotation

- `halopsa-pp-cli quotation create` — Create
- `halopsa-pp-cli quotation create-approval` — Create approval
- `halopsa-pp-cli quotation create-lines` — Create lines
- `halopsa-pp-cli quotation create-view` — Create view
- `halopsa-pp-cli quotation delete` — Delete
- `halopsa-pp-cli quotation get` — Use this to return a single instance of QuotationHeader.<br> 				Requires authentication.
- `halopsa-pp-cli quotation list` — Use this to return multiple QuotationHeader.<br> 				Requires authentication.

**raynet** — Manage raynet

- `halopsa-pp-cli raynet` — List

**raynet-details** — Manage raynet details

- `halopsa-pp-cli raynet-details create` — Create
- `halopsa-pp-cli raynet-details delete` — Delete
- `halopsa-pp-cli raynet-details get` — Get
- `halopsa-pp-cli raynet-details list` — List

**recurring-invoice** — Manage recurring invoice

- `halopsa-pp-cli recurring-invoice create` — Create
- `halopsa-pp-cli recurring-invoice create-recurringinvoice` — Create recurringinvoice
- `halopsa-pp-cli recurring-invoice create-recurringinvoice-2` — Create recurringinvoice 2
- `halopsa-pp-cli recurring-invoice create-recurringinvoice-3` — Create recurringinvoice 3
- `halopsa-pp-cli recurring-invoice delete` — Delete specific InvoiceHeader.<br> 				Requires authentication.
- `halopsa-pp-cli recurring-invoice get` — Use this to return a single instance of InvoiceHeader.<br> 				Requires authentication.
- `halopsa-pp-cli recurring-invoice list` — Use this to return multiple InvoiceHeader.<br> 				Requires authentication.

**recurring-item** — Manage recurring item

- `halopsa-pp-cli recurring-item` — Use this to return multiple AreaItem.<br> 				Requires authentication.

**release** — Manage release

- `halopsa-pp-cli release create` — Create
- `halopsa-pp-cli release delete` — Delete
- `halopsa-pp-cli release get` — Use this to return a single instance of Release.<br> 				Requires authentication.
- `halopsa-pp-cli release list` — .<br> 				Requires authentication.

**release-note-group** — Manage release note group

- `halopsa-pp-cli release-note-group create` — Create
- `halopsa-pp-cli release-note-group delete` — Delete
- `halopsa-pp-cli release-note-group get` — Use this to return a single instance of ReleaseNoteGroup.<br> 				Requires authentication.
- `halopsa-pp-cli release-note-group list` — List

**release-pipeline** — Manage release pipeline

- `halopsa-pp-cli release-pipeline create` — Create
- `halopsa-pp-cli release-pipeline delete` — Delete
- `halopsa-pp-cli release-pipeline get` — Get
- `halopsa-pp-cli release-pipeline list` — List

**release-type** — Manage release type

- `halopsa-pp-cli release-type create` — Create
- `halopsa-pp-cli release-type delete` — Delete
- `halopsa-pp-cli release-type get` — Use this to return a single instance of ReleaseType.<br> 				Requires authentication.
- `halopsa-pp-cli release-type list` — List

**remote-session** — Manage remote session

- `halopsa-pp-cli remote-session create` — Create
- `halopsa-pp-cli remote-session delete` — Delete
- `halopsa-pp-cli remote-session get` — Use this to return a single instance of RemoteSessionData.<br> 				Requires authentication.
- `halopsa-pp-cli remote-session list` — Use this to return multiple RemoteSessionData.<br> 				Requires authentication.

**remote-session-teams** — Manage remote session teams

- `halopsa-pp-cli remote-session-teams` — Use this to return multiple RemoteSessionTeams.<br> 				Requires authentication.

**report** — Manage report

- `halopsa-pp-cli report create` — Create
- `halopsa-pp-cli report create-bookmark` — Create bookmark
- `halopsa-pp-cli report create-createpdf` — Create createpdf
- `halopsa-pp-cli report create-print` — Create print
- `halopsa-pp-cli report delete` — Delete
- `halopsa-pp-cli report get` — Use this to return a single instance of AnalyzerProfile.<br> 				Requires authentication.
- `halopsa-pp-cli report list` — Use this to return multiple AnalyzerProfile.<br> 				Requires authentication.

**report-data** — Manage report data

- `halopsa-pp-cli report-data <publishedid>` — Get

**report-repository** — Manage report repository

- `halopsa-pp-cli report-repository get` — Use this to return a single instance of AnalyzerProfile.<br> 				Requires authentication.
- `halopsa-pp-cli report-repository list` — Use this to return multiple AnalyzerProfile.<br> 				Requires authentication.
- `halopsa-pp-cli report-repository list-reportrepository` — Use this to return multiple Lookup.<br> 				Requires authentication.

**resource-type** — Manage resource type

- `halopsa-pp-cli resource-type get` — Get
- `halopsa-pp-cli resource-type list` — List

**roadmap** — Manage roadmap

- `halopsa-pp-cli roadmap` — .<br> 				Requires authentication.

**roles** — Manage roles

- `halopsa-pp-cli roles create` — Create
- `halopsa-pp-cli roles delete` — Delete
- `halopsa-pp-cli roles get` — Use this to return a single instance of NHD_Roles.<br> 				Requires authentication.
- `halopsa-pp-cli roles list` — Use this to return multiple NHD_Roles.<br> 				Requires authentication.

**sage-business-cloud-details** — Manage sage business cloud details

- `halopsa-pp-cli sage-business-cloud-details create` — Create
- `halopsa-pp-cli sage-business-cloud-details delete` — Delete
- `halopsa-pp-cli sage-business-cloud-details get` — Use this to return a single instance of SageBusinessCloudDetails.<br> 				Requires authentication.
- `halopsa-pp-cli sage-business-cloud-details list` — Use this to return multiple SageBusinessCloudDetails.<br> 				Requires authentication.

**sail-point-details** — Manage sail point details

- `halopsa-pp-cli sail-point-details create` — Create
- `halopsa-pp-cli sail-point-details delete` — Delete
- `halopsa-pp-cli sail-point-details get` — Get
- `halopsa-pp-cli sail-point-details list` — List

**sail-point-role-mapping** — Manage sail point role mapping

- `halopsa-pp-cli sail-point-role-mapping` — List

**sail-point-user-mapping** — Manage sail point user mapping

- `halopsa-pp-cli sail-point-user-mapping` — List

**sales-mailbox** — Manage sales mailbox

- `halopsa-pp-cli sales-mailbox create` — Create
- `halopsa-pp-cli sales-mailbox delete` — Delete
- `halopsa-pp-cli sales-mailbox get` — Use this to return a single instance of SalesMailbox.<br> 				Requires authentication.
- `halopsa-pp-cli sales-mailbox list` — List

**sales-mailbox-detail** — Manage sales mailbox detail

- `halopsa-pp-cli sales-mailbox-detail create` — Create
- `halopsa-pp-cli sales-mailbox-detail list` — List

**sales-order** — Manage sales order

- `halopsa-pp-cli sales-order create` — Create
- `halopsa-pp-cli sales-order create-salesorder` — Create salesorder
- `halopsa-pp-cli sales-order delete` — Delete
- `halopsa-pp-cli sales-order get` — Use this to return a single instance of OrderHead.<br> 				Requires authentication.
- `halopsa-pp-cli sales-order list` — Use this to return multiple OrderHead.<br> 				Requires authentication.

**saved-forecast** — Manage saved forecast

- `halopsa-pp-cli saved-forecast create` — Create
- `halopsa-pp-cli saved-forecast delete` — Delete
- `halopsa-pp-cli saved-forecast get` — Get
- `halopsa-pp-cli saved-forecast list` — List

**schedule** — Manage schedule

- `halopsa-pp-cli schedule create` — Create
- `halopsa-pp-cli schedule get` — Use this to return a single instance of Schedule.<br> 				Requires authentication.
- `halopsa-pp-cli schedule list` — Use this to return multiple Schedule.<br> 				Requires authentication.

**schedule-occurrence** — Manage schedule occurrence

- `halopsa-pp-cli schedule-occurrence create` — Create
- `halopsa-pp-cli schedule-occurrence get` — Get
- `halopsa-pp-cli schedule-occurrence list` — List

**screen-layout** — Manage screen layout

- `halopsa-pp-cli screen-layout create` — Create
- `halopsa-pp-cli screen-layout delete` — Delete
- `halopsa-pp-cli screen-layout get` — Use this to return a single instance of ScreenLayout.<br> 				Requires authentication.
- `halopsa-pp-cli screen-layout list` — Use this to return multiple ScreenLayout.<br> 				Requires authentication.

**secure-secret-link** — Manage secure secret link

- `halopsa-pp-cli secure-secret-link create` — Create
- `halopsa-pp-cli secure-secret-link delete` — Delete
- `halopsa-pp-cli secure-secret-link get` — Get
- `halopsa-pp-cli secure-secret-link list` — List
- `halopsa-pp-cli secure-secret-link list-securesecretlink` — List securesecretlink

**security-check** — Manage security check

- `halopsa-pp-cli security-check list` — List
- `halopsa-pp-cli security-check list-securitycheck` — List securitycheck

**security-question** — Manage security question

- `halopsa-pp-cli security-question create` — Create
- `halopsa-pp-cli security-question delete` — Delete
- `halopsa-pp-cli security-question get` — Use this to return a single instance of SecurityQuestion.<br> 				Requires authentication.
- `halopsa-pp-cli security-question list` — List

**security-question-validate** — Manage security question validate

- `halopsa-pp-cli security-question-validate create` — Create
- `halopsa-pp-cli security-question-validate list` — List

**sentinel-one** — Manage sentinel one

- `halopsa-pp-cli sentinel-one` — List

**sentinel-one-details** — Manage sentinel one details

- `halopsa-pp-cli sentinel-one-details create` — Create
- `halopsa-pp-cli sentinel-one-details delete` — Delete
- `halopsa-pp-cli sentinel-one-details get` — Get
- `halopsa-pp-cli sentinel-one-details list` — List

**service** — Manage service

- `halopsa-pp-cli service create` — Create
- `halopsa-pp-cli service create-unsubscribe` — Create unsubscribe
- `halopsa-pp-cli service delete` — Delete
- `halopsa-pp-cli service get` — Use this to return a single instance of ServSite.<br> 				Requires authentication.
- `halopsa-pp-cli service list` — Use this to return multiple ServSite.<br> 				Requires authentication.

**service-availability** — Manage service availability

- `halopsa-pp-cli service-availability create` — Create
- `halopsa-pp-cli service-availability delete` — Delete
- `halopsa-pp-cli service-availability get` — Get
- `halopsa-pp-cli service-availability list` — List

**service-category** — Manage service category

- `halopsa-pp-cli service-category create` — Create
- `halopsa-pp-cli service-category delete` — Delete
- `halopsa-pp-cli service-category get` — Use this to return a single instance of ServiceCategory.<br> 				Requires authentication.
- `halopsa-pp-cli service-category list` — Use this to return multiple ServiceCategory.<br> 				Requires authentication.

**service-request-details** — Manage service request details

- `halopsa-pp-cli service-request-details get` — Use this to return a single instance of ServiceRequestDetails.<br> 				Requires authentication.
- `halopsa-pp-cli service-request-details list` — Use this to return multiple ServiceRequestDetails.<br> 				Requires authentication.

**service-restriction** — Manage service restriction

- `halopsa-pp-cli service-restriction` — Use this to return multiple ServiceRestriction.<br> 				Requires authentication.

**service-status** — Manage service status

- `halopsa-pp-cli service-status create` — Create
- `halopsa-pp-cli service-status create-servicestatus` — Create servicestatus
- `halopsa-pp-cli service-status delete` — Delete
- `halopsa-pp-cli service-status get` — Use this to return a single instance of ServStatus.<br> 				Requires authentication.
- `halopsa-pp-cli service-status get-servicestatus` — Get servicestatus
- `halopsa-pp-cli service-status list` — Use this to return multiple ServStatus.<br> 				Requires authentication.

**setup-tab** — Manage setup tab

- `halopsa-pp-cli setup-tab create` — Create
- `halopsa-pp-cli setup-tab get` — Use this to return a single instance of SetupTab.<br> 				Requires authentication.
- `halopsa-pp-cli setup-tab list` — List

**setup-tab-group** — Manage setup tab group

- `halopsa-pp-cli setup-tab-group get` — Use this to return a single instance of SetupTabGroup.<br> 				Requires authentication.
- `halopsa-pp-cli setup-tab-group list` — List

**share-point** — Manage share point

- `halopsa-pp-cli share-point` — List

**shopify-details** — Manage shopify details

- `halopsa-pp-cli shopify-details create` — Create
- `halopsa-pp-cli shopify-details delete` — Delete
- `halopsa-pp-cli shopify-details get` — Get
- `halopsa-pp-cli shopify-details list` — List

**single-sign-on-application** — Manage single sign on application

- `halopsa-pp-cli single-sign-on-application create` — Create
- `halopsa-pp-cli single-sign-on-application delete` — Delete
- `halopsa-pp-cli single-sign-on-application get` — Get
- `halopsa-pp-cli single-sign-on-application list` — List

**single-sign-on-attempt** — Manage single sign on attempt

- `halopsa-pp-cli single-sign-on-attempt delete` — Delete
- `halopsa-pp-cli single-sign-on-attempt get` — Get
- `halopsa-pp-cli single-sign-on-attempt list` — List

**site** — Manage site

- `halopsa-pp-cli site create` — Create
- `halopsa-pp-cli site delete` — Delete
- `halopsa-pp-cli site get` — Use this to return a single instance of Site.<br> 				Requires authentication.
- `halopsa-pp-cli site list` — Use this to return multiple Site.<br> 				Requires authentication.
- `halopsa-pp-cli site list-stockbins` — List stockbins

**sla** — Manage sla

- `halopsa-pp-cli sla create` — Create
- `halopsa-pp-cli sla delete` — Delete
- `halopsa-pp-cli sla get` — Use this to return a single instance of SlaHead.<br> 				Requires authentication.
- `halopsa-pp-cli sla list` — Use this to return multiple SlaHead.<br> 				Requires authentication.

**slack** — Manage slack

- `halopsa-pp-cli slack create` — Create
- `halopsa-pp-cli slack create-event` — Create event
- `halopsa-pp-cli slack create-interactivity` — Create interactivity
- `halopsa-pp-cli slack create-manifest` — Create manifest

**slack-chat-app** — Manage slack chat app

- `halopsa-pp-cli slack-chat-app create` — Create
- `halopsa-pp-cli slack-chat-app delete` — Delete
- `halopsa-pp-cli slack-chat-app get` — Get
- `halopsa-pp-cli slack-chat-app list` — List

**slack-details** — Manage slack details

- `halopsa-pp-cli slack-details create` — Create
- `halopsa-pp-cli slack-details create-slackdetails` — Create slackdetails
- `halopsa-pp-cli slack-details delete` — Delete
- `halopsa-pp-cli slack-details get` — Use this to return a single instance of SlackDetails.<br> 				Requires authentication.
- `halopsa-pp-cli slack-details list` — Use this to return multiple SlackDetails.<br> 				Requires authentication.

**snipe-itdetails** — Manage snipe itdetails

- `halopsa-pp-cli snipe-itdetails create` — Create
- `halopsa-pp-cli snipe-itdetails delete` — Delete
- `halopsa-pp-cli snipe-itdetails get` — Get
- `halopsa-pp-cli snipe-itdetails list` — List

**snow-details** — Manage snow details

- `halopsa-pp-cli snow-details create` — Create
- `halopsa-pp-cli snow-details delete` — Delete
- `halopsa-pp-cli snow-details get` — Use this to return a single instance of SnowDetails.<br> 				Requires authentication.
- `halopsa-pp-cli snow-details list` — Use this to return multiple SnowDetails.<br> 				Requires authentication.

**software-licence** — Manage software licence

- `halopsa-pp-cli software-licence create` — Create
- `halopsa-pp-cli software-licence delete` — Delete
- `halopsa-pp-cli software-licence get` — Use this to return a single instance of Licence.<br> 				Requires authentication.
- `halopsa-pp-cli software-licence list` — Use this to return multiple Licence.<br> 				Requires authentication.

**software-licence-role** — Manage software licence role

- `halopsa-pp-cli software-licence-role` — Use this to return multiple LicenceRole.<br> 				Requires authentication.

**sophos** — Manage sophos

- `halopsa-pp-cli sophos` — List

**sophos-details** — Manage sophos details

- `halopsa-pp-cli sophos-details create` — Create
- `halopsa-pp-cli sophos-details delete` — Delete
- `halopsa-pp-cli sophos-details get` — Get
- `halopsa-pp-cli sophos-details list` — List

**sqlimport** — Manage sqlimport

- `halopsa-pp-cli sqlimport create` — Create
- `halopsa-pp-cli sqlimport delete` — Delete
- `halopsa-pp-cli sqlimport get` — Use this to return a single instance of SQLImport.<br> 				Requires authentication.
- `halopsa-pp-cli sqlimport list` — Use this to return multiple SQLImport.<br> 				Requires authentication.

**status** — Manage status

- `halopsa-pp-cli status create` — Create
- `halopsa-pp-cli status delete` — Delete
- `halopsa-pp-cli status get` — Use this to return a single instance of TStatus.<br> 				Requires authentication.
- `halopsa-pp-cli status list` — Use this to return multiple TStatus.<br> 				Requires authentication.

**stock-bin** — Manage stock bin

- `halopsa-pp-cli stock-bin create` — Create
- `halopsa-pp-cli stock-bin delete` — Delete
- `halopsa-pp-cli stock-bin get` — Get
- `halopsa-pp-cli stock-bin list` — List

**stock-trace** — Manage stock trace

- `halopsa-pp-cli stock-trace get` — Get
- `halopsa-pp-cli stock-trace list` — List

**stream-one-ion-details** — Manage stream one ion details

- `halopsa-pp-cli stream-one-ion-details create` — Create
- `halopsa-pp-cli stream-one-ion-details delete` — Delete
- `halopsa-pp-cli stream-one-ion-details get` — Get
- `halopsa-pp-cli stream-one-ion-details list` — List

**style-profile** — Manage style profile

- `halopsa-pp-cli style-profile create` — Create
- `halopsa-pp-cli style-profile delete` — Delete
- `halopsa-pp-cli style-profile get` — Get
- `halopsa-pp-cli style-profile list` — List

**supplier** — Manage supplier

- `halopsa-pp-cli supplier create` — Create
- `halopsa-pp-cli supplier delete` — Delete
- `halopsa-pp-cli supplier get` — Use this to return a single instance of Company.<br> 				Requires authentication.
- `halopsa-pp-cli supplier list` — Use this to return multiple Company.<br> 				Requires authentication.

**supplier-contract** — Manage supplier contract

- `halopsa-pp-cli supplier-contract create` — Create
- `halopsa-pp-cli supplier-contract create-suppliercontract` — Create suppliercontract
- `halopsa-pp-cli supplier-contract delete` — Delete
- `halopsa-pp-cli supplier-contract get` — Use this to return a single instance of Contract.<br> 				Requires authentication.
- `halopsa-pp-cli supplier-contract list` — Use this to return multiple Contract.<br> 				Requires authentication.

**synnex-details** — Manage synnex details

- `halopsa-pp-cli synnex-details create` — Create
- `halopsa-pp-cli synnex-details delete` — Delete
- `halopsa-pp-cli synnex-details get` — Use this to return a single instance of IngramMicroDetails.<br> 				Requires authentication.
- `halopsa-pp-cli synnex-details list` — List

**tabs** — Manage tabs

- `halopsa-pp-cli tabs create` — Create
- `halopsa-pp-cli tabs delete` — Delete
- `halopsa-pp-cli tabs get` — Use this to return a single instance of Tabname.<br> 				Requires authentication.
- `halopsa-pp-cli tabs list` — Use this to return multiple Tabname.<br> 				Requires authentication.

**tags** — Manage tags

- `halopsa-pp-cli tags create` — Create
- `halopsa-pp-cli tags delete` — Delete
- `halopsa-pp-cli tags get` — Use this to return a single instance of Tag.<br> 				Requires authentication.
- `halopsa-pp-cli tags list` — List

**take-control** — Manage take control

- `halopsa-pp-cli take-control` — List

**tanium-details** — Manage tanium details

- `halopsa-pp-cli tanium-details create` — Create
- `halopsa-pp-cli tanium-details delete` — Delete
- `halopsa-pp-cli tanium-details get` — Get
- `halopsa-pp-cli tanium-details list` — List

**task-monitor-event** — Manage task monitor event

- `halopsa-pp-cli task-monitor-event` — List

**task-schedule** — Manage task schedule

- `halopsa-pp-cli task-schedule create` — Create
- `halopsa-pp-cli task-schedule list` — List

**task-trace** — Manage task trace

- `halopsa-pp-cli task-trace get` — Get
- `halopsa-pp-cli task-trace list` — List

**tax** — Manage tax

- `halopsa-pp-cli tax create` — Create
- `halopsa-pp-cli tax delete` — Delete
- `halopsa-pp-cli tax get` — Use this to return a single instance of Tax.<br> 				Requires authentication.
- `halopsa-pp-cli tax list` — Use this to return multiple Tax.<br> 				Requires authentication.

**tax-rule** — Manage tax rule

- `halopsa-pp-cli tax-rule create` — Create
- `halopsa-pp-cli tax-rule delete` — Delete
- `halopsa-pp-cli tax-rule get` — Get
- `halopsa-pp-cli tax-rule list` — List

**team** — Manage team

- `halopsa-pp-cli team create` — Create
- `halopsa-pp-cli team delete` — Delete
- `halopsa-pp-cli team get` — Use this to return a single instance of SectionDetail.<br> 				Requires authentication.
- `halopsa-pp-cli team list` — Use this to return multiple SectionDetail.<br> 				Requires authentication.
- `halopsa-pp-cli team list-tree` — List tree

**team-image** — Manage team image

- `halopsa-pp-cli team-image <id>` — Get

**tech-data-reseller-details** — Manage tech data reseller details

- `halopsa-pp-cli tech-data-reseller-details create` — Create
- `halopsa-pp-cli tech-data-reseller-details delete` — Delete
- `halopsa-pp-cli tech-data-reseller-details get` — Get
- `halopsa-pp-cli tech-data-reseller-details list` — List

**template** — Manage template

- `halopsa-pp-cli template create` — Create
- `halopsa-pp-cli template delete` — Delete
- `halopsa-pp-cli template get` — Use this to return a single instance of StdRequest.<br> 				Requires authentication.
- `halopsa-pp-cli template list` — Use this to return multiple StdRequest.<br> 				Requires authentication.

**tenable** — Manage tenable

- `halopsa-pp-cli tenable create` — Create
- `halopsa-pp-cli tenable create-export` — Create export
- `halopsa-pp-cli tenable list` — List
- `halopsa-pp-cli tenable list-status` — List status

**tenable-details** — Manage tenable details

- `halopsa-pp-cli tenable-details create` — Create
- `halopsa-pp-cli tenable-details delete` — Delete
- `halopsa-pp-cli tenable-details get` — Get
- `halopsa-pp-cli tenable-details list` — List

**tenant** — Manage tenant

- `halopsa-pp-cli tenant create` — Create
- `halopsa-pp-cli tenant list` — List

**test-error** — Manage test error

- `halopsa-pp-cli test-error` — List

**test1** — Manage test1

- `halopsa-pp-cli test1` — List

**test3** — Manage test3

- `halopsa-pp-cli test3` — List

**test4** — Manage test4

- `halopsa-pp-cli test4` — List

**ticket-approval** — Manage ticket approval

- `halopsa-pp-cli ticket-approval create` — Create
- `halopsa-pp-cli ticket-approval delete` — Delete
- `halopsa-pp-cli ticket-approval get` — Use this to return a single instance of FaultApproval.<br> 				Requires authentication.
- `halopsa-pp-cli ticket-approval list` — Use this to return multiple FaultApproval.<br> 				Requires authentication.

**ticket-area** — Manage ticket area

- `halopsa-pp-cli ticket-area create` — Create
- `halopsa-pp-cli ticket-area delete` — Delete
- `halopsa-pp-cli ticket-area get` — Use this to return a single instance of TicketArea.<br> 				Requires authentication.
- `halopsa-pp-cli ticket-area list` — List

**ticket-rules** — Manage ticket rules

- `halopsa-pp-cli ticket-rules create` — Create
- `halopsa-pp-cli ticket-rules delete` — Delete
- `halopsa-pp-cli ticket-rules get` — Use this to return a single instance of Autoassign.<br> 				Requires authentication.
- `halopsa-pp-cli ticket-rules list` — Use this to return multiple Autoassign.<br> 				Requires authentication.

**ticket-type** — Manage ticket type

- `halopsa-pp-cli ticket-type create` — Create
- `halopsa-pp-cli ticket-type delete` — Delete
- `halopsa-pp-cli ticket-type get` — Use this to return a single instance of RequestType.<br> 				Requires authentication.
- `halopsa-pp-cli ticket-type list` — Use this to return multiple RequestType.<br> 				Requires authentication.

**ticket-type-field** — Manage ticket type field

- `halopsa-pp-cli ticket-type-field` — Use this to return multiple RequestTypeField.<br> 				Requires authentication.

**ticket-type-group** — Manage ticket type group

- `halopsa-pp-cli ticket-type-group create` — Create
- `halopsa-pp-cli ticket-type-group delete` — Delete
- `halopsa-pp-cli ticket-type-group get` — Use this to return a single instance of RequestTypeGroup.<br> 				Requires authentication.
- `halopsa-pp-cli ticket-type-group list` — List

**tickets** — Manage tickets

- `halopsa-pp-cli tickets create` — Create
- `halopsa-pp-cli tickets create-object` — Create object
- `halopsa-pp-cli tickets create-processchildren` — Create processchildren
- `halopsa-pp-cli tickets create-setbillableproject` — Create setbillableproject
- `halopsa-pp-cli tickets create-view` — Create view
- `halopsa-pp-cli tickets create-vote` — Create vote
- `halopsa-pp-cli tickets delete` — Delete specific Faults.<br> 				Requires authentication.
- `halopsa-pp-cli tickets get` — Use this to return a single instance of Faults.<br> 				Requires authentication.
- `halopsa-pp-cli tickets list` — Use this to return multiple Faults.<br> 				Requires authentication.
- `halopsa-pp-cli tickets list-salesmailbox` — List salesmailbox
- `halopsa-pp-cli tickets list-zapier` — List zapier

**timesheet** — Manage timesheet

- `halopsa-pp-cli timesheet create` — Create
- `halopsa-pp-cli timesheet get` — Use this to return a single instance of Timesheet.<br> 				Requires authentication.
- `halopsa-pp-cli timesheet list` — List
- `halopsa-pp-cli timesheet list-forecasting` — List forecasting
- `halopsa-pp-cli timesheet list-mine` — List mine

**timesheet-event** — Manage timesheet event

- `halopsa-pp-cli timesheet-event create` — Create
- `halopsa-pp-cli timesheet-event delete` — Delete
- `halopsa-pp-cli timesheet-event get` — Use this to return a single instance of TimesheetEvent.<br> 				Requires authentication.
- `halopsa-pp-cli timesheet-event list` — Use this to return multiple TimesheetEvent.<br> 				Requires authentication.
- `halopsa-pp-cli timesheet-event list-timesheetevent` — List timesheetevent

**timeslot** — Manage timeslot

- `halopsa-pp-cli timeslot` — Use this to return multiple Timeslot.<br> 				Requires authentication.

**to-do** — Manage to do

- `halopsa-pp-cli to-do create` — Create
- `halopsa-pp-cli to-do list` — Use this to return multiple FaultToDo.<br> 				Requires authentication.

**to-do-group** — Manage to do group

- `halopsa-pp-cli to-do-group create` — Create
- `halopsa-pp-cli to-do-group delete` — Delete
- `halopsa-pp-cli to-do-group get` — Get
- `halopsa-pp-cli to-do-group list` — List

**top-level** — Manage top level

- `halopsa-pp-cli top-level create` — Create
- `halopsa-pp-cli top-level delete` — Delete
- `halopsa-pp-cli top-level get` — Use this to return a single instance of Tree.<br> 				Requires authentication.
- `halopsa-pp-cli top-level list` — Use this to return multiple Tree.<br> 				Requires authentication.

**transcription-store** — Manage transcription store

- `halopsa-pp-cli transcription-store create` — Create
- `halopsa-pp-cli transcription-store delete` — Delete
- `halopsa-pp-cli transcription-store get` — Get
- `halopsa-pp-cli transcription-store list` — List

**translation** — Manage translation

- `halopsa-pp-cli translation create` — Create
- `halopsa-pp-cli translation list` — List

**twilio** — Manage twilio

- `halopsa-pp-cli twilio create` — Create
- `halopsa-pp-cli twilio create-twiml` — Create twiml

**twilio-details** — Manage twilio details

- `halopsa-pp-cli twilio-details` — List

**twilio-whats-app-details** — Manage twilio whats app details

- `halopsa-pp-cli twilio-whats-app-details create` — Create
- `halopsa-pp-cli twilio-whats-app-details delete` — Delete
- `halopsa-pp-cli twilio-whats-app-details get` — Get
- `halopsa-pp-cli twilio-whats-app-details list` — List

**twitter-details** — Manage twitter details

- `halopsa-pp-cli twitter-details create` — Create
- `halopsa-pp-cli twitter-details delete` — Delete
- `halopsa-pp-cli twitter-details get` — Use this to return a single instance of TwitterDetails.<br> 				Requires authentication.
- `halopsa-pp-cli twitter-details list` — Use this to return multiple TwitterDetails.<br> 				Requires authentication.

**unsub-service-emails** — Manage unsub service emails

- `halopsa-pp-cli unsub-service-emails create` — Create
- `halopsa-pp-cli unsub-service-emails delete` — Delete
- `halopsa-pp-cli unsub-service-emails get` — Use this to return a single instance of UnsubEmailServiceUsers.<br> 				Requires authentication.
- `halopsa-pp-cli unsub-service-emails list` — List

**user-change** — Manage user change

- `halopsa-pp-cli user-change` — Use this to return multiple UserChange.<br> 				Requires authentication.

**user-roles** — Manage user roles

- `halopsa-pp-cli user-roles create` — Create
- `halopsa-pp-cli user-roles delete` — Delete
- `halopsa-pp-cli user-roles get` — Use this to return a single instance of UserRoles.<br> 				Requires authentication.
- `halopsa-pp-cli user-roles list` — List

**users** — Manage users

- `halopsa-pp-cli users create` — Create
- `halopsa-pp-cli users create-prefs` — Create prefs
- `halopsa-pp-cli users delete` — Delete
- `halopsa-pp-cli users get` — Use this to return a single instance of Users.<br> 				Requires authentication.
- `halopsa-pp-cli users list` — Use this to return multiple Users.<br> 				Requires authentication.
- `halopsa-pp-cli users list-me` — List me

**version-info** — Manage version info

- `halopsa-pp-cli version-info get` — Use this to return a single instance of Release.<br> 				Requires authentication.
- `halopsa-pp-cli version-info get-versioninfo` — Get versioninfo
- `halopsa-pp-cli version-info list` — .<br> 				Requires authentication.
- `halopsa-pp-cli version-info list-versioninfo` — List versioninfo
- `halopsa-pp-cli version-info list-versioninfo-2` — .<br> 				Requires authentication.
- `halopsa-pp-cli version-info list-versioninfo-3` — .<br> 				Requires authentication.

**view-columns** — Manage view columns

- `halopsa-pp-cli view-columns create` — Create
- `halopsa-pp-cli view-columns delete` — Delete
- `halopsa-pp-cli view-columns get` — Use this to return a single instance of ViewColumns.<br> 				Requires authentication.
- `halopsa-pp-cli view-columns list` — Use this to return multiple ViewColumns.<br> 				Requires authentication.

**view-filter** — Manage view filter

- `halopsa-pp-cli view-filter create` — Create
- `halopsa-pp-cli view-filter delete` — Delete
- `halopsa-pp-cli view-filter get` — Use this to return a single instance of ViewFilter.<br> 				Requires authentication.
- `halopsa-pp-cli view-filter list` — Use this to return multiple ViewFilter.<br> 				Requires authentication.

**view-list-group** — Manage view list group

- `halopsa-pp-cli view-list-group create` — Create
- `halopsa-pp-cli view-list-group delete` — Delete
- `halopsa-pp-cli view-list-group get` — Use this to return a single instance of ViewListGroup.<br> 				Requires authentication.
- `halopsa-pp-cli view-list-group list` — Use this to return multiple ViewListGroup.<br> 				Requires authentication.

**view-lists** — Manage view lists

- `halopsa-pp-cli view-lists create` — Create
- `halopsa-pp-cli view-lists delete` — Delete
- `halopsa-pp-cli view-lists get` — Use this to return a single instance of ViewLists.<br> 				Requires authentication.
- `halopsa-pp-cli view-lists list` — Use this to return multiple ViewLists.<br> 				Requires authentication.

**virima** — Manage virima

- `halopsa-pp-cli virima` — List

**virima-details** — Manage virima details

- `halopsa-pp-cli virima-details create` — Create
- `halopsa-pp-cli virima-details delete` — Delete
- `halopsa-pp-cli virima-details get` — Get
- `halopsa-pp-cli virima-details list` — List

**virtual-agent** — Manage virtual agent

- `halopsa-pp-cli virtual-agent create` — Create
- `halopsa-pp-cli virtual-agent delete` — Delete
- `halopsa-pp-cli virtual-agent get` — Get
- `halopsa-pp-cli virtual-agent list` — List

**vmworkspace-details** — Manage vmworkspace details

- `halopsa-pp-cli vmworkspace-details create` — Create
- `halopsa-pp-cli vmworkspace-details delete` — Delete
- `halopsa-pp-cli vmworkspace-details get` — Get
- `halopsa-pp-cli vmworkspace-details list` — List

**vorboss** — Manage vorboss

- `halopsa-pp-cli vorboss` — List

**webhook** — Manage webhook

- `halopsa-pp-cli webhook create` — Create
- `halopsa-pp-cli webhook delete` — Delete
- `halopsa-pp-cli webhook get` — Use this to return a single instance of Webhook.<br> 				Requires authentication.
- `halopsa-pp-cli webhook list` — Use this to return multiple Webhook.<br> 				Requires authentication.

**webhook-event** — Manage webhook event

- `halopsa-pp-cli webhook-event create` — Create
- `halopsa-pp-cli webhook-event get` — Use this to return a single instance of WebhookEvent.<br> 				Requires authentication.
- `halopsa-pp-cli webhook-event list` — Use this to return multiple WebhookEvent.<br> 				Requires authentication.

**webhook-repository** — Manage webhook repository

- `halopsa-pp-cli webhook-repository get` — Use this to return a single instance of Webhook.<br> 				Requires authentication.
- `halopsa-pp-cli webhook-repository list` — Use this to return multiple Webhook.<br> 				Requires authentication.

**whats-app** — Manage whats app

- `halopsa-pp-cli whats-app list` — List
- `halopsa-pp-cli whats-app list-whatsapp` — List whatsapp

**wordpress-details** — Manage wordpress details

- `halopsa-pp-cli wordpress-details create` — Create
- `halopsa-pp-cli wordpress-details delete` — Delete
- `halopsa-pp-cli wordpress-details get` — Get
- `halopsa-pp-cli wordpress-details list` — List

**wordpress-org-details** — Manage wordpress org details

- `halopsa-pp-cli wordpress-org-details create` — Create
- `halopsa-pp-cli wordpress-org-details delete` — Delete
- `halopsa-pp-cli wordpress-org-details get` — Get
- `halopsa-pp-cli wordpress-org-details list` — List

**workday** — Manage workday

- `halopsa-pp-cli workday create` — Create
- `halopsa-pp-cli workday delete` — Delete
- `halopsa-pp-cli workday get` — Use this to return a single instance of Workdays.<br> 				Requires authentication.
- `halopsa-pp-cli workday list` — Use this to return multiple Workdays.<br> 				Requires authentication.

**workflow-target** — Manage workflow target

- `halopsa-pp-cli workflow-target create` — Create
- `halopsa-pp-cli workflow-target delete` — Delete
- `halopsa-pp-cli workflow-target get` — Get
- `halopsa-pp-cli workflow-target list` — List

**workflowstep** — Manage workflowstep

- `halopsa-pp-cli workflowstep` — Use this to return multiple FlowDetail.<br> 				Requires authentication.

**xero-details** — Manage xero details

- `halopsa-pp-cli xero-details create` — Create
- `halopsa-pp-cli xero-details delete` — Delete
- `halopsa-pp-cli xero-details get` — Use this to return a single instance of XeroDetails.<br> 				Requires authentication.
- `halopsa-pp-cli xero-details list` — Use this to return multiple XeroDetails.<br> 				Requires authentication.

**xtype-role** — Manage xtype role

- `halopsa-pp-cli xtype-role` — Use this to return multiple XTypeRole.<br> 				Requires authentication.

**zendesk** — Manage zendesk

- `halopsa-pp-cli zendesk` — List

**zoom** — Manage zoom

- `halopsa-pp-cli zoom` — Create


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
halopsa-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Monday queue cleanse

```bash
halopsa-pp-cli tickets age-out --status "Awaiting Customer Reply" --stale-days 14 --action-note "Auto-closing per policy"
```

Preview every stale customer-waiting ticket. Add --apply when you're ready to close them.

### Friday SLA radar before hand-off

```bash
halopsa-pp-cli sla breaching --within 24h --team Support --agent --select id,summary,client_name,agent_name,minutes_to_breach
```

List every ticket the on-call shift inherits that's at risk of breaching SLA in their first 24h. Pipe to anything.

### Pre-call client briefing

```bash
halopsa-pp-cli client card "Acme Corp" --agent --select active_tickets,contract_hours_remaining,assets,recent_kb_links
```

Get the client's complete situation in one query before answering the phone. Use --select to narrow what an agent sees.

### Friday timesheet reconcile

```bash
halopsa-pp-cli time gaps --agent me --week current --json
```

Find every ticket you touched this week with zero time logged so the gap doesn't ship with your timesheet.

### Contract overage check before client meeting

```bash
halopsa-pp-cli contracts burn --client "Acme Corp" --month current --json
```

See current hours consumed vs. bank with projected overage so the contract conversation isn't a surprise.

## Auth Setup

Halo uses OAuth2 client_credentials. Create an API application in your tenant under Configuration > Integrations > Halo PSA API (Authentication Method: Client ID and Secret — Services), then run `HALOPSA_TENANT=<yoursub> halopsa-pp-cli auth login --client-id <id> --client-secret <secret>`. The CLI exchanges the credentials at https://<tenant>.halopsa.com/auth/token and caches the access token (auto-refreshed before expiry).

Run `halopsa-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  halopsa-pp-cli actions list --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag
- **Explicit retries** — use `--idempotent` only when an already-existing create should count as success, and `--ignore-missing` only when a missing delete target should count as success

### Response envelope

Commands that read from the local store or the API wrap output in a provenance envelope:

```json
{
  "meta": {"source": "live" | "local", "synced_at": "...", "reason": "..."},
  "results": <data>
}
```

Parse `.results` for data and `.meta.source` to know whether it's live or local. A human-readable `N results (live)` summary is printed to stderr only when stdout is a terminal AND no machine-format flag (`--json`, `--csv`, `--compact`, `--quiet`, `--plain`, `--select`) is set — piped/agent consumers and explicit-format runs get pure JSON on stdout.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
halopsa-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
halopsa-pp-cli feedback --stdin < notes.txt
halopsa-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.halopsa-pp-cli/feedback.jsonl`. They are never POSTed unless `HALOPSA_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `HALOPSA_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

Write what *surprised* you, not a bug report. Short, specific, one line: that is the part that compounds.

## Output Delivery

Every command accepts `--deliver <sink>`. The output goes to the named sink in addition to (or instead of) stdout, so agents can route command results without hand-piping. Three sinks are supported:

| Sink | Effect |
|------|--------|
| `stdout` | Default; write to stdout only |
| `file:<path>` | Atomically write output to `<path>` (tmp + rename) |
| `webhook:<url>` | POST the output body to the URL (`application/json` or `application/x-ndjson` when `--compact`) |

Unknown schemes are refused with a structured error naming the supported set. Webhook failures return non-zero and log the URL + HTTP status on stderr.

## Named Profiles

A profile is a saved set of flag values, reused across invocations. Use it when a scheduled agent calls the same command every run with the same configuration - HeyGen's "Beacon" pattern.

```
halopsa-pp-cli profile save briefing --json
halopsa-pp-cli --profile briefing actions list
halopsa-pp-cli profile list --json
halopsa-pp-cli profile show briefing
halopsa-pp-cli profile delete briefing --yes
```

Explicit flags always win over profile values; profile values win over defaults. `agent-context` lists all available profiles under `available_profiles` so introspecting agents discover them at runtime.

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 2 | Usage error (wrong arguments) |
| 3 | Resource not found |
| 4 | Authentication required |
| 5 | API error (upstream issue) |
| 7 | Rate limited (wait and retry) |
| 10 | Config error |

## Argument Parsing

Parse `$ARGUMENTS`:

1. **Empty, `help`, or `--help`** → show `halopsa-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add halopsa-pp-mcp -- halopsa-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which halopsa-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   halopsa-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `halopsa-pp-cli <command> --help`.
