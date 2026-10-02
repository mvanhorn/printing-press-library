# Drive Plaza local code review

Autofix summary: 10 initial code/documentation findings plus one metadata wording finding fixed across three review rounds; see `evidence/MAX-REVIEW.md`, `evidence/implementation.diff`, and `.printing-press-patches/driveplaza-domain-contract.json`. No implementation commits or publication were required.

Convergence: initial findings cleared at round 2; metadata-only polish recheck cleared at round 3; final Skill, Docs, Output and Code verdicts PASS.

Review path: direct dispatch to exactly one fresh-context gpt-6.1-sol MAX reviewer as the user requested. Correctness, security, reliability, performance, skill, documentation and actual-output checks were combined in that independent review. No extra coordinator or reviewer agents; no codex exec. The reviewer made no implementation edits.

Native timeout boundary: live domain commands run through dpRun/boundCtx; companion references use flags.newClient and the client hook, with bounded HTTP reads. Production MCP reference handlers invoke the parsed CLI with a 30-second context. Dry-run precedes provider I/O.

Template retro candidates: `internal/mcp/tools.go` generic HTML handlers bypass page extraction and generic context advertises cursor/sync and parent-only novel commands; `internal/client/client.go` generic HTML reads are unbounded. This print corrects shipping paths through separate, regeneration-preserved client hooks and MCP overrides. No generator-reserved cliutil/cobratree edits. The generated generic defaults remain generator improvement candidates rather than in-scope unresolved defects.

Surface-to-user findings: none; fixes preserved approved scope. Simplify was not run because it is Claude Code-only.

Metadata-only polish recheck: same reviewer, round 3; final all-PASS outcome recorded in independent-MAX-review.md.
