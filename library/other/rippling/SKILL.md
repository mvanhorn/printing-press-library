---
name: pp-rippling
description: "Printing Press CLI for Rippling. Documentation for the Rippling Platform API."
author: "Carter"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - rippling-pp-cli
    install:
      - kind: go
        bins: [rippling-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/other/rippling/cmd/rippling-pp-cli
---

# Rippling — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `rippling-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install rippling --cli-only
   ```
2. Verify: `rippling-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.23+):

```bash
go install github.com/mvanhorn/printing-press-library/library/other/rippling/cmd/rippling-pp-cli@latest
```

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

Documentation for the Rippling Platform API.

## IT workflows

The published CLI includes an `it` namespace for Tier-1, public-API-backed IT support:

- `rippling-pp-cli it overview --json` — summarizes supported IT-adjacent public APIs and explicit public API gaps.
- `rippling-pp-cli it onboarding-plan --json` — builds a safe dry-run-first new-hire IT setup plan; use explicit execution flags before draft-hire/software-deployment writes.
- `rippling-pp-cli it deploy-software --json` — IT-friendly wrapper around software deployment list/create workflows.
- `rippling-pp-cli it inventory-schema --json` — prints hardware inventory/report field names for report-driven device inventory work.

Known public API gaps: no public endpoints were found in the REST-beta spec for laptop/device ordering, shipping, retrieval, remote lock, or wipe. Do not invent those operations; use `it overview` to show the gap and route those actions to the Rippling UI or an approved private/admin integration.

## Command Reference

**break-types** — Break types configured by the company for time tracking

- `rippling-pp-cli break-types get` — Retrieve a specific break type
- `rippling-pp-cli break-types list` — A list of break types. - Requires: `API Tier 2` - Sortable fields: `id`, `created_at`, `updated_at`

**business-partner-groups** — Manage business partner groups

- `rippling-pp-cli business-partner-groups create` — Create a new business partner group
- `rippling-pp-cli business-partner-groups delete` — Delete a business partner group
- `rippling-pp-cli business-partner-groups get` — Retrieve a specific business partner group
- `rippling-pp-cli business-partner-groups list` — A list of business partner groups. - Requires: `API Tier 1` - Expandable fields: `default_business_partner` -...

**business-partners** — Business partners of the company

- `rippling-pp-cli business-partners create` — Create a new business partner
- `rippling-pp-cli business-partners delete` — Delete a business partner
- `rippling-pp-cli business-partners get` — Retrieve a specific business partner
- `rippling-pp-cli business-partners list` — A list of business partners. - Requires: `API Tier 1` - Filterable fields: `worker_id`, `business_partner_group_id`...

**candidate-applications** — An application by a candidate to a specific job requisition

- `rippling-pp-cli candidate-applications` — A list of candidate applications. - Requires: `API Tier 2` - Expandable fields: `job` - Sortable fields: `id`,...

**candidates** — Someone who applies to a job requisition opened by the company

- `rippling-pp-cli candidates` — A list of candidates. - Requires: `API Tier 2` - Sortable fields: `id`, `created_at`, `updated_at`

**companies** — Companies on Rippling

- `rippling-pp-cli companies` — A list of companies. - Requires: `API Tier 1` - Expandable fields: `parent_legal_entity`, `legal_entities` -...

**company-legal-entities** — Manage company legal entities

- `rippling-pp-cli company-legal-entities get` — Retrieve a specific company legal entity
- `rippling-pp-cli company-legal-entities list` — A list of company legal entities. - Requires: `API Tier 2` - Filterable fields: `country_code` - Sortable fields:...

**company-legal-entity-workers** — Manage company legal entity workers

- `rippling-pp-cli company-legal-entity-workers get` — Retrieve a specific company-legal-entity-worker
- `rippling-pp-cli company-legal-entity-workers list` — A list of company-legal-entity-workers. - Requires: `API Tier 2` - Filterable fields: `status`, `company_entity_id`...

**compensation-bands-details** — Compensation bands details associated with workers

- `rippling-pp-cli compensation-bands-details get` — Retrieve a specific compensation bands detail
- `rippling-pp-cli compensation-bands-details list` — A list of compensation bands details. - Requires: `API Tier 2` - Filterable fields: `created_at`, `updated_at`,...

**compensations** — Compensation associated with workers

- `rippling-pp-cli compensations get` — Retrieves the Compensation for the Worker with the ID provided in the URL path.
- `rippling-pp-cli compensations list` — A list of compensations. - Requires: `API Tier 2` - Expandable fields: `worker` - Sortable fields: `id`,...

**custom-apps** — Manage custom apps

- `rippling-pp-cli custom-apps create` — Create a new CustomApp
- `rippling-pp-cli custom-apps delete` — Delete a CustomApp
- `rippling-pp-cli custom-apps get` — Retrieve a specific CustomApp
- `rippling-pp-cli custom-apps list` — A list of CustomApp. - Requires: `Custom Apps Platform API`
- `rippling-pp-cli custom-apps update` — Update a specific CustomApp.

**custom-fields** — Custom fields defined by the company

- `rippling-pp-cli custom-fields` — A list of custom fields. - Requires: `API Tier 1` - Sortable fields: `id`, `created_at`, `updated_at`

**custom-objects** — Custom objects defined by the company

- `rippling-pp-cli custom-objects create` — Create a new custom object.
- `rippling-pp-cli custom-objects delete` — Delete a custom object.
- `rippling-pp-cli custom-objects get` — Retrieve a specific custom object.
- `rippling-pp-cli custom-objects list` — A list of custom objects. - Requires: `API Tier 1`
- `rippling-pp-cli custom-objects update` — Update a specific custom object.

**custom-pages** — Manage custom pages

- `rippling-pp-cli custom-pages create` — Create a new CustomPage
- `rippling-pp-cli custom-pages delete` — Delete a CustomPage
- `rippling-pp-cli custom-pages get` — Retrieve a specific CustomPage
- `rippling-pp-cli custom-pages list` — A list of CustomPage. - Requires: `Custom Apps Platform API`
- `rippling-pp-cli custom-pages update` — Update a specific CustomPage.

**custom-settings** — Manage custom settings

- `rippling-pp-cli custom-settings create` — Create a new Setting
- `rippling-pp-cli custom-settings delete` — Delete a Setting
- `rippling-pp-cli custom-settings get` — Retrieve a specific Setting
- `rippling-pp-cli custom-settings list` — A list of Setting. - Requires: `Functions API` - Sortable fields: `id`, `created_at`, `updated_at`
- `rippling-pp-cli custom-settings update` — Update a specific Setting.

**departments** — Departments used by the company

- `rippling-pp-cli departments create` — Create a new department
- `rippling-pp-cli departments get` — Retrieve a specific department
- `rippling-pp-cli departments list` — A list of departments. - Requires: `API Tier 1` - Expandable fields: `parent`, `department_hierarchy` - Sortable...
- `rippling-pp-cli departments update` — Update a specific department.

**draft-hires** — Candidates who have not yet started work at the company

- `rippling-pp-cli draft-hires` — Create bulk draft hire

**earning-types** — Earning types configured for the company

- `rippling-pp-cli earning-types` — A list of earning-types. - Requires: `Global Payroll`

**earnings-inputs** — External earnings inputs for the company

- `rippling-pp-cli earnings-inputs create` — Create a new earnings-input
- `rippling-pp-cli earnings-inputs delete` — Delete a earnings-input
- `rippling-pp-cli earnings-inputs list` — A list of earnings-inputs. - Requires: `Global Payroll`

**employment-types** — Employment types used by the company

- `rippling-pp-cli employment-types get` — Retrieve a specific employment type
- `rippling-pp-cli employment-types list` — A list of employment types. - Requires: `API Tier 1` - Sortable fields: `id`, `created_at`, `updated_at`

**entitlements** — Availability of API features to the company or Partners.

- `rippling-pp-cli entitlements` — A list of entitlements. - Requires: `API Tier 1`

**files** — Manage files

- `rippling-pp-cli files create` — Create a new file
- `rippling-pp-cli files get` — Retrieve a specific file

**functions** — Functions defined by the company

- `rippling-pp-cli functions create` — Create a new function.
- `rippling-pp-cli functions get` — Retrieve a specific function.
- `rippling-pp-cli functions list` — A list of functions. - Requires: `API Tier 2` - Sortable fields: `id`, `created_at`, `updated_at`
- `rippling-pp-cli functions update` — Update a function

**headcount-positions** — Headcount allocations for the company

- `rippling-pp-cli headcount-positions get` — Retrieve a specific headcount position
- `rippling-pp-cli headcount-positions list` — A list of headcount positions. - Requires: `API Tier 2` - Filterable fields: `created_at`, `position_id`,...

**headcount-priorities** — Manage headcount priorities

- `rippling-pp-cli headcount-priorities get` — Retrieve a specific headcount priority
- `rippling-pp-cli headcount-priorities list` — A list of headcount priorities. - Requires: `API Tier 2` - Sortable fields: `id`, `created_at`, `updated_at`

**job-assignments** — Manage job assignments

- `rippling-pp-cli job-assignments create` — Create a new job assignment
- `rippling-pp-cli job-assignments delete` — Delete a job assignment
- `rippling-pp-cli job-assignments get` — Retrieve a specific job assignment
- `rippling-pp-cli job-assignments list` — A list of job assignments. - Requires: `API Tier 2` - Filterable fields: `worker_id`, `job_code_id` - Expandable...
- `rippling-pp-cli job-assignments update` — Update a specific job assignment.

**job-codes** — Manage job codes

- `rippling-pp-cli job-codes create` — Create a new job code
- `rippling-pp-cli job-codes delete` — Delete a job code
- `rippling-pp-cli job-codes get` — Retrieve a specific job code
- `rippling-pp-cli job-codes list` — A list of job codes. - Requires: `API Tier 2` - Filterable fields: `job_dimension_id`, `group_id` - Expandable...
- `rippling-pp-cli job-codes update` — Update a specific job code.

**job-dimensions** — Manage job dimensions

- `rippling-pp-cli job-dimensions create` — Create a new job dimension
- `rippling-pp-cli job-dimensions delete` — Delete a job dimension
- `rippling-pp-cli job-dimensions get` — Retrieve a specific job dimension
- `rippling-pp-cli job-dimensions list` — A list of job dimensions. - Requires: `API Tier 2` - Filterable fields: `name` - Sortable fields: `id`,...
- `rippling-pp-cli job-dimensions update` — Update a specific job dimension.

**job-functions** — Organizational job categories that group similar roles and responsibilities within a company

- `rippling-pp-cli job-functions get` — Retrieve a specific job function
- `rippling-pp-cli job-functions list` — A list of job functions. - Requires: `API Tier 1` - Sortable fields: `id`, `created_at`, `updated_at`

**job-pay-rate-exceptions** — Manage job pay rate exceptions

- `rippling-pp-cli job-pay-rate-exceptions get` — Retrieve a specific job pay rate exception
- `rippling-pp-cli job-pay-rate-exceptions list` — A list of job pay rate exceptions. - Requires: `API Tier 2` - Filterable fields: `job_code_id` - Expandable fields:...

**job-requisitions** — A request for a job to be filled by a candidate

- `rippling-pp-cli job-requisitions` — A list of job requisitions. - Requires: `API Tier 2` - Sortable fields: `id`, `created_at`, `updated_at`

**job-requisitions-write** — Manage job requisitions write

- `rippling-pp-cli job-requisitions-write` — Create a new job requisition

**kiosk-badges** — Badge information used with Timeclock Kiosk

- `rippling-pp-cli kiosk-badges create` — Create a new kiosk badge
- `rippling-pp-cli kiosk-badges delete` — Delete a kiosk badge
- `rippling-pp-cli kiosk-badges get` — Retrieve a specific kiosk badge
- `rippling-pp-cli kiosk-badges list` — A list of kiosk badges. - Requires: `API Tier 2` - Filterable fields: `badge_id` - Expandable fields: `worker` -...
- `rippling-pp-cli kiosk-badges update` — Update a specific kiosk badge.

**leave-accruals** — Leave accruals for workers

- `rippling-pp-cli leave-accruals create` — Create a new leave accrual
- `rippling-pp-cli leave-accruals get` — Retrieve a specific leave accrual
- `rippling-pp-cli leave-accruals list` — A list of leave accruals. - Requires: `API Tier 2` - Filterable fields: `worker_id`, `leave_type_id`, `accrual_date`...

**leave-balances** — Leave balances for workers

- `rippling-pp-cli leave-balances get` — Retrieve a specific leave balance
- `rippling-pp-cli leave-balances list` — A list of leave balances. - Requires: `API Tier 2` - Filterable fields: `worker_id`, `leave_type_id` - Expandable...

**leave-requests** — Leave requests submitted by workers

- `rippling-pp-cli leave-requests create` — Create a new leave request
- `rippling-pp-cli leave-requests get` — Retrieve a specific leave request
- `rippling-pp-cli leave-requests list` — A list of leave requests. - Requires: `API Tier 2` - Filterable fields: `worker_id`, `requester_id`, `reviewer_id`,...
- `rippling-pp-cli leave-requests update` — Update a specific leave request.

**leave-types** — Leave types used by the company

- `rippling-pp-cli leave-types get` — Retrieve a specific leave type
- `rippling-pp-cli leave-types list` — A list of leave types. - Requires: `API Tier 2` - Filterable fields: `name` - Sortable fields: `id`, `created_at`,...

**legal-entities** — Legal entities registered by the company

- `rippling-pp-cli legal-entities get` — Retrieve a specific legal entity
- `rippling-pp-cli legal-entities list` — A list of legal entities. - Requires: `API Tier 2` - Expandable fields: `parent`, `company` - Sortable fields: `id`,...

**levels** — Manage levels

- `rippling-pp-cli levels get` — Retrieve a specific level
- `rippling-pp-cli levels list` — A list of levels. - Requires: `API Tier 2` - Expandable fields: `parent`, `track` - Sortable fields: `id`,...

**location-factors** — Geographic compensation adjustment factors that modify base compensation based on location-specific market conditions and cost of living

- `rippling-pp-cli location-factors get` — Retrieve a specific location factor
- `rippling-pp-cli location-factors list` — A list of location factors. - Requires: `API Tier 2` - Sortable fields: `id`, `created_at`, `updated_at`

**metadata** — Manage metadata

- `rippling-pp-cli metadata list` — A list of metadata components. - Requires: `Metadata Platform API`
- `rippling-pp-cli metadata list-components` — A list of metadata components. - Requires: `Metadata Platform API`
- `rippling-pp-cli metadata list-summary` — A list of metadata summary. - Requires: `Metadata Platform API`
- `rippling-pp-cli metadata list-types` — A list of metadata types. - Requires: `Metadata Platform API`

**object-categories** — Object Categories defined by the company

- `rippling-pp-cli object-categories create` — Create a new object category.
- `rippling-pp-cli object-categories delete` — Delete an object category.
- `rippling-pp-cli object-categories get` — Retrieve a specific object category.
- `rippling-pp-cli object-categories list` — A list of object categories. - Requires: `API Tier 1`
- `rippling-pp-cli object-categories update` — Update a specific object category.

**payroll-runs** — Payroll runs for the company

- `rippling-pp-cli payroll-runs get` — Retrieve a specific payroll-run
- `rippling-pp-cli payroll-runs list` — A list of payroll-runs. - Requires: `Global Payroll` - Sortable fields: `check_date`

**platform-capabilities** — Manage platform capabilities

- `rippling-pp-cli platform-capabilities` — A list of platform capabilities. - Requires: `Platform Capabilities API`

**report-runs** — Manage report runs

- `rippling-pp-cli report-runs get` — Retrieve the status and data for a specific report run
- `rippling-pp-cli report-runs trigger` — Trigger a new report run

**reports** — Report data for company

- `rippling-pp-cli reports <id>` — Retrieve a specific report by its ID

**scheduled-job-assignments** — Manage scheduled job assignments

- `rippling-pp-cli scheduled-job-assignments delete` — Delete a scheduled job assignment
- `rippling-pp-cli scheduled-job-assignments get` — Retrieve a specific scheduled job assignment
- `rippling-pp-cli scheduled-job-assignments list` — A list of scheduled job assignments. - Requires: `API Tier 2` - Sortable fields: `id`, `created_at`, `updated_at`,...

**schedules** — Schedules used by the company

- `rippling-pp-cli schedules create` — Create a new schedule
- `rippling-pp-cli schedules get` — Retrieve a specific schedule
- `rippling-pp-cli schedules list` — A list of schedules. - Requires: `API Tier 2` - Expandable fields: `managers`, `observers`, `members` - Sortable...
- `rippling-pp-cli schedules update` — Update a specific schedule.

**shift-inputs** — Shift inputs used by the company

- `rippling-pp-cli shift-inputs create` — Create a new shift input
- `rippling-pp-cli shift-inputs delete` — Delete a shift input
- `rippling-pp-cli shift-inputs get` — Retrieve a specific shift input
- `rippling-pp-cli shift-inputs list` — A list of shift inputs. - Requires: `API Tier 2` - Filterable fields: `name` - Expandable fields: `creator` -...
- `rippling-pp-cli shift-inputs update` — Update a specific shift input.

**shiftassignments** — Manage shiftassignments

- `rippling-pp-cli shiftassignments create` — Create a new shift assignment
- `rippling-pp-cli shiftassignments get` — Retrieve a specific shift assignment
- `rippling-pp-cli shiftassignments list` — A list of shift assignments. - Requires: `API Tier 2` - Filterable fields: `worker_id`, `schedule_id`,...
- `rippling-pp-cli shiftassignments update` — Update a specific shift assignment.

**software-deployments** — Software deployed to company devices

- `rippling-pp-cli software-deployments create` — Create a new software deployment
- `rippling-pp-cli software-deployments delete` — Delete a software deployment
- `rippling-pp-cli software-deployments get` — Retrieve a specific software deployment
- `rippling-pp-cli software-deployments list` — A list of software deployments. - Requires: `API Tier 2` - Filterable fields: `software_id`, `active_group_type` -...
- `rippling-pp-cli software-deployments update` — Update a specific software deployment.

**sso-me** — Manage sso me

- `rippling-pp-cli sso-me` — SSO information of the current user - Requires: `API Tier 1` - Expandable fields: `company`

**supergroups** — Supergroups used by the company

- `rippling-pp-cli supergroups get` — Retrieve a specific supergroup.
- `rippling-pp-cli supergroups list` — Retrieve supergroups matching the input parameters. - Requires: `API Tier 1` - Filterable fields: `app_owner_id`,...

**teams** — Teams at the company

- `rippling-pp-cli teams get` — Retrieve a specific team
- `rippling-pp-cli teams list` — A list of teams. - Requires: `API Tier 1` - Expandable fields: `parent` - Sortable fields: `id`, `created_at`,...

**time-cards** — Manage time cards

- `rippling-pp-cli time-cards get` — Retrieve a specific time card
- `rippling-pp-cli time-cards list` — A list of time cards. - Requires: `API Tier 2` - Filterable fields: `pay_period.start_date`, `worker_id` -...

**time-entries** — Time entries submitted by workers

- `rippling-pp-cli time-entries create` — Create a new time entry
- `rippling-pp-cli time-entries delete` — Delete a time entry
- `rippling-pp-cli time-entries get` — Retrieve a specific time entry
- `rippling-pp-cli time-entries list` — A list of time entries. - Requires: `API Tier 2` - Filterable fields: `worker_id`, `start_time`,...
- `rippling-pp-cli time-entries update` — Update a specific time entry.

**titles** — Job titles used by the company

- `rippling-pp-cli titles create` — Create a new title
- `rippling-pp-cli titles delete` — Delete a title
- `rippling-pp-cli titles get` — Retrieve a specific title
- `rippling-pp-cli titles list` — A list of titles. - Requires: `API Tier 1` - Sortable fields: `id`, `created_at`, `updated_at`
- `rippling-pp-cli titles update` — Update a specific title.

**tracks** — Manage tracks

- `rippling-pp-cli tracks get` — Retrieve a specific track
- `rippling-pp-cli tracks list` — A list of tracks. - Requires: `API Tier 2` - Sortable fields: `id`, `created_at`, `updated_at`

**unassignedshifts** — Manage unassignedshifts

- `rippling-pp-cli unassignedshifts create` — Create a new unassigned shift
- `rippling-pp-cli unassignedshifts get` — Retrieve a specific unassigned shift
- `rippling-pp-cli unassignedshifts list` — A list of unassigned shifts. - Requires: `API Tier 2` - Filterable fields: `schedule_id`, `shift_data.start_time`,...
- `rippling-pp-cli unassignedshifts update` — Update a specific unassigned shift.

**users** — Users of the company

- `rippling-pp-cli users get` — Retrieve a specific user
- `rippling-pp-cli users list` — A list of users. - Requires: `API Tier 1` - Sortable fields: `id`, `created_at`, `updated_at`

**work-locations** — Work locations used by the company

- `rippling-pp-cli work-locations create` — Create a new work location
- `rippling-pp-cli work-locations delete` — Delete a work location
- `rippling-pp-cli work-locations get` — Retrieve a specific work location
- `rippling-pp-cli work-locations list` — A list of work locations. - Requires: `API Tier 1` - Sortable fields: `id`, `created_at`, `updated_at`
- `rippling-pp-cli work-locations update` — Update a specific work location.

**worker-time-splits** — Manage worker time splits

- `rippling-pp-cli worker-time-splits create` — Create a new worker time split
- `rippling-pp-cli worker-time-splits delete` — Delete a worker time split
- `rippling-pp-cli worker-time-splits get` — Retrieve a specific worker time split
- `rippling-pp-cli worker-time-splits list` — A list of worker time splits. - Requires: `API Tier 2` - Filterable fields: `worker_id`, `is_enabled`, `updated_at`...
- `rippling-pp-cli worker-time-splits update` — Update a specific worker time split.

**workers** — Workers who work or have worked at the company

- `rippling-pp-cli workers get` — Retrieve a specific worker
- `rippling-pp-cli workers list` — A list of workers. - Requires: `API Tier 1` - Filterable fields: `status`, `work_email`, `user_id`, `created_at`,...

**workflow-action-executions** — Manage workflow action executions

- `rippling-pp-cli workflow-action-executions` — Execute a workflow action


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
rippling-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Auth Setup

Create or obtain a Rippling Platform OAuth2 production bearer token, then expose it through the environment:

```bash
export RIPPLING_PLATFORM_OAUTH2_PRODUCTION="your-token-here"
```

For scripts and agents, environment-variable auth is preferred. Browser OAuth login is also available when you have a Rippling OAuth client ID and client secret:

```bash
rippling-pp-cli auth login --client-id "$RIPPLING_CLIENT_ID" --client-secret "$RIPPLING_CLIENT_SECRET"
```

Run `rippling-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  rippling-pp-cli break-types list --agent --select id,name,status
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

Parse `.results` for data and `.meta.source` to know whether it's live or local. A human-readable `N results (live)` summary is printed to stderr only when stdout is a terminal — piped/agent consumers get pure JSON on stdout.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
rippling-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
rippling-pp-cli feedback --stdin < notes.txt
rippling-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.rippling-pp-cli/feedback.jsonl`. They are never POSTed unless `RIPPLING_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `RIPPLING_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
rippling-pp-cli profile save briefing --json
rippling-pp-cli --profile briefing break-types list
rippling-pp-cli profile list --json
rippling-pp-cli profile show briefing
rippling-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `rippling-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/other/rippling/cmd/rippling-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add rippling-pp-mcp -- rippling-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which rippling-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   rippling-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `rippling-pp-cli <command> --help`.
