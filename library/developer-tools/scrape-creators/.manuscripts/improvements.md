# PR #113 Quality Improvements

Status: IMPLEMENTED in this PR. All four axes ship as additive changes on top of PR #113.

## 1. Platform action command naming

92 leaf commands renamed from OpenAPI operation IDs to `platform action` pairs matching v1's `@scrapecreators/cli` convention.

Before:
```
scrape-creators-pp-cli tiktok list-profile --handle charlidamelio
scrape-creators-pp-cli instagram list-user-5 --user-id 123
scrape-creators-pp-cli facebook list-adlibrary-3 --query nike
```

After:
```
scrape-creators-pp-cli tiktok profile --handle charlidamelio
scrape-creators-pp-cli instagram user-posts --user-id 123
scrape-creators-pp-cli facebook adlibrary-search-companies --query nike
```

Old operation-ID names remain callable via hidden cobra aliases. Every v1 command from v1's `src/command-registry.js` now resolves to the same endpoint in this CLI.

See `action-map-proposal.md` for the full 115-endpoint map.

Not yet split: the 23 `promoted_<platform>.go` dual-role parents still carry their own RunE shortcut. `scrape-creators-pp-cli tiktok --help` now lists clean action names in the Available Commands block but the parent also shows shortcut flags. Splitting the dual-role parents is a follow-up.

## 2. Interactive wizard on bare invocation

`internal/cli/wizard.go`. Bare invocation in a TTY walks platform -> action -> required params, then executes the resolved command. Non-TTY stdin, `--no-input`, `--agent`, and `--yes` all fall through to help.

Stdin-based, no TUI dependency. Binary size unchanged.

## 3. `agent add` auto-wiring

`internal/cli/agent.go`. `scrape-creators-pp-cli agent add cursor|claude-desktop|claude-code|codex` writes a valid MCP server entry into the target's config file:

- Cursor: `~/.cursor/mcp.json` (JSON mcpServers)
- Claude Desktop: platform-correct path (macOS / Windows / Linux; JSON mcpServers)
- Claude Code: `~/.claude.json` (JSON mcpServers; no `claude` CLI shell-out)
- Codex: `~/.codex/config.toml` (TOML `[mcp_servers.*]`)

`--hosted` writes the `api.scrapecreators.com/mcp` URL with an `x-api-key` header instead of the local `scrape-creators-pp-mcp` stdio binary. `--force` overrides the existing-entry refusal, which prints a diff by default. Every write enforces mode 0600 on the target file. Parent directories are created at 0700 if missing.

## 4. Client-side input normalization

`internal/cli/input_normalize.go`. Two helpers: `NormalizeHandle` strips a single leading `@` and trims whitespace; `NormalizeHashtag` does the same with `#`. Applied in every leaf that accepts a handle or hashtag parameter (26 files). Both idempotent.

`scrape-creators-pp-cli tiktok profile --handle @charlidamelio` and `--handle charlidamelio` now produce identical requests. README surfaces the rule up front so users learn about it before hitting an API-tolerance edge case.

## v1 fact verification (R6, prior plan)

PR #113's `internal/config/config.go` already accepts both env var names: `SCRAPE_CREATORS_API_KEY_AUTH` (primary) and `SCRAPECREATORS_API_KEY` (v1-compat fallback, line 58-59). No migration needed. Config path differs from v1 but migration is out of scope here; users on v1 re-enter the key once on first v2 run.

## Ordering

1. This PR lands on top of PR #113. Review once naming is agreed.
2. After merge, follow-up PRs split the dual-role `promoted_<platform>.go` parents (cleaner `tiktok --help`), add wizard TUI polish, and port additional tests.

## Out of scope here

- Binary rename from `scrape-creators-pp-cli` to `scrapecreators`.
- npm distribution via `@scrapecreators/cli`.
- `curl | sh` installer.
- Retiring v1 or transferring repo home.

Adoption / handoff work Adrian owns when he is ready to port this into `@scrapecreators/cli`.

## Companion plan

`docs/plans/2026-04-23-002-feat-pr-113-library-quality-plan.md` on `main` has the full plan, risk register, and test scenarios.
