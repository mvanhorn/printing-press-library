# Polish pass 2026-07-11 — reprint guards

- `internal/cli/contacts_verify_line_type.go`, `internal/cli/drips_audit.go`:
  local-store queries against the `resources` table must filter on
  `resource_type`, not `type` (the store schema has no `type` column; the
  original prints failed at runtime with "no such column: type").
  `drips_audit.go` also initializes its result slice so empty audits emit
  `"doubleSends": []` instead of `null`.
- All five hand-written novel command files carry a `// pp:data-source`
  annotation (`live` for send-check / imports blame / imports watch, `auto`
  for contacts verify-line-type, `local` for drips audit) — required by
  dogfood's reimplementation check.
- Description surfaces (root.Short/Long, `.printing-press.json`,
  `manifest.json`, `tools-manifest.json`, `internal/mcp/tools.go`,
  `internal/cli/agent_context.go`, `.goreleaser.yaml`) use the research.json
  narrative headline "Full-coverage Conduyt CLI: …" — not "The official
  Conduyt CLI: …" — to satisfy dogfood's description-drift check.
