# QuickBooks Online (QBO) Absorb Manifest

- Keep official API constraints in mind: rate limits, OAuth 2.0 access token rotation, and restricted QBO SQL.
- Preserve Printing Press differentiators: `--agent`, MCP exposure, structured JSON output, local SQLite cache, and custom subcommands (`changed`, `duplicates`, `reconcile`, `net-worth`).
- Treat financial mutations as high-risk. Local dry-run checks and read-only local mirrors are preferred unless authorized sandboxes are explicitly targeted.
