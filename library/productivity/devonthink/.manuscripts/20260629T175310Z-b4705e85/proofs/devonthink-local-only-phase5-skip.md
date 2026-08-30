# DEVONthink Phase 5 Skip

- Decision: skip live public dogfood.
- Reason: DEVONthink runs only on this local Mac or the user's own LAN, and live results would expose personal database names or record metadata in public publish artifacts.
- Contract: the CLI remains local-first and wraps the official local DEVONthink MCP/automation surface; Smart Groups are search scopes only, not workflow policy.
- Verification still run in this session: `go test ./...`, `go build ./cmd/devonthink-pp-cli`, `devonthink-pp-cli records search --help`, and `cli-printing-press publish validate`.
