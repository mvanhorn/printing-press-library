# Local code review

13 findings autofixed in-place across two builder fix rounds; all findings cleared at reviewer round3. This local artifact has no repository commit baseline; authoritative before/final source hashes, independent repro logs and the durability patch record identify the verified implementation.

Review path: direct subagent dispatch, one explicitly required fresh-context gpt-6.1-sol MAX reviewer covering correctness, security, maintainability and CLI/MCP/doc contracts. No additional agents, codex exec or Claude review process.

## Template retro candidates

- internal/client/client.go:1306 (P2): generic generated reference-body read has no plain-body cap and a much larger decoded cap. API-specific public transport hook now bounds actual wire bytes and every decoded layer. Emitter: internal/generator/templates/client.go.tmpl (matching local Press source).
- internal/client/client.go:278 (P2): generic auto-rate policy has no domain ceiling. API-specific constructor hook now enforces the public2RPS budget while retaining smaller caller ceilings. Emitter: internal/generator/templates/client.go.tmpl; generator-reserved adaptive limiter is untouched.
- internal/mcp/tools.go:964 and SQL registration (P3): generic context/SQL prose assumes sync/search and cursor paging for a stateless HTML target. API-specific context override and explicit SQL disclosure fix this build; internal/generator/templates/mcp_tools.go.tmpl should use actual capability metadata.
- README.md/SKILL.md Unique section intro (P3): generic exclusive-capability assertion is unsupported. Emitters: internal/generator/templates/readme.md.tmpl and skill.md.tmpl. This build uses concrete compact sourced-output wording.
- Gosec generated/reserved31 findings: per-rule/file/line details and ownership in gosec-triage.json. None is unresolved hand-authored planner/transport/context code. The reserved testenv directory-permission and generated local migration-path findings are included even though those files omit a Generated header.

No pending surface-to-user finding or scope tradeoff. No change in reserved internal/cliutil or internal/mcp/cobratree. Convergence: PASS at round3, confirmed in max-review-final.md. Claude-only simplify is inapplicable to this harness.
