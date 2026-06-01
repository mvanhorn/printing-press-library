# Rippling CLI

Documentation for the Rippling Platform API.

## Install

The recommended path installs both the `rippling-pp-cli` binary and the `pp-rippling` agent skill in one shot:

```bash
npx -y @mvanhorn/printing-press install rippling
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install rippling --cli-only
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.23+):

```bash
go install github.com/mvanhorn/printing-press-library/library/other/rippling/cmd/rippling-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/rippling-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-rippling --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-rippling --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-rippling skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-rippling. The skill defines how its required CLI can be installed.
```

## Quick Start

### 1. Install

See [Install](#install) above.

### 2. Set Up Credentials

Create or obtain a Rippling Platform OAuth2 production bearer token in your Rippling developer/API settings, then expose it to the CLI via environment variable:

```bash
export RIPPLING_PLATFORM_OAUTH2_PRODUCTION="your-token-here"
```

For scripts and agents, environment-variable auth is preferred. Browser OAuth login is also available when you have a Rippling OAuth client ID and client secret:

```bash
rippling-pp-cli auth login --client-id "$RIPPLING_CLIENT_ID" --client-secret "$RIPPLING_CLIENT_SECRET"
```

### 3. Verify Setup

```bash
rippling-pp-cli doctor
```

This checks your configuration and credentials.

### 4. Try Your First Command

```bash
rippling-pp-cli break-types list
```

## IT workflows

The `it` namespace groups Tier-1, public-API-backed IT support in one place:

```bash
rippling-pp-cli it overview --json
rippling-pp-cli it onboarding-plan --json
rippling-pp-cli it deploy-software --json
rippling-pp-cli it inventory-schema --json
```

These commands cover new-hire draft setup planning, software deployment workflows, and report-driven hardware inventory schemas. `it overview` also documents public API gaps for laptop/device ordering, shipping, retrieval, remote lock, and wipe; those operations are not exposed by the public REST-beta spec used to print this CLI.

## Usage

Run `rippling-pp-cli --help` for the full command reference and flag list.

## Commands

### break-types

Break types configured by the company for time tracking

- **`rippling-pp-cli break-types get`** - Retrieve a specific break type
- **`rippling-pp-cli break-types list`** - A list of break types.
 - Requires: `API Tier 2`
 - Sortable fields: `id`, `created_at`, `updated_at`

### business-partner-groups

Manage business partner groups

- **`rippling-pp-cli business-partner-groups create`** - Create a new business partner group
- **`rippling-pp-cli business-partner-groups delete`** - Delete a business partner group
- **`rippling-pp-cli business-partner-groups get`** - Retrieve a specific business partner group
- **`rippling-pp-cli business-partner-groups list`** - A list of business partner groups.
 - Requires: `API Tier 1`
 - Expandable fields: `default_business_partner`
 - Sortable fields: `id`, `created_at`, `updated_at`

### business-partners

Business partners of the company

- **`rippling-pp-cli business-partners create`** - Create a new business partner
- **`rippling-pp-cli business-partners delete`** - Delete a business partner
- **`rippling-pp-cli business-partners get`** - Retrieve a specific business partner
- **`rippling-pp-cli business-partners list`** - A list of business partners.
 - Requires: `API Tier 1`
 - Filterable fields: `worker_id`, `business_partner_group_id`
 - Expandable fields: `business_partner_group`, `worker`, `client_group`
 - Sortable fields: `id`, `created_at`, `updated_at`

### candidate-applications

An application by a candidate to a specific job requisition

- **`rippling-pp-cli candidate-applications list`** - A list of candidate applications.
 - Requires: `API Tier 2`
 - Expandable fields: `job`
 - Sortable fields: `id`, `created_at`, `updated_at`

### candidates

Someone who applies to a job requisition opened by the company

- **`rippling-pp-cli candidates list`** - A list of candidates.
 - Requires: `API Tier 2`
 - Sortable fields: `id`, `created_at`, `updated_at`

### companies

Companies on Rippling

- **`rippling-pp-cli companies list`** - A list of companies.
 - Requires: `API Tier 1`
 - Expandable fields: `parent_legal_entity`, `legal_entities`
 - Sortable fields: `id`, `created_at`, `updated_at`

### company-legal-entities

Manage company legal entities

- **`rippling-pp-cli company-legal-entities get`** - Retrieve a specific company legal entity
- **`rippling-pp-cli company-legal-entities list`** - A list of company legal entities.
 - Requires: `API Tier 2`
 - Filterable fields: `country_code`
 - Sortable fields: `id`, `created_at`, `updated_at`

### company-legal-entity-workers

Manage company legal entity workers

- **`rippling-pp-cli company-legal-entity-workers get`** - Retrieve a specific company-legal-entity-worker
- **`rippling-pp-cli company-legal-entity-workers list`** - A list of company-legal-entity-workers.
 - Requires: `API Tier 2`
 - Filterable fields: `status`, `company_entity_id`
 - Sortable fields: `id`, `created_at`, `updated_at`

### compensation-bands-details

Compensation bands details associated with workers

- **`rippling-pp-cli compensation-bands-details get`** - Retrieve a specific compensation bands detail
- **`rippling-pp-cli compensation-bands-details list`** - A list of compensation bands details.
 - Requires: `API Tier 2`
 - Filterable fields: `created_at`, `updated_at`, `internal_job_code`, `worker_id`
 - Expandable fields: `worker`
 - Sortable fields: `id`, `created_at`, `updated_at`

### compensations

Compensation associated with workers

- **`rippling-pp-cli compensations get`** - Retrieves the Compensation for the Worker with the ID provided in the URL path.
- **`rippling-pp-cli compensations list`** - A list of compensations.
 - Requires: `API Tier 2`
 - Expandable fields: `worker`
 - Sortable fields: `id`, `created_at`, `updated_at`

### custom-apps

Manage custom apps

- **`rippling-pp-cli custom-apps create`** - Create a new CustomApp
- **`rippling-pp-cli custom-apps delete`** - Delete a CustomApp
- **`rippling-pp-cli custom-apps get`** - Retrieve a specific CustomApp
- **`rippling-pp-cli custom-apps list`** - A list of CustomApp.
 - Requires: `Custom Apps Platform API`
- **`rippling-pp-cli custom-apps update`** - Update a specific CustomApp.

### custom-fields

Custom fields defined by the company

- **`rippling-pp-cli custom-fields list`** - A list of custom fields.
 - Requires: `API Tier 1`
 - Sortable fields: `id`, `created_at`, `updated_at`

### custom-objects

Custom objects defined by the company

- **`rippling-pp-cli custom-objects create`** - Create a new custom object.
- **`rippling-pp-cli custom-objects delete`** - Delete a custom object.
- **`rippling-pp-cli custom-objects get`** - Retrieve a specific custom object.
- **`rippling-pp-cli custom-objects list`** - A list of custom objects.
 - Requires: `API Tier 1`
- **`rippling-pp-cli custom-objects update`** - Update a specific custom object.

### custom-pages

Manage custom pages

- **`rippling-pp-cli custom-pages create`** - Create a new CustomPage
- **`rippling-pp-cli custom-pages delete`** - Delete a CustomPage
- **`rippling-pp-cli custom-pages get`** - Retrieve a specific CustomPage
- **`rippling-pp-cli custom-pages list`** - A list of CustomPage.
 - Requires: `Custom Apps Platform API`
- **`rippling-pp-cli custom-pages update`** - Update a specific CustomPage.

### custom-settings

Manage custom settings

- **`rippling-pp-cli custom-settings create`** - Create a new Setting
- **`rippling-pp-cli custom-settings delete`** - Delete a Setting
- **`rippling-pp-cli custom-settings get`** - Retrieve a specific Setting
- **`rippling-pp-cli custom-settings list`** - A list of Setting.
 - Requires: `Functions API`
 - Sortable fields: `id`, `created_at`, `updated_at`
- **`rippling-pp-cli custom-settings update`** - Update a specific Setting.

### departments

Departments used by the company

- **`rippling-pp-cli departments create`** - Create a new department
- **`rippling-pp-cli departments get`** - Retrieve a specific department
- **`rippling-pp-cli departments list`** - A list of departments.
 - Requires: `API Tier 1`
 - Expandable fields: `parent`, `department_hierarchy`
 - Sortable fields: `id`, `created_at`, `updated_at`
- **`rippling-pp-cli departments update`** - Update a specific department.

### draft-hires

Candidates who have not yet started work at the company

- **`rippling-pp-cli draft-hires create`** - Create bulk draft hire

### earning-types

Earning types configured for the company

- **`rippling-pp-cli earning-types list`** - A list of earning-types.
 - Requires: `Global Payroll`

### earnings-inputs

External earnings inputs for the company

- **`rippling-pp-cli earnings-inputs create`** - Create a new earnings-input
- **`rippling-pp-cli earnings-inputs delete`** - Delete a earnings-input
- **`rippling-pp-cli earnings-inputs list`** - A list of earnings-inputs.
 - Requires: `Global Payroll`

### employment-types

Employment types used by the company

- **`rippling-pp-cli employment-types get`** - Retrieve a specific employment type
- **`rippling-pp-cli employment-types list`** - A list of employment types.
 - Requires: `API Tier 1`
 - Sortable fields: `id`, `created_at`, `updated_at`

### entitlements

Availability of API features to the company or Partners.

- **`rippling-pp-cli entitlements list`** - A list of entitlements.
 - Requires: `API Tier 1`

### files

Manage files

- **`rippling-pp-cli files create`** - Create a new file
- **`rippling-pp-cli files get`** - Retrieve a specific file

### functions

Functions defined by the company

- **`rippling-pp-cli functions create`** - Create a new function.
- **`rippling-pp-cli functions get`** - Retrieve a specific function.
- **`rippling-pp-cli functions list`** - A list of functions.
 - Requires: `API Tier 2`
 - Sortable fields: `id`, `created_at`, `updated_at`
- **`rippling-pp-cli functions update`** - Update a function

### headcount-positions

Headcount allocations for the company

- **`rippling-pp-cli headcount-positions get`** - Retrieve a specific headcount position
- **`rippling-pp-cli headcount-positions list`** - A list of headcount positions.
 - Requires: `API Tier 2`
 - Filterable fields: `created_at`, `position_id`, `position_type`, `position_sub_type`, `department_id`, `work_location_id`, `level_id`, `teams_id`, `job_requisition_id`, `recruiter_id`, `headcount_owner_id`, `worker_id`, `backfill_for_id`, `employment_type_id`, `in_budget`, `target_start_date`, `current_start_date`, `job_function_id`, `priority_id`
 - Expandable fields: `department`, `work_location`, `level`, `teams`, `job_requisition`, `recruiter`, `headcount_owner`, `worker`, `backfill_for`, `employment_type`, `priority`, `job_function`, `location_factor`
 - Sortable fields: `id`, `created_at`, `updated_at`, `position_id`, `position_type`, `position_sub_type`, `department_id`, `work_location_id`, `level_id`, `job_requisition_id`, `recruiter_id`, `headcount_owner_id`, `worker_id`, `backfill_for_id`, `employment_type_id`, `in_budget`, `target_start_date`, `current_start_date`, `priority_id`, `job_function_id`

### headcount-priorities

Manage headcount priorities

- **`rippling-pp-cli headcount-priorities get`** - Retrieve a specific headcount priority
- **`rippling-pp-cli headcount-priorities list`** - A list of headcount priorities.
 - Requires: `API Tier 2`
 - Sortable fields: `id`, `created_at`, `updated_at`

### job-assignments

Manage job assignments

- **`rippling-pp-cli job-assignments create`** - Create a new job assignment
- **`rippling-pp-cli job-assignments delete`** - Delete a job assignment
- **`rippling-pp-cli job-assignments get`** - Retrieve a specific job assignment
- **`rippling-pp-cli job-assignments list`** - A list of job assignments.
 - Requires: `API Tier 2`
 - Filterable fields: `worker_id`, `job_code_id`
 - Expandable fields: `worker`, `job_code`, `job_pay_rate_exceptions`
 - Sortable fields: `id`, `created_at`, `updated_at`
- **`rippling-pp-cli job-assignments update`** - Update a specific job assignment.

### job-codes

Manage job codes

- **`rippling-pp-cli job-codes create`** - Create a new job code
- **`rippling-pp-cli job-codes delete`** - Delete a job code
- **`rippling-pp-cli job-codes get`** - Retrieve a specific job code
- **`rippling-pp-cli job-codes list`** - A list of job codes.
 - Requires: `API Tier 2`
 - Filterable fields: `job_dimension_id`, `group_id`
 - Expandable fields: `job_dimension`, `work_location`, `department`, `pay_rate_exceptions`
 - Sortable fields: `id`, `created_at`, `updated_at`
- **`rippling-pp-cli job-codes update`** - Update a specific job code.

### job-dimensions

Manage job dimensions

- **`rippling-pp-cli job-dimensions create`** - Create a new job dimension
- **`rippling-pp-cli job-dimensions delete`** - Delete a job dimension
- **`rippling-pp-cli job-dimensions get`** - Retrieve a specific job dimension
- **`rippling-pp-cli job-dimensions list`** - A list of job dimensions.
 - Requires: `API Tier 2`
 - Filterable fields: `name`
 - Sortable fields: `id`, `created_at`, `updated_at`
- **`rippling-pp-cli job-dimensions update`** - Update a specific job dimension.

### job-functions

Organizational job categories that group similar roles and responsibilities within a company

- **`rippling-pp-cli job-functions get`** - Retrieve a specific job function
- **`rippling-pp-cli job-functions list`** - A list of job functions.
 - Requires: `API Tier 1`
 - Sortable fields: `id`, `created_at`, `updated_at`

### job-pay-rate-exceptions

Manage job pay rate exceptions

- **`rippling-pp-cli job-pay-rate-exceptions get`** - Retrieve a specific job pay rate exception
- **`rippling-pp-cli job-pay-rate-exceptions list`** - A list of job pay rate exceptions.
 - Requires: `API Tier 2`
 - Filterable fields: `job_code_id`
 - Expandable fields: `job_code`
 - Sortable fields: `id`, `created_at`, `updated_at`

### job-requisitions

A request for a job to be filled by a candidate

- **`rippling-pp-cli job-requisitions list`** - A list of job requisitions.
 - Requires: `API Tier 2`
 - Sortable fields: `id`, `created_at`, `updated_at`

### job-requisitions-write

Manage job requisitions write

- **`rippling-pp-cli job-requisitions-write create`** - Create a new job requisition

### kiosk-badges

Badge information used with Timeclock Kiosk

- **`rippling-pp-cli kiosk-badges create`** - Create a new kiosk badge
- **`rippling-pp-cli kiosk-badges delete`** - Delete a kiosk badge
- **`rippling-pp-cli kiosk-badges get`** - Retrieve a specific kiosk badge
- **`rippling-pp-cli kiosk-badges list`** - A list of kiosk badges.
 - Requires: `API Tier 2`
 - Filterable fields: `badge_id`
 - Expandable fields: `worker`
 - Sortable fields: `id`, `created_at`, `updated_at`
- **`rippling-pp-cli kiosk-badges update`** - Update a specific kiosk badge.

### leave-accruals

Leave accruals for workers

- **`rippling-pp-cli leave-accruals create`** - Create a new leave accrual
- **`rippling-pp-cli leave-accruals get`** - Retrieve a specific leave accrual
- **`rippling-pp-cli leave-accruals list`** - A list of leave accruals.
 - Requires: `API Tier 2`
 - Filterable fields: `worker_id`, `leave_type_id`, `accrual_date`
 - Expandable fields: `worker`, `grantor`
 - Sortable fields: `id`, `created_at`, `updated_at`, `accrual_date`, `expiration_date`

### leave-balances

Leave balances for workers

- **`rippling-pp-cli leave-balances get`** - Retrieve a specific leave balance
- **`rippling-pp-cli leave-balances list`** - A list of leave balances.
 - Requires: `API Tier 2`
 - Filterable fields: `worker_id`, `leave_type_id`
 - Expandable fields: `worker`, `leave_type`
 - Sortable fields: `id`, `created_at`, `updated_at`

### leave-requests

Leave requests submitted by workers

- **`rippling-pp-cli leave-requests create`** - Create a new leave request
- **`rippling-pp-cli leave-requests get`** - Retrieve a specific leave request
- **`rippling-pp-cli leave-requests list`** - A list of leave requests.
 - Requires: `API Tier 2`
 - Filterable fields: `worker_id`, `requester_id`, `reviewer_id`, `status`, `leave_policy_id`, `leave_type_id`, `start_date`, `end_date`
 - Expandable fields: `worker`, `requester`, `leave_type`, `reviewer`
 - Sortable fields: `id`, `created_at`, `updated_at`
- **`rippling-pp-cli leave-requests update`** - Update a specific leave request.

### leave-types

Leave types used by the company

- **`rippling-pp-cli leave-types get`** - Retrieve a specific leave type
- **`rippling-pp-cli leave-types list`** - A list of leave types.
 - Requires: `API Tier 2`
 - Filterable fields: `name`
 - Sortable fields: `id`, `created_at`, `updated_at`

### legal-entities

Legal entities registered by the company

- **`rippling-pp-cli legal-entities get`** - Retrieve a specific legal entity
- **`rippling-pp-cli legal-entities list`** - A list of legal entities.
 - Requires: `API Tier 2`
 - Expandable fields: `parent`, `company`
 - Sortable fields: `id`, `created_at`, `updated_at`

### levels

Manage levels

- **`rippling-pp-cli levels get`** - Retrieve a specific level
- **`rippling-pp-cli levels list`** - A list of levels.
 - Requires: `API Tier 2`
 - Expandable fields: `parent`, `track`
 - Sortable fields: `id`, `created_at`, `updated_at`

### location-factors

Geographic compensation adjustment factors that modify base compensation based on location-specific market conditions and cost of living

- **`rippling-pp-cli location-factors get`** - Retrieve a specific location factor
- **`rippling-pp-cli location-factors list`** - A list of location factors.
 - Requires: `API Tier 2`
 - Sortable fields: `id`, `created_at`, `updated_at`

### metadata

Manage metadata

- **`rippling-pp-cli metadata list`** - A list of metadata components.
 - Requires: `Metadata Platform API`
- **`rippling-pp-cli metadata list-components`** - A list of metadata components.
 - Requires: `Metadata Platform API`
- **`rippling-pp-cli metadata list-summary`** - A list of metadata summary.
 - Requires: `Metadata Platform API`
- **`rippling-pp-cli metadata list-types`** - A list of metadata types.
 - Requires: `Metadata Platform API`

### object-categories

Object Categories defined by the company

- **`rippling-pp-cli object-categories create`** - Create a new object category.
- **`rippling-pp-cli object-categories delete`** - Delete an object category.
- **`rippling-pp-cli object-categories get`** - Retrieve a specific object category.
- **`rippling-pp-cli object-categories list`** - A list of object categories.
 - Requires: `API Tier 1`
- **`rippling-pp-cli object-categories update`** - Update a specific object category.

### payroll-runs

Payroll runs for the company

- **`rippling-pp-cli payroll-runs get`** - Retrieve a specific payroll-run
- **`rippling-pp-cli payroll-runs list`** - A list of payroll-runs.
 - Requires: `Global Payroll`
 - Sortable fields: `check_date`

### platform-capabilities

Manage platform capabilities

- **`rippling-pp-cli platform-capabilities list`** - A list of platform capabilities.
 - Requires: `Platform Capabilities API`

### report-runs

Manage report runs

- **`rippling-pp-cli report-runs get`** - Retrieve the status and data for a specific report run
- **`rippling-pp-cli report-runs trigger`** - Trigger a new report run

### reports

Report data for company

- **`rippling-pp-cli reports get`** - Retrieve a specific report by its ID

### scheduled-job-assignments

Manage scheduled job assignments

- **`rippling-pp-cli scheduled-job-assignments delete`** - Delete a scheduled job assignment
- **`rippling-pp-cli scheduled-job-assignments get`** - Retrieve a specific scheduled job assignment
- **`rippling-pp-cli scheduled-job-assignments list`** - A list of scheduled job assignments.
 - Requires: `API Tier 2`
 - Sortable fields: `id`, `created_at`, `updated_at`, `effective_from`

### schedules

Schedules used by the company

- **`rippling-pp-cli schedules create`** - Create a new schedule
- **`rippling-pp-cli schedules get`** - Retrieve a specific schedule
- **`rippling-pp-cli schedules list`** - A list of schedules.
 - Requires: `API Tier 2`
 - Expandable fields: `managers`, `observers`, `members`
 - Sortable fields: `id`, `created_at`, `updated_at`
- **`rippling-pp-cli schedules update`** - Update a specific schedule.

### shift-inputs

Shift inputs used by the company

- **`rippling-pp-cli shift-inputs create`** - Create a new shift input
- **`rippling-pp-cli shift-inputs delete`** - Delete a shift input
- **`rippling-pp-cli shift-inputs get`** - Retrieve a specific shift input
- **`rippling-pp-cli shift-inputs list`** - A list of shift inputs.
 - Requires: `API Tier 2`
 - Filterable fields: `name`
 - Expandable fields: `creator`
 - Sortable fields: `id`, `created_at`, `updated_at`
- **`rippling-pp-cli shift-inputs update`** - Update a specific shift input.

### shiftassignments

Manage shiftassignments

- **`rippling-pp-cli shiftassignments create`** - Create a new shift assignment
- **`rippling-pp-cli shiftassignments get`** - Retrieve a specific shift assignment
- **`rippling-pp-cli shiftassignments list`** - A list of shift assignments.
 - Requires: `API Tier 2`
 - Filterable fields: `worker_id`, `schedule_id`, `shift_data.start_time`, `shift_data.end_time`
 - Expandable fields: `worker`, `schedule`
 - Sortable fields: `id`, `created_at`, `updated_at`
- **`rippling-pp-cli shiftassignments update`** - Update a specific shift assignment.

### software-deployments

Software deployed to company devices

- **`rippling-pp-cli software-deployments create`** - Create a new software deployment
- **`rippling-pp-cli software-deployments delete`** - Delete a software deployment
- **`rippling-pp-cli software-deployments get`** - Retrieve a specific software deployment
- **`rippling-pp-cli software-deployments list`** - A list of software deployments.
 - Requires: `API Tier 2`
 - Filterable fields: `software_id`, `active_group_type`
 - Expandable fields: `software`
 - Sortable fields: `id`, `created_at`, `updated_at`
- **`rippling-pp-cli software-deployments update`** - Update a specific software deployment.

### sso-me

Manage sso me

- **`rippling-pp-cli sso-me list`** - SSO information of the current user
 - Requires: `API Tier 1`
 - Expandable fields: `company`

### supergroups

Supergroups used by the company

- **`rippling-pp-cli supergroups get`** - Retrieve a specific supergroup.
- **`rippling-pp-cli supergroups list`** - Retrieve supergroups matching the input parameters.
 - Requires: `API Tier 1`
 - Filterable fields: `app_owner_id`, `group_type`
 - Sortable fields: `id`, `created_at`, `updated_at`

### teams

Teams at the company

- **`rippling-pp-cli teams get`** - Retrieve a specific team
- **`rippling-pp-cli teams list`** - A list of teams.
 - Requires: `API Tier 1`
 - Expandable fields: `parent`
 - Sortable fields: `id`, `created_at`, `updated_at`

### time-cards

Manage time cards

- **`rippling-pp-cli time-cards get`** - Retrieve a specific time card
- **`rippling-pp-cli time-cards list`** - A list of time cards.
 - Requires: `API Tier 2`
 - Filterable fields: `pay_period.start_date`, `worker_id`
 - Expandable fields: `worker`
 - Sortable fields: `id`, `created_at`, `updated_at`

### time-entries

Time entries submitted by workers

- **`rippling-pp-cli time-entries create`** - Create a new time entry
- **`rippling-pp-cli time-entries delete`** - Delete a time entry
- **`rippling-pp-cli time-entries get`** - Retrieve a specific time entry
- **`rippling-pp-cli time-entries list`** - A list of time entries.
 - Requires: `API Tier 2`
 - Filterable fields: `worker_id`, `start_time`, `pay_period.start_date`, `updated_at`
 - Expandable fields: `worker`, `time_card`
 - Sortable fields: `id`, `created_at`, `updated_at`
- **`rippling-pp-cli time-entries update`** - Update a specific time entry.

### titles

Job titles used by the company

- **`rippling-pp-cli titles create`** - Create a new title
- **`rippling-pp-cli titles delete`** - Delete a title
- **`rippling-pp-cli titles get`** - Retrieve a specific title
- **`rippling-pp-cli titles list`** - A list of titles.
 - Requires: `API Tier 1`
 - Sortable fields: `id`, `created_at`, `updated_at`
- **`rippling-pp-cli titles update`** - Update a specific title.

### tracks

Manage tracks

- **`rippling-pp-cli tracks get`** - Retrieve a specific track
- **`rippling-pp-cli tracks list`** - A list of tracks.
 - Requires: `API Tier 2`
 - Sortable fields: `id`, `created_at`, `updated_at`

### unassignedshifts

Manage unassignedshifts

- **`rippling-pp-cli unassignedshifts create`** - Create a new unassigned shift
- **`rippling-pp-cli unassignedshifts get`** - Retrieve a specific unassigned shift
- **`rippling-pp-cli unassignedshifts list`** - A list of unassigned shifts.
 - Requires: `API Tier 2`
 - Filterable fields: `schedule_id`, `shift_data.start_time`, `shift_data.end_time`
 - Expandable fields: `schedule`
 - Sortable fields: `id`, `created_at`, `updated_at`
- **`rippling-pp-cli unassignedshifts update`** - Update a specific unassigned shift.

### users

Users of the company

- **`rippling-pp-cli users get`** - Retrieve a specific user
- **`rippling-pp-cli users list`** - A list of users.
 - Requires: `API Tier 1`
 - Sortable fields: `id`, `created_at`, `updated_at`

### work-locations

Work locations used by the company

- **`rippling-pp-cli work-locations create`** - Create a new work location
- **`rippling-pp-cli work-locations delete`** - Delete a work location
- **`rippling-pp-cli work-locations get`** - Retrieve a specific work location
- **`rippling-pp-cli work-locations list`** - A list of work locations.
 - Requires: `API Tier 1`
 - Sortable fields: `id`, `created_at`, `updated_at`
- **`rippling-pp-cli work-locations update`** - Update a specific work location.

### worker-time-splits

Manage worker time splits

- **`rippling-pp-cli worker-time-splits create`** - Create a new worker time split
- **`rippling-pp-cli worker-time-splits delete`** - Delete a worker time split
- **`rippling-pp-cli worker-time-splits get`** - Retrieve a specific worker time split
- **`rippling-pp-cli worker-time-splits list`** - A list of worker time splits.
 - Requires: `API Tier 2`
 - Filterable fields: `worker_id`, `is_enabled`, `updated_at`
 - Expandable fields: `worker`, `job_codes`
 - Sortable fields: `id`, `created_at`, `updated_at`
- **`rippling-pp-cli worker-time-splits update`** - Update a specific worker time split.

### workers

Workers who work or have worked at the company

- **`rippling-pp-cli workers get`** - Retrieve a specific worker
- **`rippling-pp-cli workers list`** - A list of workers.
 - Requires: `API Tier 1`
 - Filterable fields: `status`, `work_email`, `user_id`, `created_at`, `updated_at`
 - Expandable fields: `user`, `manager`, `legal_entity`, `employment_type`, `compensation`, `department`, `teams`, `level`, `custom_fields`, `business_partners`
 - Sortable fields: `id`, `created_at`, `updated_at`

### workflow-action-executions

Manage workflow action executions

- **`rippling-pp-cli workflow-action-executions workflow_action_executions`** - Execute a workflow action


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
rippling-pp-cli break-types list

# JSON for scripting and agents
rippling-pp-cli break-types list --json

# Filter to specific fields
rippling-pp-cli break-types list --json --select id,name,status

# Dry run — show the request without sending
rippling-pp-cli break-types list --dry-run

# Agent mode — JSON + compact + no prompts in one flag
rippling-pp-cli break-types list --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Explicit retries** - add `--idempotent` to create retries and `--ignore-missing` to delete retries when a no-op success is acceptable
- **Confirmable** - `--yes` for explicit confirmation of destructive actions
- **Piped input** - write commands can accept structured input when their help lists `--stdin`
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Use with Claude Code

Install the focused skill — it auto-installs the CLI on first invocation:

```bash
npx skills add mvanhorn/printing-press-library/cli-skills/pp-rippling -g
```

Then invoke `/pp-rippling <query>` in Claude Code. The skill is the most efficient path — Claude Code drives the CLI directly without an MCP server in the middle.

<details>
<summary>Use as an MCP server in Claude Code (advanced)</summary>

If you'd rather register this CLI as an MCP server in Claude Code, install the MCP binary first:

```bash
go install github.com/mvanhorn/printing-press-library/library/other/rippling/cmd/rippling-pp-mcp@latest
```

Then register it:

```bash
claude mcp add rippling rippling-pp-mcp -e RIPPLING_PLATFORM_OAUTH2_PRODUCTION=<your-token>
```

</details>

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/rippling-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `RIPPLING_PLATFORM_OAUTH2_PRODUCTION` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.

```bash
go install github.com/mvanhorn/printing-press-library/library/other/rippling/cmd/rippling-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "rippling": {
      "command": "rippling-pp-mcp",
      "env": {
        "RIPPLING_PLATFORM_OAUTH2_PRODUCTION": "<your-key>"
      }
    }
  }
}
```

</details>

## Health Check

```bash
rippling-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/rippling-platform-pp-cli/config.toml`

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `RIPPLING_PLATFORM_OAUTH2_PRODUCTION` | per_call | Yes | Set to your API credential. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `rippling-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $RIPPLING_PLATFORM_OAUTH2_PRODUCTION`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
