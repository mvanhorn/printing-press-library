---
name: pp-linear
description: "Printing Press CLI for Linear. Offline-capable, agent-native CLI for the Linear API with SQLite-backed sync, search, and cross-entity queries. Capabilities include: analytics, attachments, audit-entry-types, auth-resolver-responses, authentication-session-responses, bottleneck, custom-views, customer-statuses, customer-tiers, customers, cycles, documents, email-intake-addresses, entity-external-links, favorites, feedback, initiative-to-projects, initiatives, integration-templates, integrations, integrations-settingses, issue-labels, issue-priority-values, issue-relations, issue-to-releases, issues, me, organization-invites, organization-metas, organizations, profile, project-labels, project-milestones, project-relations, project-statuses, projects, release-pipelines, release-stages, releases, roadmap-to-projects, roadmaps, similar, tail, team-memberships, teams, templates, today, user-settingses, users, velocity, workflow-states, workload. Trigger phrases: 'install linear', 'use linear', 'run linear', 'Linear commands', 'setup linear'."
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata: '{"openclaw":{"requires":{"bins":["linear-pp-cli"],"env":["LINEAR_API_KEY"]},"primaryEnv":"LINEAR_API_KEY","install":[{"id":"go","kind":"shell","command":"go install github.com/mvanhorn/printing-press-library/library/project-management/linear/cmd/linear-pp-cli@latest","bins":["linear-pp-cli"],"label":"Install via go install"}]}}'
---

# Linear — Printing Press CLI

Offline-capable, agent-native CLI for the Linear API with SQLite-backed sync, search, and cross-entity queries.

## Argument Parsing

Parse `$ARGUMENTS`:

1. **Empty, `help`, or `--help`** → show `linear-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → CLI installation
3. **Anything else** → Direct Use (execute as CLI command)

## CLI Installation

1. Check Go is installed: `go version` (requires Go 1.23+)
2. Install:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/project-management/linear/cmd/linear-pp-cli@latest
   ```

   If `@latest` installs a stale build (the Go module proxy cache can lag the repo by hours after a fresh merge), install from main directly:
   ```bash
   GOPRIVATE='github.com/mvanhorn/*' GOFLAGS=-mod=mod \
     go install github.com/mvanhorn/printing-press-library/library/project-management/linear/cmd/linear-pp-cli@main
   ```
3. Verify: `linear-pp-cli --version`
4. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.
5. Auth setup — set the API key and register it with the CLI:
   ```bash
   export LINEAR_API_KEY="your-key-here"
   linear-pp-cli auth set-token
   ```
   Run `linear-pp-cli doctor` to verify credentials.

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/project-management/linear/cmd/linear-pp-mcp@latest
   ```

   If `@latest` installs a stale build, install from main directly:
   ```bash
   GOPRIVATE='github.com/mvanhorn/*' GOFLAGS=-mod=mod \
     go install github.com/mvanhorn/printing-press-library/library/project-management/linear/cmd/linear-pp-mcp@main
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add -e LINEAR_API_KEY=value linear-pp-mcp -- linear-pp-mcp
   ```
   Ask the user for actual values of required API keys before running.
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which linear-pp-cli`
   If not found, offer to install (see CLI Installation above).
2. Discover commands: `linear-pp-cli --help`
   Key commands:
   - `analytics` — Run analytics queries on locally synced data
   - `attachments` — Get a single attachment
   - `audit-entry-types` — Get a single auditentrytype
   - `auth-resolver-responses` — Get a single authresolverresponse
   - `authentication-session-responses` — Get a single authenticationsessionresponse
   - `bottleneck` — Find overloaded team members and blocked issues
   - `custom-views` — Get a single customview
   - `customer-statuses` — Get a single customerstatus
   - `customer-tiers` — Get a single customertier
   - `customers` — Get a single customer
   - `cycles` — Get a single cycle
   - `documents` — Get a single document
   - `email-intake-addresses` — Get a single emailintakeaddress
   - `entity-external-links` — Get a single entityexternallink
   - `favorites` — Get a single favorite
   - `feedback` — Record feedback about this CLI (local by default; upstream opt-in)
   - `initiative-to-projects` — Get a single initiativetoproject
   - `initiatives` — Get a single initiative
   - `integration-templates` — Get a single integrationtemplate
   - `integrations` — Get a single integration
   - `integrations-settingses` — Get a single integrationssettings
   - `issue-labels` — Get a single issuelabel
   - `issue-priority-values` — Get a single issuepriorityvalue
   - `issue-relations` — Get a single issuerelation
   - `issue-to-releases` — Get a single issuetorelease
   - `issues` — Get a single issue
   - `me` — Show current authenticated user
   - `organization-invites` — Get a single organizationinvite
   - `organization-metas` — Get a single organizationmeta
   - `organizations` — Get a single organization
   - `profile` — Named sets of flags saved for reuse
   - `project-labels` — Get a single projectlabel
   - `project-milestones` — Get a single projectmilestone
   - `project-relations` — Get a single projectrelation
   - `project-statuses` — Get a single projectstatus
   - `projects` — Get a single project
   - `release-pipelines` — Get a single releasepipeline
   - `release-stages` — Get a single releasestage
   - `releases` — Get a single release
   - `roadmap-to-projects` — Get a single roadmaptoproject
   - `roadmaps` — Get a single roadmap
   - `similar` — Find potentially duplicate issues using fuzzy text search
   - `tail` — Stream live changes by polling the API at regular intervals
   - `team-memberships` — Get a single teammembership
   - `teams` — Get a single team
   - `templates` — Get a single template
   - `today` — Show your issues for today across all teams
   - `user-settingses` — Get a single usersettings
   - `users` — Get a single user
   - `velocity` — Show sprint velocity trends over recent cycles
   - `workflow-states` — Get a single workflowstate
   - `workload` — Show issue and estimate distribution per team member
3. Match the user query to the best command. Drill into subcommand help if needed: `linear-pp-cli <command> --help`
4. Execute with the `--agent` flag:
   ```bash
   linear-pp-cli <command> [subcommand] [args] --agent
   ```
5. The `--agent` flag sets `--json --compact --no-input --no-color --yes` for structured, token-efficient output.

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 2 | Usage error (wrong arguments) |
| 3 | Resource not found |
| 4 | Authentication required |
| 5 | API error (upstream issue) |
| 7 | Rate limited (wait and retry) |
