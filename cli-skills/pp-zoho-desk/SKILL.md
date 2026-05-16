---
name: pp-zoho-desk
description: "Every Zoho Desk API endpoint, plus offline FTS over conversation content and cross-agent insights no native view... Trigger phrases: `find tickets at risk`, `what's stale in zoho desk`, `agent workload`, `search support tickets`, `customer history in zoho`, `use zoho-desk-pp`, `run zoho-desk-pp`."
author: "Dan Bronson"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - zoho-desk-pp-cli
---
<!-- GENERATED FILE — DO NOT EDIT.
     This file is a verbatim mirror of library/sales-and-crm/zoho-desk/SKILL.md,
     regenerated post-merge by tools/generate-skills/. Hand-edits here are
     silently overwritten on the next regen. Edit the library/ source instead.
     See AGENTS.md "Generated artifacts: registry.json, cli-skills/". -->

# Zoho Desk — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `zoho-desk-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install zoho-desk --cli-only
   ```
2. Verify: `zoho-desk-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails before this CLI has a public-library category, install Node or use the category-specific Go fallback after publish.

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

Full coverage of Zoho Desk's tickets, contacts, accounts, agents, departments, and search modules — plus FTS5 over locally-synced ticket conversations, SLA-at-risk radar, workload fairness, reopen-pattern detection, and escalation audits. Built on a synced local SQLite store so support managers and agents can answer 'who has gaps?' and 'what's about to breach?' without rebuilding the Zoho UI in spreadsheets.

## When to Use This CLI

Reach for zoho-desk-pp-cli when you need to answer cross-entity questions about your support pipeline — SLA risk, workload distribution, reopen patterns, conversational search — without keeping Zoho's MCP server running. Also for cron-driven dashboards, BQ exports, and morning-triage automation. For one-off ticket CRUD an agent could call the API directly; use this CLI when the local store, FTS, or cross-agent joins matter.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`at-risk`** — List tickets at risk of breaching SLA in the next N hours, optionally filtered by unassigned.

  _Use this for cron-driven Slack alerts when tickets need an owner before they breach._

  ```bash
  zoho-desk-pp-cli at-risk --within 4h --unassigned --json
  ```
- **`aging`** — Open tickets where the last agent reply was more than N days ago.

  _Use this in your morning triage to find tickets the customer is waiting on._

  ```bash
  zoho-desk-pp-cli aging --days 5 --status open --json
  ```
- **`grep`** — FTS5 search across ticket subject, description, threads, and comments.

  _Use this to recall a customer's exact error string from six weeks ago without paginating the API._

  ```bash
  zoho-desk-pp-cli grep 'TLS handshake' --in comments,threads --json
  ```
- **`workload`** — Per-agent open-ticket counts with mean/stddev/Gini across the team.

  _Use this for sprint planning — surfaces who's overloaded vs underloaded._

  ```bash
  zoho-desk-pp-cli workload --department 'Support' --sort spread --json
  ```

### Agent-native plumbing
- **`reopens`** — Tickets that have been closed and reopened at least N times within a window.

  _Use this in weekly CSAT review to identify customers being repeatedly underserved._

  ```bash
  zoho-desk-pp-cli reopens --min-count 2 --window 30d --json
  ```
- **`suggest-agent`** — Suggests the least-loaded active agent in a ticket's department for reassignment.

  _Use this when manually reassigning — gives a defensible 'why this person' answer._

  ```bash
  zoho-desk-pp-cli suggest-agent 12345 --json
  ```
- **`contact-history`** — All tickets, accounts, and ticket stats for a contact in one query.

  _Use this before reaching out to a customer — see their full history in one shot._

  ```bash
  zoho-desk-pp-cli contact-history '<contact-email>' --json
  ```
- **`escalations`** — Tickets reassigned to N or more distinct agents — surfaces ownership churn.

  _Use this in governance review to find tickets bouncing across teams._

  ```bash
  zoho-desk-pp-cli escalations --min-reassigns 3 --json
  ```

## Command Reference

**accessible-organizations** — Manage accessible organizations

- `zoho-desk-pp-cli accessible-organizations` — This API lists all organizations which can be accessed using the current Oauth token.

**account-contact-mapping** — Manage account contact mapping

- `zoho-desk-pp-cli account-contact-mapping <mappingId>` — This API updates the details of an account-contact mapping.

**accounts** — Manage accounts

- `zoho-desk-pp-cli accounts bulk-update` — This API updates multiple accounts at once.
- `zoho-desk-pp-cli accounts create` — This API creates an account in your help desk portal.
- `zoho-desk-pp-cli accounts delete` — This API moves the accounts specified to the Recycle Bin.
- `zoho-desk-pp-cli accounts get` — This API lists a particular number of accounts, based on the limit specified.
- `zoho-desk-pp-cli accounts get-accountid` — This API fetches an account from your help desk portal.
- `zoho-desk-pp-cli accounts search` — This API searches for accounts in your help desk.<br/> You can provide multiple values separated by commas, and the...
- `zoho-desk-pp-cli accounts update` — This API updates details of an existing account.

**accounts-deduplication** — Manage accounts deduplication

- `zoho-desk-pp-cli accounts-deduplication get-default-field-name-for-account-deduplication` — This API fetches the name of the field set as the default field for account deduplication.
- `zoho-desk-pp-cli accounts-deduplication set-default-field-name-for-account-deduplication` — This API sets the field you specify as the default field for deduplicating accounts.

**actions** — Manage actions


**activities** — Manage activities

- `zoho-desk-pp-cli activities delete-spam` — This API deletes all spam activities.
- `zoho-desk-pp-cli activities search` — This API searches for the activities in your help desk portal.<br/> You can provide comma-separated values, which...

**agent-availability** — Manage agent availability

- `zoho-desk-pp-cli agent-availability` — This API lists the current availability of agents in a particular department.

**agent-availability-config** — Manage agent availability config

- `zoho-desk-pp-cli agent-availability-config get-config` — This API fetches the agent availability setting configured in your help desk portal.
- `zoho-desk-pp-cli agent-availability-config update-config` — This API updates the agent availability setting configured in your help desk portal.

**agents** — Manage agents

- `zoho-desk-pp-cli agents activate` — This API activates agents in your help desk.
- `zoho-desk-pp-cli agents add` — This API adds an agent to your help desk.<p><br>Keep in mind the following points while adding an agent: <ol>...
- `zoho-desk-pp-cli agents delete-unconfirmed` — This API deletes unconfirmed agents from your help desk.
- `zoho-desk-pp-cli agents get` — This API lists a particular number of agents, based on the limit specified.
- `zoho-desk-pp-cli agents get-agentid` — This API fetches details of an agent in your help desk.
- `zoho-desk-pp-cli agents get-count` — This API lists the agents count by status, confirmed and include light
- `zoho-desk-pp-cli agents re-invite` — This API sends reinvitation mails to unconfirmed agents.
- `zoho-desk-pp-cli agents update` — This API updates details of an agent.

**agents-by-ids** — Manage agents by ids

- `zoho-desk-pp-cli agents-by-ids` — This API fetches details of agents via the agent IDs passed in the API request.

**agents-tickets-count** — Manage agents tickets count

- `zoho-desk-pp-cli agents-tickets-count` — This API returns the number of tickets assigned to multiple agents.

**all-active-timers** — Manage all active timers

- `zoho-desk-pp-cli all-active-timers` — This API lists a particular number of currently active timers in a department, based on the limit specified.

**article-attachments** — Manage article attachments

- `zoho-desk-pp-cli article-attachments` — This API uploads files to your portal gallery.

**article-feedbacks** — Manage article feedbacks

- `zoho-desk-pp-cli article-feedbacks delete` — This API deletes a particular feedback comment from an article.
- `zoho-desk-pp-cli article-feedbacks get` — This API lists a particular number of comments received on an article, based on the limit defined.
- `zoho-desk-pp-cli article-feedbacks get-articlefeedbacks` — This API fetches the details of a particular feedback comment.

**article-translations** — Manage article translations

- `zoho-desk-pp-cli article-translations search` — This API searches for the provided string from within the translation of articles.
- `zoho-desk-pp-cli article-translations search-by-tag` — This API searches for the provided string from within the translation of articles.

**articles** — Manage articles

- `zoho-desk-pp-cli articles check-permalink-availability` — This API validates if a permalink is available for a help article.
- `zoho-desk-pp-cli articles create` — This API creates an article in your helpdesk.
- `zoho-desk-pp-cli articles delete` — This API moves articles to the Recycle Bin
- `zoho-desk-pp-cli articles get` — This API fetches a specific number of articles from your knowledge base, based on the limit defined.
- `zoho-desk-pp-cli articles get-count` — This API returns the number of articles published in the knowledge base of your help desk portal.
- `zoho-desk-pp-cli articles get-id` — This API fetches an article from your knowledge base.
- `zoho-desk-pp-cli articles preview` — This API shows a preview of help articles, through which you can check the content for formatting, alignment, and...
- `zoho-desk-pp-cli articles search-solutions` — This API searches for help articles in your help desk.<br/> You can provide multiple values separated by commas, and...
- `zoho-desk-pp-cli articles update` — This API updates an existing article.

**associated-tickets** — Manage associated tickets

- `zoho-desk-pp-cli associated-tickets` — This API lists a particular number of tickets that are associated to you from your help desk, based on the limit...

**automation-engine** — Manage automation engine

- `zoho-desk-pp-cli automation-engine` — This API fetches the supported Actions of an AutomationEngine

**automation-feature-count** — Manage automation feature count

- `zoho-desk-pp-cli automation-feature-count` — This API is used to Get Automation Feature Limits, For BusinessHours and HoldiayList, There are no mandatory params,...

**available-dependency-mappings** — Manage available dependency mappings

- `zoho-desk-pp-cli available-dependency-mappings` — This API fetches the parent and child fields using which you can configure dependency mappings in a layout.

**badges** — Manage badges

- `zoho-desk-pp-cli badges` — This API lists a particular number of default and custom badges, based on the limit defined.

**blueprints** — Manage blueprints

- `zoho-desk-pp-cli blueprints create` — To create a blueprint
- `zoho-desk-pp-cli blueprints delete` — To delete a specific blueprint
- `zoho-desk-pp-cli blueprints get` — To get a specific blueprint details
- `zoho-desk-pp-cli blueprints get-blue-prints` — To get all blueprints in a specific department
- `zoho-desk-pp-cli blueprints reorder` — To reorder blueprints in a specific department
- `zoho-desk-pp-cli blueprints update` — To update an existing blueprint

**bug** — Manage bug


**bulk-export** — Manage bulk export

- `zoho-desk-pp-cli bulk-export get` — To get Bulk Export details.
- `zoho-desk-pp-cli bulk-export initiate` — To initiate module Exports.

**bulk-import** — Manage bulk import


**business-hours** — Manage business hours

- `zoho-desk-pp-cli business-hours create` — This API creates a business hours set in your help desk portal
- `zoho-desk-pp-cli business-hours get` — This API lists a particular number of business hour sets, based on the limit specified
- `zoho-desk-pp-cli business-hours get-businesshours` — This API fetches the details of a business hours set configured in your help desk portal
- `zoho-desk-pp-cli business-hours update` — This API updates the details of a business hours set configured in your help desk portal

**calls** — Manage calls

- `zoho-desk-pp-cli calls bulk-update` — This API updates multiple calls at once.
- `zoho-desk-pp-cli calls create` — This API adds a call entry to your help desk portal.
- `zoho-desk-pp-cli calls delete` — This API moves call entries to the Recycle Bin of your help desk portal.
- `zoho-desk-pp-cli calls delete-all-spam` — This API deletes all spam calls.
- `zoho-desk-pp-cli calls delete-spam` — This API deletes the given spam calls
- `zoho-desk-pp-cli calls get` — This API fetches a particular number of calls, based on the limit specified.
- `zoho-desk-pp-cli calls get-callid` — This API fetches the details of a call.
- `zoho-desk-pp-cli calls search` — This API searches for the calls in your help desk portal.<br/> You can provide multiple values separated by commas,...
- `zoho-desk-pp-cli calls update` — This API updates the details of a call.

**channels** — Manage channels

- `zoho-desk-pp-cli channels get` — This API fetches currently installed channels including System, Channel integration and Instant Messaging channels
- `zoho-desk-pp-cli channels get-by-code` — This API gets fetches details of a given channel.

**close-tickets** — Manage close tickets

- `zoho-desk-pp-cli close-tickets` — This API closes multiple tickets at once.

**community** — Manage community

- `zoho-desk-pp-cli community mark-all-comments-as-spam` — This API marks multiple comments on a forum topic as spam.
- `zoho-desk-pp-cli community mark-all-topics-as-spam` — This API marks multiple forum topics as spam.

**community-attachments** — Manage community attachments

- `zoho-desk-pp-cli community-attachments` — This API uploads files to the storage space allocated for the community module.

**community-category** — Manage community category

- `zoho-desk-pp-cli community-category get` — This API lists a particular number of community categories.
- `zoho-desk-pp-cli community-category get-acategory` — This API get a community categories details.

**community-category-by-permalink** — Manage community category by permalink

- `zoho-desk-pp-cli community-category-by-permalink` — This API fetches the details of a community category or forum, based on the permalink in the API request.

**community-comments** — Manage community comments

- `zoho-desk-pp-cli community-comments approve-community-topic-comments` — This API approves comments that are pending moderation.
- `zoho-desk-pp-cli community-comments delete-community-topic-comments` — This API permanently deletes comments from the trash of your help desk portal.
- `zoho-desk-pp-cli community-comments delete-un-moderated-community-topic-comments` — This API trashes comments that are yet to be approved for publishing on a forum topic.
- `zoho-desk-pp-cli community-comments get-view-count` — This API fetches the total count of the comments based on the selected view.
- `zoho-desk-pp-cli community-comments restore-community-trashed-comments` — This API restores comments that were trashed from forum topics.

**community-moderated-topics** — Manage community moderated topics

- `zoho-desk-pp-cli community-moderated-topics` — This API lists a particular number of moderated topics, based on the limit defined.

**community-moderated-users** — Manage community moderated users

- `zoho-desk-pp-cli community-moderated-users` — This API lists a particular number of moderated end-users, based on the limit defined.

**community-moderation** — Manage community moderation

- `zoho-desk-pp-cli community-moderation get-community-topic-moderation-comments` — This API lists a particular number of comments that are pending moderation, based on the limit defined.
- `zoho-desk-pp-cli community-moderation get-moderation-counts` — This API fetches statistics related to topic/comment moderation in the user community.
- `zoho-desk-pp-cli community-moderation get-topics` — This API lists a particular number of forum topics that are pending moderation, based on the limit defined.

**community-my-drafted-topics** — Manage community my drafted topics

- `zoho-desk-pp-cli community-my-drafted-topics` — This API List all Drafted Topics of the current User

**community-preferences** — Manage community preferences

- `zoho-desk-pp-cli community-preferences get` — This API fetches the settings defined for the user community in your help desk portal.
- `zoho-desk-pp-cli community-preferences update` — This API helps define the settings for the user community in your help desk portal.

**community-topic-by-permalink** — Manage community topic by permalink

- `zoho-desk-pp-cli community-topic-by-permalink` — This API fetches a forum topic, based on the permalink in the API request.

**community-topic-types** — Manage community topic types

- `zoho-desk-pp-cli community-topic-types get` — This API fetches the topic types defined in the user community of your help desk portal.
- `zoho-desk-pp-cli community-topic-types update` — This API helps set the default topic type and enable/disable other topic types in the user community.

**community-topics** — Manage community topics

- `zoho-desk-pp-cli community-topics add` — This API helps add a forum topic in the user community.
- `zoho-desk-pp-cli community-topics add-as-draft` — This API saves a draft of a forum topic.
- `zoho-desk-pp-cli community-topics add-trashed` — This API restores topics from trash.
- `zoho-desk-pp-cli community-topics approve-topics` — This API helps to approve topics and publish them on the forum.
- `zoho-desk-pp-cli community-topics check-permalink-availability` — API to check whether the given permalink is available
- `zoho-desk-pp-cli community-topics delete-trashed` — This API permanently deletes forum topics from the trash.
- `zoho-desk-pp-cli community-topics delete-unmoderated` — This API trashes forum topics that are yet to be moderated for publishing.
- `zoho-desk-pp-cli community-topics get` — This API fetches a forum topic from the user community.
- `zoho-desk-pp-cli community-topics get-all` — This API lists a particular number of forum topics, based on the limit defined.
- `zoho-desk-pp-cli community-topics get-views-count` — This API fetches the total count of the topics based on the selected view.
- `zoho-desk-pp-cli community-topics unmoderate` — This API unmoderate the moderated forum topics.The comments that are posted during moderation will be displayed to...
- `zoho-desk-pp-cli community-topics update` — This API helps update a published forum topic.

**community-users** — Manage community users

- `zoho-desk-pp-cli community-users get-info` — This API fetches the details of an user from the community.
- `zoho-desk-pp-cli community-users unmoderate` — This API disables moderation for end-users in the user community.
- `zoho-desk-pp-cli community-users update-info` — This API updates the moderation status of end-users in the user community.

**contacts** — Manage contacts

- `zoho-desk-pp-cli contacts bulk-update` — This API updates multiple contacts at once.
- `zoho-desk-pp-cli contacts create` — This API creates a contact in your help desk portal.
- `zoho-desk-pp-cli contacts delete` — This API moves the contacts specified to the Recycle Bin.
- `zoho-desk-pp-cli contacts delete-spam` — This API deletes the given spam contacts
- `zoho-desk-pp-cli contacts get` — This API lists a particular number of contacts, based on the limit specified.
- `zoho-desk-pp-cli contacts get-by-ids` — This API lists details of specific contacts, based on the IDs passed in the request.
- `zoho-desk-pp-cli contacts get-contactid` — This API fetches a single contact from your help desk portal.
- `zoho-desk-pp-cli contacts get-count` — This API displays the count for the number of contacts in a custom view
- `zoho-desk-pp-cli contacts invite-as-end-user` — This API helps invite multiple contacts as end-users to your help center.
- `zoho-desk-pp-cli contacts mark-as-spam` — This API marks contacts as spam.
- `zoho-desk-pp-cli contacts search` — This API searches for contacts in your help desk.<br/> You can provide multiple values separated by commas, and the...
- `zoho-desk-pp-cli contacts update` — This API updates details of an existing contact.

**contacts-deduplication** — Manage contacts deduplication

- `zoho-desk-pp-cli contacts-deduplication get-default-field-name-for-contact-deduplication` — This API fetches the name of the field set as the default field for contact deduplication.
- `zoho-desk-pp-cli contacts-deduplication set-default-field-name-for-contact-deduplication` — This API sets the field you specify as the default field for deduplicating contacts.

**contracts** — Manage contracts

- `zoho-desk-pp-cli contracts create` — This API creates a contract in your helpdesk.
- `zoho-desk-pp-cli contracts get` — To get a list of contracts
- `zoho-desk-pp-cli contracts get-count` — This API return the count of contract based on the custom view and department specified. If ownerId is specified, it...
- `zoho-desk-pp-cli contracts get-id` — This API fetches a single contract from your helpdesk.
- `zoho-desk-pp-cli contracts update` — This API updates details of an existing contact.
- `zoho-desk-pp-cli contracts update-many` — This API updates multiple contracts at once.

**countries** — Manage countries

- `zoho-desk-pp-cli countries` — This API lists the countries that can be set in the locale field in Zoho Desk. Keep in mind that the...

**custom-field-count** — Manage custom field count

- `zoho-desk-pp-cli custom-field-count` — This API returns the custom field count allowed and available in a module.

**customer-happiness-link-holder** — Manage customer happiness link holder

- `zoho-desk-pp-cli customer-happiness-link-holder` — This API provides an html placeholder to insert the customer feedback link into the reply mail. The link, when...

**dashboards** — Manage dashboards

- `zoho-desk-pp-cli dashboards get` — To get a specific dashboard
- `zoho-desk-pp-cli dashboards get-backlog-tickets` — This API fetches the number of tickets that have remained unresolved over a particular period.
- `zoho-desk-pp-cli dashboards get-created-tickets` — This API fetches the number of tickets created in a particular duration.
- `zoho-desk-pp-cli dashboards get-onhold-tickets` — This API fetches the number of tickets that are in the on hold status.
- `zoho-desk-pp-cli dashboards get-reopened-tickets` — This API fetches the number of tickets that were reopened in your help desk portal.
- `zoho-desk-pp-cli dashboards get-resolution-time` — This API lists the durations taken to resolve tickets.
- `zoho-desk-pp-cli dashboards get-response-count` — This API fetches the total number of responses sent/made by your agents.
- `zoho-desk-pp-cli dashboards get-response-time` — This API lists the durations taken to respond to tickets.
- `zoho-desk-pp-cli dashboards get-solved-tickets` — This API fetches the number of tickets that are resolved.

**data-sharing-rules** — Manage data sharing rules

- `zoho-desk-pp-cli data-sharing-rules get` — This API fetches the different data sharing rules configured in your help desk portal.
- `zoho-desk-pp-cli data-sharing-rules update` — This API updates the data sharing rules configured in your help desk portal.

**delete-my-photo** — Manage delete my photo

- `zoho-desk-pp-cli delete-my-photo` — This API deletes the profile photo of the currently logged in agent.

**deleted-agents** — Manage deleted agents


**departments** — Manage departments

- `zoho-desk-pp-cli departments add` — This API adds a department to your help desk portal.
- `zoho-desk-pp-cli departments check-name-exist` — This API checks if multiple departments have the same name.
- `zoho-desk-pp-cli departments get` — This API lists a particular number of departments, based on the limit specified.
- `zoho-desk-pp-cli departments get-count` — This API returns the number of departments configured in your help desk portal.
- `zoho-desk-pp-cli departments get-departmentid` — This API fetches the details of a department from your help desk
- `zoho-desk-pp-cli departments update` — This API updates the details of an existing department.

**departments-by-ids** — Manage departments by ids

- `zoho-desk-pp-cli departments-by-ids` — This API fetches the details of the departments whose IDs are passed in the API request.

**dependency-mappings** — Manage dependency mappings

- `zoho-desk-pp-cli dependency-mappings` — This API lists the dependency mappings configured in a layout.

**domains** — Manage domains

- `zoho-desk-pp-cli domains add` — This API adds a domain to your help desk.
- `zoho-desk-pp-cli domains apply` — This API maps a domain to the default portal in your help desk.
- `zoho-desk-pp-cli domains delete-unused` — This API deletes unused domain from your help desk.
- `zoho-desk-pp-cli domains get` — This API fetches the details of a domain from your help desk.
- `zoho-desk-pp-cli domains list` — This API lists the domains configured in your help desk.
- `zoho-desk-pp-cli domains reset-current` — This API resets the current domain of the default portal in your help desk.
- `zoho-desk-pp-cli domains verify` — This API verifies a domain in your help desk.

**download-bulk-export-file** — Manage download bulk export file

- `zoho-desk-pp-cli download-bulk-export-file <exportId>` — To download the exported data as a ZIP file

**email-failure-alerts** — Manage email failure alerts

- `zoho-desk-pp-cli email-failure-alerts clear` — This API deletes all email delivery failure alerts configured in a particular department.
- `zoho-desk-pp-cli email-failure-alerts get` — This API lists the email delivery failure alerts configured in a particular department.

**engines** — Manage engines


**events** — Manage events

- `zoho-desk-pp-cli events bulk-update` — This API updates multiple events at once.
- `zoho-desk-pp-cli events create` — This API adds an event entry to your help desk portal.
- `zoho-desk-pp-cli events delete` — This API moves event entries to the Recycle Bin of your help desk portal.
- `zoho-desk-pp-cli events delete-all-spam` — This API deletes all spam events.
- `zoho-desk-pp-cli events delete-spam` — This API deletes the given spam events
- `zoho-desk-pp-cli events get` — This API lists a particular number of events, based on the limit specified.
- `zoho-desk-pp-cli events get-eventid` — This API fetches the details of an event.
- `zoho-desk-pp-cli events search` — This API searches for the events in your help desk portal.<br/> You can provide multiple values separated by commas,...
- `zoho-desk-pp-cli events update` — This API updates the details of an event.

**finance** — Manage finance

- `zoho-desk-pp-cli finance <id>` — To get associated tickets for invoice id.

**group-account-duplicate-values** — Manage group account duplicate values

- `zoho-desk-pp-cli group-account-duplicate-values` — This API lists the duplicate entries of an account.The _fieldName_ parameters is mandatory in the API request...

**group-accounts** — Manage group accounts

- `zoho-desk-pp-cli group-accounts` — This API lists all the details of duplicate accounts.The _fieldName_ and _fieldValues_ parameters are mandatory in...

**group-contact-duplicate-values** — Manage group contact duplicate values

- `zoho-desk-pp-cli group-contact-duplicate-values` — This API lists the duplicate entries of a contact.The _fieldName_ parameters is mandatory in the API request...

**group-contacts** — Manage group contacts

- `zoho-desk-pp-cli group-contacts` — This API lists all the details of duplicate contacts.The _fieldName_ and _fieldValues_ parameters are mandatory in...

**groups** — Manage groups

- `zoho-desk-pp-cli groups add-help-center` — This API creates a user group in your help center.
- `zoho-desk-pp-cli groups delete-help-center-user` — This API deletes a user group from your help center.
- `zoho-desk-pp-cli groups get-help-center` — This API fetches the details of a particular user group.
- `zoho-desk-pp-cli groups get-help-center-list` — This API lists a particular number of groups, based on the limit defined.
- `zoho-desk-pp-cli groups update-help-center-user` — This API helps update the details of a user group.

**help-centers** — Manage help centers

- `zoho-desk-pp-cli help-centers get` — This API fetches the details of a particular help center.
- `zoho-desk-pp-cli help-centers get-all` — This API lists the help centers configured in your Zoho Desk portal.

**holiday-list** — Manage holiday list

- `zoho-desk-pp-cli holiday-list create` — This API creates a holiday list in your help desk portal
- `zoho-desk-pp-cli holiday-list get` — This API lists a particular number of holiday lists, based on the limit specified
- `zoho-desk-pp-cli holiday-list get-holidaylist` — This API fetches the details of a holiday list configured in your help desk portal
- `zoho-desk-pp-cli holiday-list update` — This API updates the details of a holiday list configured in your help desk portal

**im** — Manage im

- `zoho-desk-pp-cli im get-agent-metrics` — This data provides leads/managers with a holistic details into agents performance and can be used in coaching them.
- `zoho-desk-pp-cli im get-canned-message` — This API fetches details of a template message.
- `zoho-desk-pp-cli im get-canned-messages` — This API lists all sessions
- `zoho-desk-pp-cli im get-imchannel` — This API fetches details of an channel.
- `zoho-desk-pp-cli im get-imchannels-list` — This API lists all channels
- `zoho-desk-pp-cli im get-session` — This API fetches details of an session.
- `zoho-desk-pp-cli im get-sessions` — This API lists all sessions
- `zoho-desk-pp-cli im initiate-session-using-template` — This API initiates a session with a template message

**import-mappings** — Manage import mappings

- `zoho-desk-pp-cli import-mappings` — This API fetches entity id's for given external ids your help desk portal.

**imports** — Manage imports

- `zoho-desk-pp-cli imports create` — This API add a import entry under a given scope in your help desk portal.
- `zoho-desk-pp-cli imports get` — This API fetches a single import info from your help desk portal.

**kb-category** — Manage kb category


**kb-category-logo** — Manage kb category logo

- `zoho-desk-pp-cli kb-category-logo` — This API is used to upload an image to be used as logo

**kb-root-categories** — Manage kb root categories

- `zoho-desk-pp-cli kb-root-categories add-kbroot-category` — This API creates a root category (i.e., parent category) in your knowledge base.
- `zoho-desk-pp-cli kb-root-categories get-all-kbroot-categories` — This API lists a particular number of root categories, based on the limit defined.
- `zoho-desk-pp-cli kb-root-categories get-kbroot-category` — This API fetches the details of a root category.

**kb-sections** — Manage kb sections

- `zoho-desk-pp-cli kb-sections add` — This API creates a section in your helpdesk
- `zoho-desk-pp-cli kb-sections get` — This API fetches a section in your helpdesk
- `zoho-desk-pp-cli kb-sections update` — This API updates a section in your helpdesk

**labels** — Manage labels

- `zoho-desk-pp-cli labels create` — This API creates a label in your help center.
- `zoho-desk-pp-cli labels delete` — This API deletes a label from your help center.
- `zoho-desk-pp-cli labels get` — This API lists a particular number of labels, based on the limit defined.
- `zoho-desk-pp-cli labels get-labelid` — This API fetches the details of a particular label.
- `zoho-desk-pp-cli labels update` — This API helps update the details of a label.

**languages** — Manage languages

- `zoho-desk-pp-cli languages` — This API lists the languages that are available for listing in Zoho Desk.

**last-accessed-view** — Manage last accessed view

- `zoho-desk-pp-cli last-accessed-view get-recent-view` — This API fetches the view last accessed by the user in the module and department specified in the request.
- `zoho-desk-pp-cli last-accessed-view update-recent-view` — This API updates the view last accessed by the user in the module and department specified in the request.

**layout-rules** — Manage layout rules

- `zoho-desk-pp-cli layout-rules` — This API lists all the rules configured for a department/layout.

**layouts** — Manage layouts

- `zoho-desk-pp-cli layouts create` — This API creates the layout
- `zoho-desk-pp-cli layouts delete` — This API deletes a layout
- `zoho-desk-pp-cli layouts get` — This API lists all the layouts configured for a module.
- `zoho-desk-pp-cli layouts get-layoutid` — This API fetches a layout configured for a module
- `zoho-desk-pp-cli layouts get-standard-format` — This Api provides the standrd layout format for the given module with organisations fields that can be used for...
- `zoho-desk-pp-cli layouts update` — This API updates details of an existing layout.

**licenses** — Manage licenses

- `zoho-desk-pp-cli licenses check-availability` — Checks if a specific license feature is available, potentially for a given entity and item count.
- `zoho-desk-pp-cli licenses feature-info` — Fetches the list of all license features, with optional filters for active status and specific feature codes. Useful...
- `zoho-desk-pp-cli licenses planinfo` — Retrieves details about the current license plan.

**light-agent-profile** — Manage light agent profile

- `zoho-desk-pp-cli light-agent-profile` — This API fetches the different permissions configured for the light agent profile.

**mail-reply-address** — Manage mail reply address

- `zoho-desk-pp-cli mail-reply-address` — This API lists the mailReplyAddresses configured in your help desk portal.

**modules** — Manage modules

- `zoho-desk-pp-cli modules` — The API fetches all the modules available in Zoho Desk. <br></br> <b> This API is <code>deprecated</code>, use...

**my-accessible-modules** — Manage my accessible modules

- `zoho-desk-pp-cli my-accessible-modules` — The API fetches a list of modules that are accessible to the current user profile.

**my-active-timers** — Manage my active timers

- `zoho-desk-pp-cli my-active-timers` — This API fetches currently running timer details for an agent

**my-form** — Manage my form

- `zoho-desk-pp-cli my-form` — This API fetches details of a layout, based on the profile of the current user.

**my-pending-approvals** — Manage my pending approvals

- `zoho-desk-pp-cli my-pending-approvals` — This API lists the pending approvals for a given user

**my-preferences** — Manage my preferences

- `zoho-desk-pp-cli my-preferences get` — This API fetches the preferences of the currently logged in agent.
- `zoho-desk-pp-cli my-preferences update` — This API updates the preferences of the currently logged in agent.

**my-profile** — Manage my profile

- `zoho-desk-pp-cli my-profile` — This API fetches the configuration details and permissions defined for the profile of the currently logged in user.

**my-profile-permissions** — Manage my profile permissions

- `zoho-desk-pp-cli my-profile-permissions` — This API fetches the permissions associated with the profile of the currently logged in user.

**myinfo** — Manage myinfo

- `zoho-desk-pp-cli myinfo` — This API fetches details of the currently logged in agent.

**offline-agents** — Manage offline agents

- `zoho-desk-pp-cli offline-agents` — This API lists the agents who are currently offline in a particular department.

**online-agents** — Manage online agents

- `zoho-desk-pp-cli online-agents` — This API lists the agents who are currently online in a particular department.

**organization-fields** — Manage organization fields

- `zoho-desk-pp-cli organization-fields create-field` — This API creates a field
- `zoho-desk-pp-cli organization-fields get-field` — This API get a field
- `zoho-desk-pp-cli organization-fields get-fields` — This API fetches fields in a module
- `zoho-desk-pp-cli organization-fields update-field` — This API edits a field

**organization-modules** — Manage organization modules

- `zoho-desk-pp-cli organization-modules create-module` — The API creates a Custom Module. <a href='https://help.zoho.com/portal/en/kb/desk/customization/modules/articles/crea...
- `zoho-desk-pp-cli organization-modules get-module` — This API fetches a single Module
- `zoho-desk-pp-cli organization-modules get-modules` — The API fetches all the modules.
- `zoho-desk-pp-cli organization-modules update-module` — The API updates a Module.

**organizations** — Manage organizations

- `zoho-desk-pp-cli organizations get` — This API lists all organizations to which the current user belongs.
- `zoho-desk-pp-cli organizations get-organizationid` — This API fetches the details of an organization from your help desk.
- `zoho-desk-pp-cli organizations update` — This API updates the details of an organization.
- `zoho-desk-pp-cli organizations update-default-organizaion` — This API updates the default organization for the current user in Zoho Desk.

**personal-role** — Manage personal role

- `zoho-desk-pp-cli personal-role` — This API fetches the details of the personal role configured in your help desk. Agents with personal role can view...

**products** — Manage products

- `zoho-desk-pp-cli products create` — This API adds a product to your helpdesk.
- `zoho-desk-pp-cli products delete` — This API moves products to the Recycle Bin of your help desk portal.
- `zoho-desk-pp-cli products get` — This API lists a specific number of products from your help desk portal, based on the limit defined. (Note:...
- `zoho-desk-pp-cli products get-productid` — This API fetches a single product from your helpdesk.
- `zoho-desk-pp-cli products search` — This API searches for the products in your help desk portal.<br/> You can provide multiple values separated by...
- `zoho-desk-pp-cli products search-duplicates` — This API searches for duplicate records of a product.
- `zoho-desk-pp-cli products update` — This API updates details of a product in your help desk portal.

**profiles** — Manage profiles

- `zoho-desk-pp-cli profiles get` — This API fetches the details of a particular profile.
- `zoho-desk-pp-cli profiles get-count` — This API fetches the number of profiles configured in your help desk.
- `zoho-desk-pp-cli profiles get-list` — This API lists a particular number of user profiles, based on the limit specified.
- `zoho-desk-pp-cli profiles update` — This API updates the details of an existing profile.

**recent-ticket-tags** — Manage recent ticket tags

- `zoho-desk-pp-cli recent-ticket-tags get-recent-tags` — This API lists the five most recent tags associated with tickets.
- `zoho-desk-pp-cli recent-ticket-tags update-recent-tags` — This API adds a tag to the list of recently viewed tags. tag_id is a mandatory parameter in the API request.

**recycle-bin** — Manage recycle bin

- `zoho-desk-pp-cli recycle-bin delete-all-entities` — This API permanently deletes all resources from the Recycle Bin.
- `zoho-desk-pp-cli recycle-bin delete-entities` — This API permanently deletes the resources specified in the API request. Additionally, the child resources are also...
- `zoho-desk-pp-cli recycle-bin get-entities` — This API lists a particular number of deleted resources, based on the limit specified.
- `zoho-desk-pp-cli recycle-bin restore-all-entities` — This API restores all deleted resources from the Recycle Bin.
- `zoho-desk-pp-cli recycle-bin restore-entities` — This API restores the deleted resources specified in the API request. Additionally, the parent resources are also...

**reports** — Manage reports

- `zoho-desk-pp-cli reports` — Retrieve the list of report fields for a specific module for integration.

**roles** — Manage roles

- `zoho-desk-pp-cli roles add` — This API creates a role in your help desk.
- `zoho-desk-pp-cli roles get` — This API lists a particular number of roles, based on the limit specified.
- `zoho-desk-pp-cli roles get-count` — This API fetches the number of roles configured in your help desk.
- `zoho-desk-pp-cli roles get-roleid` — This API fetches the details of a particular role.
- `zoho-desk-pp-cli roles update` — This API updates details of an existing role.

**roles-by-ids** — Manage roles by ids

- `zoho-desk-pp-cli roles-by-ids` — This API lists details of the roles whose IDs are passed in the API request.

**routing-preferences** — Manage routing preferences

- `zoho-desk-pp-cli routing-preferences get` — This API is used to Get the Routing Preferences for a Department
- `zoho-desk-pp-cli routing-preferences update` — This API is used to Update the Routing Preferences for a Department

**rulegroups** — Manage rulegroups

- `zoho-desk-pp-cli rulegroups create-rule-group` — This API creates a new RuleGroup.
- `zoho-desk-pp-cli rulegroups get-rule-groups` — This API returns a list of all rule groups configured for a specific engine.
- `zoho-desk-pp-cli rulegroups update-rule-group` — This API updates the rulegroup

**skill-configuration** — Manage skill configuration

- `zoho-desk-pp-cli skill-configuration get` — This API gets Configuration of Skill in a Department
- `zoho-desk-pp-cli skill-configuration update` — This API updates Configuration of Skill in a Department

**skill-types** — Manage skill types

- `zoho-desk-pp-cli skill-types create` — This API Creates a Skill Type
- `zoho-desk-pp-cli skill-types delete` — This API Deletes a Skill Type
- `zoho-desk-pp-cli skill-types get` — This API Lists all Skill Types in a department
- `zoho-desk-pp-cli skill-types get-skilltypes` — This API Gets the details of a Skill Type
- `zoho-desk-pp-cli skill-types update` — This API Updates a Skill Type

**skills** — Manage skills

- `zoho-desk-pp-cli skills create` — This API Creates a skill
- `zoho-desk-pp-cli skills delete` — This API deletes a skill from your help desk portal
- `zoho-desk-pp-cli skills get` — This API lists all skills in a department
- `zoho-desk-pp-cli skills get-criteria-fields-for` — This API returns list of fields supported in criteria for Skills by module
- `zoho-desk-pp-cli skills get-skillid` — This API Gets the details of a skill
- `zoho-desk-pp-cli skills reorder` — This API lists reorders Skills in a Department
- `zoho-desk-pp-cli skills update` — This API Updates a skill

**starred-views** — Manage starred views

- `zoho-desk-pp-cli starred-views get` — This API lists the starred views in a module. Number of resources in the starred view is displayed only for the...
- `zoho-desk-pp-cli starred-views update-order` — This API helps reorder the starred views in a module.

**subject-access-requests** — Manage subject access requests

- `zoho-desk-pp-cli subject-access-requests get` — This API fetches the details of a particular SAR.
- `zoho-desk-pp-cli subject-access-requests get-count` — This API returns the number of resources related to a subject access request.
- `zoho-desk-pp-cli subject-access-requests get-fields-and-conditions` — This API fetchs possible fileds list and conditions
- `zoho-desk-pp-cli subject-access-requests get-list` — This API lists a particular number of subject access requests, based on the limit specified.
- `zoho-desk-pp-cli subject-access-requests sar-erase` — This API erases specific resources related to a subject access request.
- `zoho-desk-pp-cli subject-access-requests sar-erase-all` — This API erases data related to the subject, from your Zoho Desk portal.
- `zoho-desk-pp-cli subject-access-requests sar-export` — This API exports specific resources related to a subject access request.
- `zoho-desk-pp-cli subject-access-requests sar-export-all` — This API exports data related to the subject, from your Zoho Desk portal.

**support-email-domain** — Manage support email domain

- `zoho-desk-pp-cli support-email-domain get-support-email-address-domain` — This API fetches the subdomain of the support email address.
- `zoho-desk-pp-cli support-email-domain update-support-email-address-domain` — This API updates the subdomain (the '~mycompany~' part) of the support email address.!!1. Only the primary contact...

**support-plans** — Manage support plans

- `zoho-desk-pp-cli support-plans create` — This API Creates a Support Plan
- `zoho-desk-pp-cli support-plans delete` — This API deletes a support plan.
- `zoho-desk-pp-cli support-plans get` — This API lists all support plans in a department
- `zoho-desk-pp-cli support-plans get-details` — To get details of a specific support plan
- `zoho-desk-pp-cli support-plans update` — This API Updates the Support Plan

**tags** — Manage tags

- `zoho-desk-pp-cli tags` — This API searches for tags added in your help desk portal.

**tasks** — Manage tasks

- `zoho-desk-pp-cli tasks bulk-update` — This API updates multiple tasks at once.
- `zoho-desk-pp-cli tasks create` — This API creates a task in your help desk portal.
- `zoho-desk-pp-cli tasks delete` — This API moves task entries to the Recycle Bin of your help desk portal.
- `zoho-desk-pp-cli tasks delete-all-spam` — This API deletes all spam tasks.
- `zoho-desk-pp-cli tasks delete-spam` — This API deletes the given spam tasks
- `zoho-desk-pp-cli tasks get` — This API fetches a particular number of tasks, based on the limit specified.
- `zoho-desk-pp-cli tasks get-count` — This API returns the number of tasks in your help desk.
- `zoho-desk-pp-cli tasks get-taskid` — This API fetches a task from your help desk portal.
- `zoho-desk-pp-cli tasks search` — This API searches for tasks in your help desk.<br/> You can provide multiple values separated by commas, and the...
- `zoho-desk-pp-cli tasks update` — This API helps update the details of a task.

**teams** — Manage teams

- `zoho-desk-pp-cli teams create` — This API creates a team in your help desk portal.
- `zoho-desk-pp-cli teams get` — This API fetches details of all teams created in all departments to which the current user belongs.
- `zoho-desk-pp-cli teams get-teamid` — This API fetches the details of a team.
- `zoho-desk-pp-cli teams update` — This API updates details of an existing team.

**template-folders** — Manage template folders

- `zoho-desk-pp-cli template-folders add` — Adding a Template Folder
- `zoho-desk-pp-cli template-folders delete` — Delete a Template Folder
- `zoho-desk-pp-cli template-folders get` — List all Template Folders in alphabetical order
- `zoho-desk-pp-cli template-folders get-templatefolders` — Listing a particular Template Folder
- `zoho-desk-pp-cli template-folders update` — Updating a Template Folder

**templates** — Manage templates

- `zoho-desk-pp-cli templates add` — Add a new Template
- `zoho-desk-pp-cli templates delete` — Delete a Template
- `zoho-desk-pp-cli templates get` — List all Templates
- `zoho-desk-pp-cli templates get-emailtemplateid` — View a particular Template
- `zoho-desk-pp-cli templates get-place-holders` — List the placeholders supported in emailTemplates
- `zoho-desk-pp-cli templates update` — Update an existing Template

**ticket-queue-view** — Manage ticket queue view

- `zoho-desk-pp-cli ticket-queue-view <agentId>` — This API returns the number of tickets in a particular view.

**ticket-tags** — Manage ticket tags

- `zoho-desk-pp-cli ticket-tags` — This API lists the ticket tags added in your help desk portal.�

**ticket-templates** — Manage ticket templates

- `zoho-desk-pp-cli ticket-templates create` — This API helps create a ticket template in your help desk portal.
- `zoho-desk-pp-cli ticket-templates delete` — This API deletes a ticket template from your help desk portal.
- `zoho-desk-pp-cli ticket-templates get` — This API lists a particular number of ticket templates, based on the limit specified.
- `zoho-desk-pp-cli ticket-templates get-tickettemplates` — This API fetches the details of a particular ticket template.
- `zoho-desk-pp-cli ticket-templates update` — This API helps update the details of a particular ticket template.

**tickets** — Manage tickets

- `zoho-desk-pp-cli tickets bulk-update` — This API updates multiple tickets at once.
- `zoho-desk-pp-cli tickets create` — This API creates a ticket in your helpdesk.
- `zoho-desk-pp-cli tickets delete` — This API moves tickets to the Recycle Bin
- `zoho-desk-pp-cli tickets delete-all-spam` — This API deletes all spam tickets.
- `zoho-desk-pp-cli tickets delete-spam` — This API deletes the given spam tickets
- `zoho-desk-pp-cli tickets get` — This API lists a particular number of tickets, based on the limit specified.
- `zoho-desk-pp-cli tickets get-archived` — This API gets the archived tickets list in given department.
- `zoho-desk-pp-cli tickets get-ticketid` — This API fetches a single ticket from your helpdesk.
- `zoho-desk-pp-cli tickets mark-as-spam` — This API marks tickets as spam.
- `zoho-desk-pp-cli tickets search` — This API searches for tickets in your help desk.<br/> You can provide multiple values separated by commas, and the...
- `zoho-desk-pp-cli tickets update` — This API updates the details of an existing ticket.

**tickets-count** — Manage tickets count

- `zoho-desk-pp-cli tickets-count <contactId>` — This API returns the ticket count of your help desk.

**tickets-count-by-field-values** — Manage tickets count by field values

- `zoho-desk-pp-cli tickets-count-by-field-values <contactId>` — This API returns the ticket count of your help desk, filtered by a specific field.

**time-track-history** — Manage time track history

- `zoho-desk-pp-cli time-track-history` — This API fetches the history of changes made to the billing preferences in time tracking settings. The different...

**time-track-settings** — Manage time track settings

- `zoho-desk-pp-cli time-track-settings get-time-tracking-setup` — This API fetches the details of the TimeTrack Settings configured in your helpdesk.
- `zoho-desk-pp-cli time-track-settings save-time-tracking-settings` — This API adds a TimeTracking configuration to your helpdesk.
- `zoho-desk-pp-cli time-track-settings update-time-tracking-settings` — This API updates an existing TimeTracking configuration.

**time-zones** — Manage time zones

- `zoho-desk-pp-cli time-zones` — This API lists the time zones that are available for listing in Zoho Desk.

**upload-my-photo** — Manage upload my photo

- `zoho-desk-pp-cli upload-my-photo` — This API sets the profile photo for the currently logged in agent.<p><b>Note</b>: To upload your photo generate...

**uploads** — Manage uploads

- `zoho-desk-pp-cli uploads` — This API uploads a file.

**users** — Manage users

- `zoho-desk-pp-cli users get` — This API lists a particular number of help center users, based on the limit defined. It also helps you search for...
- `zoho-desk-pp-cli users get-details` — This API fetches the details of a particular help center user.
- `zoho-desk-pp-cli users update-details` — This API helps update the details of a particular help center user.

**validation-rules** — Manage validation rules

- `zoho-desk-pp-cli validation-rules` — This API fetches all the validation rules configured for your department/layout.

**views** — Manage views

- `zoho-desk-pp-cli views` — This API lists the different views configured for a specific module or for all modules in your help desk portal.

**webhooks** — Manage webhooks

- `zoho-desk-pp-cli webhooks create` — This API creates a webhook in your help desk. When the API is invoked, a validation GET request is sent to the...
- `zoho-desk-pp-cli webhooks delete` — This API deletes an existing webhook. After this API is executed, the URL in the webhook stops receiving event...
- `zoho-desk-pp-cli webhooks get` — This API lists all webhooks configured in your help desk.
- `zoho-desk-pp-cli webhooks get-webhookid` — This API fetches a single webhook from your help desk.
- `zoho-desk-pp-cli webhooks update` — This API updates the details of an existing webhook. When the webhook's URL is altered, a validation GET request is...

**widgets** — Manage widgets

- `zoho-desk-pp-cli widgets <widgetId>` — Delete a widget using widget ID

**zoho-desk-search** — Manage zoho desk search

- `zoho-desk-pp-cli zoho-desk-search` — This API returns information from all modules or a specific module, based on the value of the module query param.

**zoho-finance** — Manage zoho finance



### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
zoho-desk-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Morning triage (SLA + stale)

```bash
zoho-desk-pp-cli at-risk --within 8h --unassigned --json --select id,subject,due_date,department
```

Lists unassigned tickets approaching SLA breach. Pipe into Slack.

### Customer recall (deep --select)

```bash
zoho-desk-pp-cli grep 'TLS handshake' --in comments,threads --from 2026-01-01 --json --select id,ticket.subject,thread.content,thread.created_time
```

FTS5 over local notes; dotted --select paths narrow the response to only the fields you need.

### Workload check

```bash
zoho-desk-pp-cli workload --department 'Support' --sort spread --json --select agent,open_count,share_pct
```

Surfaces uneven assignment — pair with `suggest-agent` to rebalance.

### Customer 360

```bash
zoho-desk-pp-cli contact-history '<contact-email>' --json
```

All tickets + account + stats for that contact in one shot.

### Escalation governance

```bash
zoho-desk-pp-cli escalations --min-reassigns 3 --json
```

Tickets bouncing across agents — signal for process review.

## Auth Setup

Zoho Desk uses OAuth 2.0 only. You need a self-client refresh token, client ID/secret, and your org ID. Set ZOHODESK_REFRESH_TOKEN, ZOHODESK_CLIENT_ID, ZOHODESK_CLIENT_SECRET, ZOHODESK_ORG_ID — or run `zoho-desk-pp-cli auth set-token` to store them locally. The CLI auto-refreshes access tokens.

Run `zoho-desk-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  zoho-desk-pp-cli accessible-organizations --agent --select id,name,status
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
zoho-desk-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
zoho-desk-pp-cli feedback --stdin < notes.txt
zoho-desk-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.zoho-desk-pp-cli/feedback.jsonl`. They are never POSTed unless `ZOHO_DESK_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `ZOHO_DESK_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
zoho-desk-pp-cli profile save briefing --json
zoho-desk-pp-cli --profile briefing accessible-organizations
zoho-desk-pp-cli profile list --json
zoho-desk-pp-cli profile show briefing
zoho-desk-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `zoho-desk-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add zoho-desk-pp-mcp -- zoho-desk-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which zoho-desk-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   zoho-desk-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `zoho-desk-pp-cli <command> --help`.
