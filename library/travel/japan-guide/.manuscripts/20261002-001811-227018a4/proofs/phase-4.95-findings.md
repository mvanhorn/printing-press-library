# Local code review

Autofix summary: Nine application/documentation findings fixed in-place across three review rounds; no Git repository existed. See reviewed-source-hashes.json and the hand-authored source/tests.

Convergence outcome: Findings cleared at round 3.

Review path: one dedicated fresh-context gpt-6.1-sol MAX reviewer combining correctness, security, maintainability, semantic/docs/output review and every-tool judgment, as explicitly requested.

## Template and reserved retro candidates


These are generator-shaped findings, separate from application convergence. The reserved package must not receive a local patch.

### R1 — P2: structured variadic MCP arguments become one scalar argv

**Target:** `internal/mcp/cobratree/shellout.go:111`–141; schema emitted through `internal/mcp/cobratree/typemap.go` and walker.

The live `guide_compare` schema exposes `pages` as a string. `positionalArgsFromMCP` appends that complete string once and does not honor `positional.Variadic`.

**Reproduction:** tools/call with name `guide_compare`, arguments `{"pages":"e3001 e3002","agent":true,"timeout":"15s"}` returns an MCP error wrapping CLI exit 2 and the canonical source-ID usage error. The control `pages:"e3001"` succeeds with requested=successful=1. Thus the exposed multi-page comparison cannot be expressed through its natural advertised string field. An array also does not match the emitted schema.

**Upstream fix:** emit and correctly expand a variadic array, or explicitly tokenize a documented variadic string consistently with raw args, while retaining positional security validation. The origin is the shared runtime Cobra MCP mirror; exact generator template path was not locally located. Filed as retro because cobratree is reserved.

**Permitted local workaround:** normalize one whitespace-separated ID argument inside the hand-authored compare command, then enforce the 1..5 bound and canonical validation before fetching. This supports the existing MCP contract without modifying the reserved mirror. Validate two IDs and rejection of six through the actual MCP tool.

### R2 — P3: universal exclusivity claim is unsupported

**Target:** `README.md:97` and `SKILL.md:48`.

Both documents assert that the listed capabilities are unavailable in any other tool for this API. The research/manifest documents source extraction and implementation choices, not a comparison proving universal exclusivity. This identical generated default is an unverifiable marketing claim.

**Upstream fix:** replace the generator's Unique Features/Unique Capabilities preamble with a concrete description such as capabilities added by this CLI. The exact template path was not locally located; the repeated generated narrative shape identifies the generator origin. Source feature descriptions and their five verified entries are otherwise aligned. A durable narrative override/removal may keep these local documents truthful while the template fix is filed.


### R3 — SQL description assumes sync

P3, internal/mcp/tools.go:47. Literal origin: cli-printing-press/internal/generator/templates/mcp_tools.go.tmpl:153. It says Requires sync first although no sync command is emitted and guide snapshots are separate JSON files. No verified framework override/toggle exists; generated registration remains untouched. README/SKILL/AGENTS direct page facts to guide commands and explain the local SQL scope.

### Generated static analysis

31 generated/foundation findings remain separate from hand-authored convergence; see generated-gosec-retro.json for exact paths, rules and lines. Their template-shaped origins and reserved helper scope prevent local template patches. Domain/CLI gosec findings are zero after fixes. No surface-to-user application tradeoff or scope shrink remains.
