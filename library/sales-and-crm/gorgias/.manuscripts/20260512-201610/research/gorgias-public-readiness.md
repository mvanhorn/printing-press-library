# Gorgias Public Readiness Notes

## Scope

Gorgias is a customer support and helpdesk API used by ecommerce support teams. This CLI packages the Gorgias REST surface as a Go CLI plus MCP server with agent-first defaults, local SQLite sync, full-text search, read-only SQL, workload reports, stale-ticket checks, and structured runtime discovery.

## Public Review Position

- Category: `sales-and-crm`, matching the closest current Printing Press Library category for support, CRM, and customer conversation systems.
- Auth: HTTP Basic with `GORGIAS_USERNAME`, `GORGIAS_API_KEY`, and `GORGIAS_BASE_URL`.
- MCP: 108 endpoint definitions exposed through a compact code-orchestration surface rather than one tool per endpoint.
- Local data: synced resources are stored under XDG data paths; SQL is restricted to read-only `SELECT` and `WITH` statements.

## Sanitization Policy

This public manuscript intentionally excludes tenant names, credential-state claims, customer records, ticket contents, and live-data screenshots. Reviewers should use their own sandbox tenant for live checks.

## Known Review Focus

- Confirm the `sales-and-crm` category is acceptable for a helpdesk API in the current catalog taxonomy.
- Confirm post-merge generated registry and `cli-skills` mirrors pick up the package correctly.
- Re-run live checks only with reviewer-owned sandbox credentials.
