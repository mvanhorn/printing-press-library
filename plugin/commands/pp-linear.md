---
description: "Printing Press CLI for Linear. Offline-capable, agent-native CLI for the Linear API with SQLite-backed sync, search, and cross-entity queries. Capabilities include: analytics, attachments, audit-entry-types, auth-resolver-responses, authentication-session-responses, bottleneck, custom-views, customer-statuses, customer-tiers, customers, cycles, documents, email-intake-addresses, entity-external-links, favorites, feedback, initiative-to-projects, initiatives, integration-templates, integrations, integrations-settingses, issue-labels, issue-priority-values, issue-relations, issue-to-releases, issues, me, organization-invites, organization-metas, organizations, profile, project-labels, project-milestones, project-relations, project-statuses, projects, release-pipelines, release-stages, releases, roadmap-to-projects, roadmaps, similar, tail, team-memberships, teams, templates, today, user-settingses, users, velocity, workflow-states, workload. Trigger phrases: 'install linear', 'use linear', 'run linear', 'Linear commands', 'setup linear'."
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
---

Invoke the `pp-linear` skill with the user's arguments: $ARGUMENTS

If the user passes no arguments or just `help`, show `linear-pp-cli --help`. If the user's arguments start with `install`, follow the skill's install flow. Otherwise, map the arguments to the best `linear-pp-cli` command and execute.
