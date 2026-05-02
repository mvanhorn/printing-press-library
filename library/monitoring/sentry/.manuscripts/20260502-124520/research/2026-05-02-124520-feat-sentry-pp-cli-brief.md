# Sentry CLI Brief

## API Identity
- Domain: application monitoring, issue triage, release health, replay, alerts, projects, teams, integrations, and observability data.
- Users: on-call engineers, release managers, platform teams, AI coding agents debugging production failures, and SREs auditing projects across organizations.
- Data profile: mostly authenticated JSON resources keyed by organization, project, issue, event, release, team, monitor, alert, replay, and integration identifiers.

## Reachability Risk
- Low. The catalog OpenAPI resolves, the spec advertises bearer auth, and an authenticated base API probe returned HTTP 200.
- Runtime must support Sentry SaaS regional hosts. The catalog spec server is `https://{region}.sentry.io` with `us` and `de` region variables.

## Top Workflows
1. Triage issues by organization/project, status, environment, recency, count, level, and short ID.
2. Fetch event details, issue events, tags, hashes, and related release data for debugging.
3. Audit projects, teams, members, keys, environments, monitors, alert rules, workflows, and integrations.
4. Inspect releases, commits, deploys, release files, and release health session statistics.
5. Export and locally search synchronized Sentry data for incident review and agent context.

## Table Stakes
- Official Sentry CLI emphasizes auth login, issue list, issue explain/plan, project detection, JSON output, self-hosted support, releases, and automation.
- Sentry MCP focuses on human-in-the-loop coding-agent debugging, issue/event/search workflows, and Sentry-hosted/self-hosted connection modes.
- The printed CLI must match broad API coverage, structured output, local store/search/SQL, and scriptable commands.

## Data Layer
- Primary entities: organizations, projects, issues, events, releases, teams, members, monitors, dashboards, integrations, environments, alerts/workflows, replays, and project keys.
- Sync cursor: organization/project scoped pagination and updated/seen timestamps where exposed.
- FTS/search: issue titles, event messages, release versions, project/team slugs, monitor names, integration names, and workflow names.

## Product Thesis
- Name: Sentry Printing Press CLI
- Why it should exist: expose the broad Sentry OpenAPI as a predictable Go CLI and MCP server, while adding local sync/search/SQL surfaces that make incident triage and agent workflows composable.

## Build Priorities
1. Generate full spec-backed command and MCP coverage from the catalog OpenAPI.
2. Preserve bearer-token auth and regional host configurability.
3. Verify command docs, SKILL recipes, and generated README examples against the actual command tree.
4. Run shipcheck and live read-only smoke testing with the provided Sentry token.
