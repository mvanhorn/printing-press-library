<!-- slop-gate: off -->
# Printing Press retro candidates from the Postmark run

Each item was worked around in the printed CLI (spec or hand file); the root cause is in the machine.

1. Per-operation credential selection (P1 for split-token APIs). Composed sibling apiKey headers go on every request; no way to route by operation security. Postmark GET /server returns 401 ErrorCode 10 when both tokens are sent. Workaround: transport hook routing headers by path. User approved filing this as an issue.
2. singleArrayPropertyRef treats any object response with exactly one typed array as a list envelope (internal/openapi/parser.go). GET /server printed only ApiTokens, i.e. the server token. Workaround: drop items from non-envelope arrays in the spec.
3. Offset-style sync omits the offset param on the first page; spec defaults for paging keys are skipped. APIs that require offset (Postmark: "Parameter 'offset' is required") fail every list. Workaround: transport adds offset=0.
4. Since-param detection (isEndpointSinceParamName) misses `fromdate`/`todate`; `sync --since` silently ignored.
5. OpenAPI specs cannot declare extra_commands (internal YAML only), so hand-built commands never reach SKILL/README Command Reference. Workaround: hand-appended "Hand-written Extensions" section.
6. Integer path-param examples are dropped from generated Examples/happy-args; string examples work. Workaround: typed ID path params as strings.
7. agent_context.go ignores x-auth-vars descriptions ("Set to your API credential." for every var); README env table and MCP manual config list only the primary scheme's env var.
8. Regen merge preserves stale root.go Short and .printing-press.json description after narrative.headline changes (description drift until hand-edited).
9. Novel-host guard artifact roots expect <research-dir>/../research while the skill passes --research-dir $API_RUN_DIR (research lives at $API_RUN_DIR/research); placeholder emails/RFC 2606 example domains in novel examples are rejected.
10. Generated `sync --full` prunes across the whole resources table; multi-tenant/multi-server stores lose other tenants' rows. Workaround: default --no-prune in a pre-run hook.
11. Runtime verify mock returns a bare JSON array for any path ending in "s"; envelope-shaped APIs fail EXEC cells.
12. Template text issues in README/SKILL/AGENTS: cookie/browser-session wording on API-key CLIs, garbled "--idempotent to create retries" sentence in README, `<command>` placeholders in executable code blocks, "aren't available in any other tool" claim, --dry-run described as showing the request for hand-written commands.
13. Generated .golangci.yml lacks a v2 `version` key; unused generated helper handleBinaryResponseDelivery emitted when no binary endpoints exist.
14. Generated endpoint send commands (POST /email) deliver immediately; no print-by-default option for side-effect endpoints declared in the spec. Workaround: --send gate hook.
15. rsync of a module copy with `--exclude <binary-name>` also excludes cmd/<binary-name>/ (operator note, not a machine bug).
