# Fec CLI

This application programming interface (API) allows you to explore the way candidates and committees fund their campaigns. 

 The Federal Election Commission (FEC) API is a RESTful web service supporting full-text and field-specific searches on FEC data. [Bulk downloads](https://www.fec.gov/data/advanced/?tab=bulk-data) are available on the current site. Information is tied to the underlying forms by file ID and image ID. Data are updated nightly. 

 There are a lot of data, and a good place to start is to use search to find interesting candidates and committees. Then, you can use their IDs to find report or line item details with the other endpoints. If you are interested in individual donors, check out contributor information in the `/schedule_a/` endpoints. 

 <b class="body" id="getting_started_head">Getting started with the openFEC API</b><br> 

 If you would like to use the FEC's API programmatically, you can sign up for your own API key using our form. Alternatively, you can still try out our API without an API key by using the web interface and using DEMO_KEY. Note that when you use the openFEC API you are subject to the [Terms of Service](https://github.com/fecgov/FEC/blob/master/TERMS-OF-SERVICE.md) and [Acceptable Use policy](https://github.com/fecgov/FEC/blob/master/ACCEPTABLE-USE-POLICY.md). 

 Signing up for an API key will enable you to place up to 1,000 calls an hour. Each call is limited to 100 results per page. You can email questions, comments or a request to get a key for 7,200 calls an hour (120 calls per minute) to <a href="mailto:APIinfo@fec.gov">APIinfo@fec.gov</a>. You can also ask questions and discuss the data in a community led [group](https://groups.google.com/forum/#!forum/fec-data). 

 The model definitions and schema are available at [/swagger](/swagger/). This is useful for making wrappers and exploring the data. 

 A few restrictions limit the way you can use FEC data. For example, you can’t use contributor lists for commercial purposes or to solicit donations. [Learn more here](https://www.fec.gov/updates/sale-or-use-contributor-information/). 

 [Inspect our source code](https://github.com/fecgov/openFEC). We welcome issues and pull requests! 

 <p><br></p> <h2 class="title" id="signup_head">Sign up for an API key</h2> <div id="apidatagov_signup">Loading signup form...</div>

Created by [@mvanhorn](https://github.com/mvanhorn) (Hunter Veltri).

## Install

The recommended path installs both the `fec-pp-cli` binary and the `pp-fec` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install fec
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install fec --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install fec --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install fec --agent claude-code
npx -y @mvanhorn/printing-press-library install fec --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.6 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/other/fec/cmd/fec-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/fec-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install fec --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-fec --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-fec --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install fec --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/fec-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `FEC_API_KEY_QUERY_AUTH` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/other/fec/cmd/fec-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "fec": {
      "command": "fec-pp-mcp",
      "env": {
        "FEC_API_KEY_QUERY_AUTH": "<your-key>"
      }
    }
  }
}
```

</details>

## Quick Start

### 1. Install

See [Install](#install) above.

### 2. Set Up Credentials

Get your API key from your API provider's developer portal. The key typically looks like a long alphanumeric string.

```bash
export FEC_API_KEY_QUERY_AUTH="<paste-your-key>"
```

To persist credentials, use `fec-pp-cli auth set-token <token>`. Stored secrets live in `credentials.toml` under the data directory, not in `config.toml`.

### 3. Verify Setup

```bash
fec-pp-cli doctor
```

This checks your configuration and credentials.

### 4. Try Your First Command

```bash
fec-pp-cli audit-case
```

## Usage

Run `fec-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data: `credentials.toml`, `data.db`, cookies, browser-session proof files, and other auth sidecars |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `FEC_CONFIG_DIR`, `FEC_DATA_DIR`, `FEC_STATE_DIR`, or `FEC_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `FEC_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export FEC_HOME=/srv/fec
fec-pp-cli doctor
```

Under `FEC_HOME=/srv/fec`, the four dirs resolve to `/srv/fec/config`, `/srv/fec/data`, `/srv/fec/state`, and `/srv/fec/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "fec": {
      "command": "fec-pp-mcp",
      "env": {
        "FEC_HOME": "/srv/fec"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `FEC_DATA_DIR` overrides an explicit `--home` for that kind. Use `FEC_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `FEC_HOME` does not move files back to platform defaults, and `doctor` cannot find credentials left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. On the first auth write, stored secrets leave `config.toml` and are consolidated into `credentials.toml` under the data directory. Run `fec-pp-cli doctor --fail-on warn` to check path and credential-location warnings in automation.

## Commands

### audit-case

Manage audit case

- **`fec-pp-cli audit-case`** - This endpoint contains Final Audit Reports approved by the Commission since inception.
The search can be based on information about the audited committee (Name, FEC ID Number, Type, 
Election Cycle) or the issues covered in the report.

### audit-category

Manage audit category

- **`fec-pp-cli audit-category`** - This lists the options for the categories and subcategories available in the /audit-search/ endpoint.

### audit-primary-category

Manage audit primary category

- **`fec-pp-cli audit-primary-category`** - This lists the options for the primary categories available in the /audit-search/ endpoint.

### calendar-dates

Manage calendar dates

- **`fec-pp-cli calendar-dates list`** - Combines the election and reporting dates with Commission meetings, conferences, outreach, Advisory Opinions, rules, litigation dates and other
events into one calendar.

State and report type filtering is no longer available.
- **`fec-pp-cli calendar-dates list-calendardates`** - Returns CSV or ICS for downloading directly into calendar applications like Google, Outlook or other applications.

Combines the election and reporting dates with Commission meetings, conferences, outreach, Advisory Opinions, rules, litigation dates and other
events into one calendar.

State filtering now applies to elections, reports and reporting periods.

Presidential pre-primary report due dates are not shown on even years.
Filers generally opt to file monthly rather than submit over 50 pre-primary election
reports. All reporting deadlines are available at /reporting-dates/ for reference.

This is [the sql function](https://github.com/fecgov/openFEC/blob/develop/data/migrations/V40__omnibus_dates.sql)
that creates the calendar.

### candidate

Candidate endpoints give you access to information about the people running for office. This information is organized by `candidate_id`. If you're unfamiliar with candidate IDs, using `/candidates/search/` will help you locate a particular candidate. 

 Officially, a candidate is an individual seeking nomination for election to a federal office. People become candidates when they (or agents working on their behalf) raise contributions or make expenditures that exceed $5,000. 

 The candidate endpoints primarily use data from FEC registration [Form 1](https://www.fec.gov/resources/cms-content/documents/fecfrm1.pdf) for committee information and [Form 2](https://www.fec.gov/resources/cms-content/documents/fecfrm2.pdf) for candidate information.

- **`fec-pp-cli candidate`** - This endpoint is useful for finding detailed information about a particular candidate. Use the
`candidate_id` to find the most recent information about that candidate.

### candidates

Candidate endpoints give you access to information about the people running for office. This information is organized by `candidate_id`. If you're unfamiliar with candidate IDs, using `/candidates/search/` will help you locate a particular candidate. 

 Officially, a candidate is an individual seeking nomination for election to a federal office. People become candidates when they (or agents working on their behalf) raise contributions or make expenditures that exceed $5,000. 

 The candidate endpoints primarily use data from FEC registration [Form 1](https://www.fec.gov/resources/cms-content/documents/fecfrm1.pdf) for committee information and [Form 2](https://www.fec.gov/resources/cms-content/documents/fecfrm2.pdf) for candidate information.

- **`fec-pp-cli candidates list`** - Fetch basic information about candidates, and use parameters to filter results to the
candidates you're looking for.

Each result reflects a unique FEC candidate ID. That ID is particular to the candidate for a
particular office sought. If a candidate runs for the same office multiple times, the ID
stays the same. If the same person runs for another office — for example, a House
candidate runs for a Senate office — that candidate will get a unique ID for each office.
- **`fec-pp-cli candidates list-search`** - Fetch basic information about candidates and their principal committees.

Each result reflects a unique FEC candidate ID. That ID is assigned to the candidate for a
particular office sought. If a candidate runs for the same office over time, that ID
stays the same. If the same person runs for multiple offices — for example, a House
candidate runs for a Senate office — that candidate will get a unique ID for each office.

The candidate endpoints primarily use data from FEC registration
[Form 1](https://www.fec.gov/pdf/forms/fecfrm1.pdf) for committee information and
[Form 2](https://www.fec.gov/pdf/forms/fecfrm2.pdf) for candidate information.
- **`fec-pp-cli candidates list-totals`** - Aggregated candidate receipts and disbursements grouped by cycle.
- **`fec-pp-cli candidates list-totals-2`** - Candidate total receipts and disbursements aggregated by `aggregate_by`.

### committee

Committees are entities that spend and raise money in an election. Their characteristics and relationships with candidates can change over time. 

 You might want to use filters or search endpoints to find the committee you're looking for. Then you can use other committee endpoints to explore information about the committee that interests you. 

 Financial information is organized by `committee_id`, so finding the committee you're interested in will lead you to more granular financial information. 

 The committee endpoints include all FEC filers, even if they aren't registered as a committee. 

 Officially, committees include the committees and organizations that file with the FEC. Several different types of organizations file financial reports with the FEC: 

 *Campaign committees authorized by particular candidates to raise and spend funds in their campaigns. Non-party committees (e.g., PACs), some of which may be sponsored by corporations, unions, trade or membership groups, etc. Political party committees at the national, state, and local levels. Groups and individuals making only independent expenditures Corporations, unions, and other organizations making internal communications* 

 The committee endpoints primarily use data from FEC registration Form 1 and Form 2.

- **`fec-pp-cli committee <committee_id>`** - This endpoint is useful for finding detailed information about a particular committee or
filer. Use the `committee_id` to find the most recent information about the committee.

### committees

Committees are entities that spend and raise money in an election. Their characteristics and relationships with candidates can change over time. 

 You might want to use filters or search endpoints to find the committee you're looking for. Then you can use other committee endpoints to explore information about the committee that interests you. 

 Financial information is organized by `committee_id`, so finding the committee you're interested in will lead you to more granular financial information. 

 The committee endpoints include all FEC filers, even if they aren't registered as a committee. 

 Officially, committees include the committees and organizations that file with the FEC. Several different types of organizations file financial reports with the FEC: 

 *Campaign committees authorized by particular candidates to raise and spend funds in their campaigns. Non-party committees (e.g., PACs), some of which may be sponsored by corporations, unions, trade or membership groups, etc. Political party committees at the national, state, and local levels. Groups and individuals making only independent expenditures Corporations, unions, and other organizations making internal communications* 

 The committee endpoints primarily use data from FEC registration Form 1 and Form 2.

- **`fec-pp-cli committees`** - Fetch basic information about committees and filers. Use parameters to filter for
particular characteristics.

### communication-costs

Reports of communication costs by corporations and membership organizations from the FEC [F7 forms](https://www.fec.gov/pdf/forms/fecform7.pdf).

- **`fec-pp-cli communication-costs list`** - 52 U.S.C. 30118 allows "communications by a corporation to its stockholders and executive or administrative personnel and their families or by a labor organization to its members and their families on any subject," including the express advocacy of the election or defeat of any Federal candidate.  The costs of such communications must be reported to the Federal Election Commission under certain circumstances.
- **`fec-pp-cli communication-costs list-communicationcosts`** - Communication cost aggregated by candidate ID and committee ID.
- **`fec-pp-cli communication-costs list-communicationcosts-2`** - Communication cost aggregated by candidate ID and committee ID.
- **`fec-pp-cli communication-costs list-communicationcosts-3`** - Total communications costs aggregated across committees on supported or opposed candidates by cycle or candidate election year.

### efile

Manage efile

- **`fec-pp-cli efile list`** - Basic information about electronic files coming into the FEC, posted as they are received.
- **`fec-pp-cli efile list-form1`** - Basic information about electronic files coming into the FEC, posted as they are received.
- **`fec-pp-cli efile list-form2`** - Basic information about electronic files coming into the FEC, posted as they are received.
- **`fec-pp-cli efile list-reports`** - Key financial data reported periodically by committees as they are reported. This feed includes summary
information from the the House F3 reports, the presidential F3p reports and the PAC and party
F3x reports.

Generally, committees file reports on a quarterly or monthly basis, but
some must also submit a report 12 days before primary elections. Therefore, during the primary
season, the period covered by this file may be different for different committees. These totals
also incorporate any changes made by committees, if any report covering the period is amended.

DISCLAIMER: The field labels contained within this resource are subject to change.  We are attempting to succinctly
label these fields while conveying clear meaning to ensure accessibility for all users.
- **`fec-pp-cli efile list-reports-2`** - Key financial data reported periodically by committees as they are reported. This feed includes summary
information from the the House F3 reports, the presidential F3p reports and the PAC and party
F3x reports.

Generally, committees file reports on a quarterly or monthly basis, but
some must also submit a report 12 days before primary elections. Therefore, during the primary
season, the period covered by this file may be different for different committees. These totals
also incorporate any changes made by committees, if any report covering the period is amended.

DISCLAIMER: The field labels contained within this resource are subject to change.  We are attempting to succinctly
label these fields while conveying clear meaning to ensure accessibility for all users.
- **`fec-pp-cli efile list-reports-3`** - Key financial data reported periodically by committees as they are reported. This feed includes summary
information from the the House F3 reports, the presidential F3p reports and the PAC and party
F3x reports.

Generally, committees file reports on a quarterly or monthly basis, but
some must also submit a report 12 days before primary elections. Therefore, during the primary
season, the period covered by this file may be different for different committees. These totals
also incorporate any changes made by committees, if any report covering the period is amended.

DISCLAIMER: The field labels contained within this resource are subject to change.  We are attempting to succinctly
label these fields while conveying clear meaning to ensure accessibility for all users.

### election-dates

Manage election dates

- **`fec-pp-cli election-dates`** - FEC election dates since 1995.

### electioneering

An electioneering communication is any broadcast, cable or satellite communication that fulfills each of the following conditions: 

 _The communication refers to a clearly identified federal candidate._ 

 _The communication is publicly distributed by a television station, radio station, cable television system or satellite system for a fee._ 

 _The communication is distributed within 60 days prior to a general election or 30 days prior to a primary election to federal office._

- **`fec-pp-cli electioneering list`** - An electioneering communication is any broadcast, cable or satellite communication that fulfills each of the following conditions:

_The communication refers to a clearly identified federal candidate._

_The communication is publicly distributed by a television station, radio station, cable television system or satellite system for a fee._

_The communication is distributed within 60 days prior to a general election or 30 days prior to a primary election to federal office._
- **`fec-pp-cli electioneering list-aggregates`** - Electioneering communications costs aggregates
- **`fec-pp-cli electioneering list-bycandidate`** - Electioneering costs aggregated by candidate
- **`fec-pp-cli electioneering list-totals`** - Total electioneering communications spent on candidates by cycle
or candidate election year

### elections

Manage elections

- **`fec-pp-cli elections list`** - Look at the top-level financial information for all candidates running for the same
office.

Choose a 2-year cycle, and `house`, `senate` or `presidential`.

If you are looking for a Senate seat, you will need to select the state using a two-letter
abbreviation.

House races require state and a two-digit district number.

Since this endpoint reflects financial information, it will only have candidates once they file
financial reporting forms. Query the `/candidates` endpoint to retrieve an-up-to-date list of all the
candidates that filed to run for a particular seat.
- **`fec-pp-cli elections list-search`** - List elections by cycle, office, state, and district.
- **`fec-pp-cli elections list-summary`** - List elections by cycle, office, state, and district.

### filings

All official records and reports filed by or delivered to the FEC. 

 Note: because the filings data includes many records, counts for large result sets are approximate; you will want to page through the records until no records are returned.

- **`fec-pp-cli filings`** - All official records and reports filed by or delivered to the FEC.

Note: because the filings data includes many records, counts for large
result sets are approximate; you will want to page through the records until no records are returned.

### legal

Explore relevant statutes, regulations and Commission actions.

- **`fec-pp-cli legal get`** - Search legal documents by type and number
- **`fec-pp-cli legal list`** - Search legal documents by document type, or across all document types using keywords, parameter values and ranges.
This endpoint uses opensearch-dsl pagination.For pagination, use both `from_hit` and `hits_returned` parameters. `from_hit` defines the offset from the first result you want to fetch. `hits_returned` allows you to configure the maximum results to be returned.
By default `from_hit` = 0 and `hits_returned` = 20, endpoint will return the first 20 documents (i.e. 0 to 19).
if set `from_hit` = 20 and `hits_returned` = 20, endpoint will return documents range from 21 to 40 (i.e. 20 to 39).
The maximum value of `hits_returned` is 200.

### names

Manage names

- **`fec-pp-cli names list`** - Search for candidates or committees by name. If you're looking for information on a
particular person or group, using a name to find the `candidate_id` or `committee_id` on
this endpoint can be a helpful first step.
- **`fec-pp-cli names list-auditcommittees`** - Search for candidates or committees by name. If you're looking for information on a
particular person or group, using a name to find the `candidate_id` or `committee_id` on
this endpoint can be a helpful first step.
- **`fec-pp-cli names list-candidates`** - Search for candidates or committees by name. If you're looking for information on a
particular person or group, using a name to find the `candidate_id` or `committee_id` on
this endpoint can be a helpful first step.
- **`fec-pp-cli names list-committees`** - Search for candidates or committees by name. If you're looking for information on a
particular person or group, using a name to find the `candidate_id` or `committee_id` on
this endpoint can be a helpful first step.

### national-party

Manage national party

- **`fec-pp-cli national-party list`** - This endpoint includes national party committee account receipts for presidential nominating conventions,
national party headquarters buildings, and election recounts and contests and other legal proceedings accounts.
- **`fec-pp-cli national-party list-nationalparty`** - This endpoint includes national party committee account disbursements for presidential nominating conventions,
national party headquarters buildings, and election recounts and contests and other legal proceedings accounts
- **`fec-pp-cli national-party list-nationalparty-2`** - This endpoint includes national party committee account total receipts and total disbursements for 

presidential nominating conventions, national party headquarters buildings, and election recounts 

and contests and other legal proceedings accounts for a given two year cycle.

### operations-log

Manage operations log

- **`fec-pp-cli operations-log`** - The Operations log contains details of each report loaded into the database. It is primarily
used as status check to determine when all of the data processes, from initial entry through
review are complete.

### presidential

Data supporting fec.gov's presidential map. 

 For more information about the presidential map data available to download from fec.gov, please visit: https://www.fec.gov/campaign-finance-data/presidential-map-data/

- **`fec-pp-cli presidential list`** - Coverage end date per candidate.

Filter by candidate_id and/or election_year
- **`fec-pp-cli presidential list-contributions`** - Net receipts per candidate.

Filter with `contributor_state='US'` for national totals
- **`fec-pp-cli presidential list-contributions-2`** - Contribution receipts by size per candidate.

Filter by candidate_id, election_year and/or size
- **`fec-pp-cli presidential list-contributions-3`** - Contribution receipts by state per candidate.

Filter by candidate_id and/or election_year
- **`fec-pp-cli presidential list-financialsummary`** - Financial summary per candidate.

Filter by candidate_id and/or election_year

### rad-analyst

Manage rad analyst

- **`fec-pp-cli rad-analyst`** - Use this endpoint to look up the RAD Analyst for a committee.

The mission of the Reports Analysis Division (RAD) is to ensure that
campaigns and political committees file timely and accurate reports that fully disclose
their financial activities.  RAD is responsible for reviewing statements and financial
reports filed by political committees participating in federal elections, providing
assistance and guidance to the committees to properly file their reports, and for taking
appropriate action to ensure compliance with the Federal Election Campaign Act (FECA).

### reporting-dates

Manage reporting dates

- **`fec-pp-cli reporting-dates`** - FEC election dates since 1995.

### reports

Manage reports

- **`fec-pp-cli reports`** - Each report represents the summary information from Form 3, Form 3X and Form 3P.
These reports have key statistics that illuminate the financial status of a given committee.
Things like cash on hand, debts owed by committee, total receipts, and total disbursements
are especially helpful for understanding a committee's financial dealings.

By default, this endpoint includes both amended and final versions of each report. To restrict
to only the final versions of each report, use `is_amended=false`; to retrieve only reports that
have been amended, use `is_amended=true`.

Several different reporting structures exist, depending on the type of organization that
submits financial information. To see an example of these reporting requirements,
look at the summary and detailed summary pages of Form 3, Form 3X, and Form 3P.

DISCLAIMER: The field labels contained within this resource are subject to change.  We are attempting to succinctly
label these fields while conveying clear meaning to ensure accessibility for all users.

### rulemaking

Manage rulemaking

- **`fec-pp-cli rulemaking`** - The Searchable Electronic Rulemaking System (SERS) lets you search all public documents associated
with Federal Election Commission rulemakings (REGs), including draft Federal Register publications,
 open meeting agendas, comments submitted by the public, and hearing transcripts.

### schedules

Manage schedules

- **`fec-pp-cli schedules get`** - This description is for both ​`/schedules​/schedule_a​/` and ​ `/schedules​/schedule_a​/{sub_id}​/`.

This endpoint provides itemized receipts. Schedule A records describe itemized receipts, including contributions from individuals. If you are interested in contributions from an individual, use the `/schedules/schedule_a/` endpoint. For a more complete description of all Schedule A records visit [About receipts data](https://www.fec.gov/campaign-finance-data/about-campaign-finance-data/about-receipts-data/). If you are interested in our "is_individual" methodology visit our [methodology page](https://www.fec.gov/campaign-finance-data/about-campaign-finance-data/methodology/). 
​The `/schedules​/schedule_a​/` endpoint is not paginated by page number. This endpoint uses keyset pagination to improve query performance and these indices are required to properly page through this large dataset. To request the next page, you should append the values found in the `last_indexes` object from pagination to the URL of your last request as additional parameters. 
For example, when sorting by `contribution_receipt_date`, you might receive a page of results with the two scenarios of following pagination information:

case #1:
```
pagination: {
    pages: 2152643,
    per_page: 20,
    is_count_exact: False,
    count: 43052850,
    last_indexes: {
        last_index: "230880619",
        last_contribution_receipt_date: "2014-01-01"
    }
}
```
<br/>
case #2 (results which include contribution_receipt_date = NULL):

```
pagination: {
    pages: 2152644,
    per_page: 20,
    count: 43052850,
    is_count_exact: False,
    last_indexes: {
        last_index: "230880639",
        sort_null_only: True
    }
}
```
To fetch the next page of sorted results, append `last_index=230880619` and `last_contribution_receipt_date=2014-01-01` to the URL and when reaching `contribution_receipt_date=NULL`, append `last_index=230880639` and `sort_null_only=True`. We strongly advise paging through these results using sort indices. The default sort is acending by `contribution_receipt_date` (`deprecated`, will be descending). If you do not page using sort indices, some transactions may be unintentionally filtered out.

Calls to ​`/schedules​/schedule_a​/` may return many records. For large result sets, the record counts found in the pagination object are approximate; you will need to page through the records until no records are returned.

To avoid throwing the "out of range" exception on the last page, one recommandation is to use total count and `per_page` to control the traverse loop of results.

​The `/schedules​/schedule_a​/{sub_id}​/` endpoint returns a single transaction, but it does include a pagination object class. Please ignore the information in that object class.
- **`fec-pp-cli schedules get-scheduleb`** - Schedule B filings describe itemized disbursements. This data
explains how committees and other filers spend their money. These figures are
reported as part of forms F3, F3X and F3P.

The data are divided in two-year periods, called `two_year_transaction_period`, which
is derived from the `report_year` submitted of the corresponding form. If no value is supplied, the results will
default to the most recent two-year period that is named after the ending,
even-numbered year.

Due to the large quantity of Schedule B filings, this endpoint is not paginated by
page number. Instead, you can request the next page of results by adding the values in
the `last_indexes` object from `pagination` to the URL of your last request. For
example, when sorting by `disbursement_date`, you might receive a page of
results with the following pagination information:

```
pagination: {
    pages: 965191,
    per_page: 20,
    count: 19303814,
    is_count_exact: False,
    last_indexes: {
        last_index: "230906248",
        last_disbursement_date: "2014-07-04"
    }
}
```

To fetch the next page of sorted results, append `last_index=230906248` and
`last_disbursement_date=2014-07-04` to the URL.  We strongly advise paging through
these results by using the sort indices (defaults to sort by disbursement date, e.g.
`last_disbursement_date`), otherwise some resources may be unintentionally filtered out.
This resource uses keyset pagination to improve query performance
and these indices are required to properly page through this large dataset.

Note: because the Schedule B data includes many records, counts for
large result sets are approximate; you will want to page through the records until no records are returned.
- **`fec-pp-cli schedules get-schedulec`** - Schedule C shows all loans, endorsements and loan guarantees a committee
receives or makes.

The committee continues to report the loan until it is repaid.
- **`fec-pp-cli schedules get-scheduled`** - Schedule D, it shows debts and obligations owed to or by the committee that are
required to be disclosed.
- **`fec-pp-cli schedules get-schedulef`** - Schedule F, it shows all special expenditures a national or state party committee
makes in connection with the general election campaigns of federal candidates.

These coordinated party expenditures do not count against the contribution limits but are subject to other limits,
these limits are detailed in Chapter 7 of the FEC Campaign Guide for Political Party Committees.
- **`fec-pp-cli schedules list`** - This description is for both ​`/schedules​/schedule_a​/` and ​ `/schedules​/schedule_a​/{sub_id}​/`.

This endpoint provides itemized receipts. Schedule A records describe itemized receipts, including contributions from individuals. If you are interested in contributions from an individual, use the `/schedules/schedule_a/` endpoint. For a more complete description of all Schedule A records visit [About receipts data](https://www.fec.gov/campaign-finance-data/about-campaign-finance-data/about-receipts-data/). If you are interested in our "is_individual" methodology visit our [methodology page](https://www.fec.gov/campaign-finance-data/about-campaign-finance-data/methodology/). 
​The `/schedules​/schedule_a​/` endpoint is not paginated by page number. This endpoint uses keyset pagination to improve query performance and these indices are required to properly page through this large dataset. To request the next page, you should append the values found in the `last_indexes` object from pagination to the URL of your last request as additional parameters. 
For example, when sorting by `contribution_receipt_date`, you might receive a page of results with the two scenarios of following pagination information:

case #1:
```
pagination: {
    pages: 2152643,
    per_page: 20,
    is_count_exact: False,
    count: 43052850,
    last_indexes: {
        last_index: "230880619",
        last_contribution_receipt_date: "2014-01-01"
    }
}
```
<br/>
case #2 (results which include contribution_receipt_date = NULL):

```
pagination: {
    pages: 2152644,
    per_page: 20,
    count: 43052850,
    is_count_exact: False,
    last_indexes: {
        last_index: "230880639",
        sort_null_only: True
    }
}
```
To fetch the next page of sorted results, append `last_index=230880619` and `last_contribution_receipt_date=2014-01-01` to the URL and when reaching `contribution_receipt_date=NULL`, append `last_index=230880639` and `sort_null_only=True`. We strongly advise paging through these results using sort indices. The default sort is acending by `contribution_receipt_date` (`deprecated`, will be descending). If you do not page using sort indices, some transactions may be unintentionally filtered out.

Calls to ​`/schedules​/schedule_a​/` may return many records. For large result sets, the record counts found in the pagination object are approximate; you will need to page through the records until no records are returned.

To avoid throwing the "out of range" exception on the last page, one recommandation is to use total count and `per_page` to control the traverse loop of results.

​The `/schedules​/schedule_a​/{sub_id}​/` endpoint returns a single transaction, but it does include a pagination object class. Please ignore the information in that object class.
- **`fec-pp-cli schedules list-schedulea`** - This endpoint provides itemized individual contributions received by a committee, aggregated by the contributor’s employer name. If you are interested in our “is_individual” methodology, review the [methodology page](https://www.fec.gov/campaign-finance-data/about-campaign-finance-data/methodology). Unitemized individual contributions are not included.
- **`fec-pp-cli schedules list-schedulea-10`** - Itemized individual contributions aggregated by contributor’s state, candidate, committee type and cycle. If you are interested in our “is_individual” methodology, review the [methodology page](https://www.fec.gov/campaign-finance-data/about-campaign-finance-data/methodology). Unitemized individual contributions are not included.
- **`fec-pp-cli schedules list-schedulea-2`** - This endpoint provides itemized individual contributions received by a committee, aggregated by the contributor’s occupation. If you are interested in our “is_individual” methodology, review the [methodology page](https://www.fec.gov/campaign-finance-data/about-campaign-finance-data/methodology). Unitemized individual contributions are not included.
- **`fec-pp-cli schedules list-schedulea-3`** - This endpoint provides individual contributions received by a committee, aggregated by size:

```
 - $200 and under
 - $200.01 - $499.99
 - $500 - $999.99
 - $1000 - $1999.99
 - $2000 +
```

The $200.00 and under category includes contributions of $200 or less combined with unitemized individual contributions.
- **`fec-pp-cli schedules list-schedulea-4`** - This endpoint provides itemized individual contributions received by a committee, aggregated by the contributor’s state. If you are interested in our “is_individual” methodology, review the [methodology page](https://www.fec.gov/campaign-finance-data/about-campaign-finance-data/methodology). Unitemized individual contributions are not included.
- **`fec-pp-cli schedules list-schedulea-5`** - This endpoint provides itemized individual contributions received by a committee, aggregated by the contributor’s ZIP code. If you are interested in our “is_individual” methodology, review the [methodology page](https://www.fec.gov/campaign-finance-data/about-campaign-finance-data/methodology). Unitemized individual contributions are not included.
- **`fec-pp-cli schedules list-schedulea-6`** - Efiling endpoints provide real-time campaign finance data received from electronic filers. Efiling endpoints only contain the most recent four months of data and don't contain the processed and coded data that you can find on other endpoints.
- **`fec-pp-cli schedules list-schedulea-7`** - This endpoint provides itemized individual contributions received by a committee, aggregated by size of contribution and candidate. If you are interested in our “is_individual” methodology, review the [methodology page](https://www.fec.gov/campaign-finance-data/about-campaign-finance-data/methodology). Unitemized individual contributions are not included.
- **`fec-pp-cli schedules list-schedulea-8`** - This endpoint provides itemized individual contributions received by a committee, aggregated by contributor’s state and candidate. If you are interested in our “is_individual” methodology, review the [methodology page](https://www.fec.gov/campaign-finance-data/about-campaign-finance-data/methodology). Unitemized individual contributions are not included.
- **`fec-pp-cli schedules list-schedulea-9`** - This endpoint provides itemized individual contributions received by a committee, aggregated by contributor’s state, committee type and cycle. If you are interested in our “is_individual” methodology, review the [methodology page](https://www.fec.gov/campaign-finance-data/about-campaign-finance-data/methodology). Unitemized individual contributions are not included.
- **`fec-pp-cli schedules list-scheduleaform5`** - FEC FORM 5 Receipts
REPORT OF INDEPENDENT EXPENDITURES MADE AND CONTRIBUTIONS RECEIVED
To Be Used By Persons (Other than Political Committees)
- **`fec-pp-cli schedules list-scheduleb`** - Schedule B filings describe itemized disbursements. This data
explains how committees and other filers spend their money. These figures are
reported as part of forms F3, F3X and F3P.

The data are divided in two-year periods, called `two_year_transaction_period`, which
is derived from the `report_year` submitted of the corresponding form. If no value is supplied, the results will
default to the most recent two-year period that is named after the ending,
even-numbered year.

Due to the large quantity of Schedule B filings, this endpoint is not paginated by
page number. Instead, you can request the next page of results by adding the values in
the `last_indexes` object from `pagination` to the URL of your last request. For
example, when sorting by `disbursement_date`, you might receive a page of
results with the following pagination information:

```
pagination: {
    pages: 965191,
    per_page: 20,
    count: 19303814,
    is_count_exact: False,
    last_indexes: {
        last_index: "230906248",
        last_disbursement_date: "2014-07-04"
    }
}
```

To fetch the next page of sorted results, append `last_index=230906248` and
`last_disbursement_date=2014-07-04` to the URL.  We strongly advise paging through
these results by using the sort indices (defaults to sort by disbursement date, e.g.
`last_disbursement_date`), otherwise some resources may be unintentionally filtered out.
This resource uses keyset pagination to improve query performance
and these indices are required to properly page through this large dataset.

Note: because the Schedule B data includes many records, counts for
large result sets are approximate; you will want to page through the records until no records are returned.
- **`fec-pp-cli schedules list-scheduleb-2`** - Schedule B disbursements aggregated by disbursement purpose category. To avoid double counting,
memoed items are not included.
Purpose is a combination of transaction codes, category codes and disbursement description.
Inspect the `disbursement_purpose` sql function within the migrations for more details.
- **`fec-pp-cli schedules list-scheduleb-3`** - Schedule B disbursements aggregated by recipient name. To avoid double counting,
memoed items are not included.
- **`fec-pp-cli schedules list-scheduleb-4`** - Schedule B disbursements aggregated by recipient committee ID, if applicable.
To avoid double counting, memoed items are not included.
- **`fec-pp-cli schedules list-scheduleb-5`** - Efiling endpoints provide real-time campaign finance data received from electronic filers. Efiling endpoints only contain the most recent four months of data and don't contain the processed and coded data that you can find on other endpoints.
- **`fec-pp-cli schedules list-schedulec`** - Schedule C shows all loans, endorsements and loan guarantees a committee
receives or makes.

The committee continues to report the loan until it is repaid.
- **`fec-pp-cli schedules list-scheduled`** - Schedule D, it shows debts and obligations owed to or by the committee that are
required to be disclosed.
- **`fec-pp-cli schedules list-schedulee`** - Schedule E covers the line item expenditures for independent expenditures. For example, if a super PAC
bought ads on TV to oppose a federal candidate, each ad purchase would be recorded here with
the expenditure amount, name and id of the candidate, and whether the ad supported or opposed the candidate.

An independent expenditure is an expenditure for a communication "expressly advocating the election or
defeat of a clearly identified candidate that is not made in cooperation, consultation, or concert with,
or at the request or suggestion of, a candidate, a candidate’s authorized committee, or their agents, or
a political party or its agents."

Aggregates by candidate do not include 24 and 48 hour reports. This ensures we don't double count expenditures
and the totals are more accurate. You can still find the information from 24 and 48 hour reports in
`/schedule/schedule_e/`.

Due to the large quantity of Schedule E filings, this endpoint is not paginated by
page number. Instead, you can request the next page of results by adding the values in
the `last_indexes` object from `pagination` to the URL of your last request. For
example, when sorting by `expenditure_amount`, you might receive a page of
results with the following pagination information:

```
 "pagination": {
    "count": 152623,
    "is_count_exact": True,
    "last_indexes": {
      "last_index": "3023037",
      "last_expenditure_amount": -17348.5
    },
    "per_page": 20,
    "pages": 7632
  }
}
```

To fetch the next page of sorted results, append `last_index=3023037` and
`last_expenditure_amount=` to the URL.  We strongly advise paging through
these results by using the sort indices (defaults to sort by disbursement date,
e.g. `last_disbursement_date`), otherwise some resources may be unintentionally
filtered out.  This resource uses keyset pagination to improve query performance
and these indices are required to properly page through this large dataset.

Note: because the Schedule E data includes many records, counts for
large result sets are approximate; you will want to page through the records until no records are returned.
- **`fec-pp-cli schedules list-schedulee-2`** - Schedule E receipts aggregated by recipient candidate. To avoid double
counting, memoed items are not included.
- **`fec-pp-cli schedules list-schedulee-3`** - Efiling endpoints provide real-time campaign finance data received from electronic filers. Efiling endpoints only contain the most recent four months of data and don't contain the processed and coded data that you can find on other endpoints.
- **`fec-pp-cli schedules list-schedulee-4`** - Total independent expenditure on supported or opposed candidates by cycle or candidate election year.
- **`fec-pp-cli schedules list-schedulef`** - Schedule F, it shows all special expenditures a national or state party committee
makes in connection with the general election campaigns of federal candidates.

These coordinated party expenditures do not count against the contribution limits but are subject to other limits,
these limits are detailed in Chapter 7 of the FEC Campaign Guide for Political Party Committees.
- **`fec-pp-cli schedules list-scheduleh4`** - Schedule H4 filings describe disbursements for allocated federal/nonfederal activity. This data
demonstrates how separate segregated funds, party committees and nonconnected committees that are active
in both federal and nonfederal elections, and have established separate federal and nonfederal accounts,
allocate their activity. These figures are reported on Form 3X.

The data are divided in two-year periods, called `two_year_transaction_period`, which are derived from the
`report_year` submitted on Form 3X. If no value is supplied, the results will default to the most recent
two-year period.
- **`fec-pp-cli schedules list-scheduleh4-2`** - Efiling endpoints provide real-time campaign finance data received from electronic filers. Efiling endpoints only contain the most recent four months of data and don't contain the processed and coded data that you can find on other endpoints.

### state-election-office

Manage state election office

- **`fec-pp-cli state-election-office`** - State laws and procedures govern elections for state or local offices as well as
how candidates appear on election ballots.
Contact the appropriate state election office for more information.

### totals

Manage totals

- **`fec-pp-cli totals get`** - This endpoint provides information about a committee's Form 3, Form 3X, or Form 3P financial reports,
which are aggregated by two-year period. We refer to two-year periods as a `cycle`.

The cycle is named after the even-numbered year and includes the year before it. To obtain
totals from 2013 and 2014, you would use 2014. In odd-numbered years, the current cycle
is the next year — for example, in 2015, the current cycle is 2016.

For presidential and Senate candidates, multiple two-year cycles exist between elections.
- **`fec-pp-cli totals list`** - Provides cumulative receipt totals by entity type, over a two year cycle. Totals are adjusted to avoid double counting.

This is [the sql](https://github.com/fecgov/openFEC/blob/develop/data/migrations/V41__large_aggregates.sql) that creates these calculations.
- **`fec-pp-cli totals list-inauguralcommittees`** - This endpoint provides information about an inaugural committee's Form 13 report of donations accepted.
The data is aggregated by the contributor and the two-year period. We refer to two-year periods as a `cycle`.


### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`fec-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`fec-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`fec-pp-cli learnings list`** - Inspect taught rows
- **`fec-pp-cli learnings forget <query>`** - Undo a teach
- **`fec-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`fec-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`fec-pp-cli teach-pattern`** - Install a query/resource template up front
- **`fec-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `FEC_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `fec-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
fec-pp-cli audit-case

# JSON for scripting and agents
fec-pp-cli audit-case --json

# Filter to specific fields
fec-pp-cli audit-case --json --select id,name,status

# Dry run — show the request without sending
fec-pp-cli audit-case --dry-run

# Agent mode — JSON + compact + no prompts in one flag
fec-pp-cli audit-case --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Read-only by default** - this CLI does not create, update, delete, publish, send, or mutate remote resources
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
fec-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Run `fec-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/openfec-pp-cli/config.toml`; `--home`, `FEC_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `FEC_API_KEY_QUERY_AUTH` | per_call | Yes | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `fec-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `fec-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $FEC_API_KEY_QUERY_AUTH`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
