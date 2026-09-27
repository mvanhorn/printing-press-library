# Local code review

Review path: root Astra direct review, as explicitly required by the user; correctness, security, maintainability, API contract, reliability and performance assessed. No additional orchestration/review children.

0new findings required autofix in this phase; prior root-review fixes and final source-window correction were verified before entry. Findings cleared at round1.

Native timeout: the sole handwritten CLI file planning.go applies boundCtx with validated <=30s timeout before every sibling planner operation. Context flows through HTTP, rate-limit waits, retries and scan workers. Inputs bound geographic search/party/date/limit; request attempts and response bytes are capped; redirects refused. Calendar schema and timezone checked; unknown source modes retained, false not inferred soldout, cache errors not inventory. Amounts remain decimal text; unknown fees staynull. Partial scans preserve every requested row and report errors. Tests include budget/concurrency/cache bounds.

Generated clients/MCP/store/entrypoints reviewed at the relevant seams: read-only source methods and hint annotations, SQL readonly connection, native CLI exit-code mapping, stdout data versus stderr errors. Planner does not use the generated FTS store. Generated MCP scaffolding is unconfigured; local delivery changes no host configuration.

Template retro candidates: helpers.go's unused handleBinaryResponseDelivery/readSecretFromStdin/successfulNoop are emitted for capabilities outside this focused no-auth/no-binary CLI. Empty defaultSyncResources is intentional; broad sync is excluded. These are structural warnings, not planning data-path bugs. Generated doc sync inserts an unsupported exclusivity claim; local docs remove it after synchronization. No shared generator files modified or issue/PR sent.

No unresolved source correctness/security finding. /simplify skipped because this is Codex; no custom substitute required.
