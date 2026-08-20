# Amazon Ads Local Validation Proof

Run ID: `20260527-233111`

Local validation completed against the generated CLI tree after Amazon Ads auth/profile, analytics, report-normalization, MCP, and automation-safety customizations.

Passing local gates:

- `jq . .printing-press.json`
- `jq . .printing-press-patches.json`
- `go test ./...`
- `go vet ./...`
- `go build ./cmd/amazon-ads-pp-cli ./cmd/amazon-ads-pp-mcp`
- `govulncheck ./...`
- `go run ./cmd/amazon-ads-pp-cli --help`
- `go run ./cmd/amazon-ads-pp-cli version`
- `python3 .../verify-skill/verify_skill.py --dir . --json`
- Simulated public-library `verify_publish_package.py` new-CLI check
- Simulated public-library `generate-registry --validate amazon-ads` check, with Amazon-specific catalog description/search terms and MCP metadata preserved
- Simulated public-library `verify_binary_payload.py` check
- Simulated public-library supply-chain scan
- MCP stdio smoke: JSON-RPC `initialize` succeeds, `tools/list` returns 53 runtime tools, and the tool list includes `auto_negate`
- `.printing-press.json` MCP counts match the runtime `tools/list` surface: 53 total/public tools. The static `tools-manifest.json` remains endpoint metadata for the code-orchestration search/execute path.
- Local novel-command smoke with temporary sample CSV/TOML inputs: `portfolio-dashboard`, `acos-vs-tacos`, `wasted-spend`, `search-term-mining`, `true-profit`, and `break-even-acos` all returned structured JSON.

Focused proof coverage:

- OAuth setup exposes `auth login`, persists local `.env` credentials, surfaces provider token errors, and auto-selects a single returned profile.
- `auth setup` and unauthenticated `doctor --json` point to `amazon-ads-pp-cli auth login --port 8085` and `http://localhost:8085/callback`.
- Refresh-token failure paths surface provider `error` and `error_description` details.
- `break-even-acos` rejects zero price instead of dividing by zero.
- Seller-store TACOS fallback degrades when the store is missing and reports incompatible schema or malformed JSON rows in notes.
- Report normalization reads gzip-compressed report files and can import normalized rows into the local SQLite store.
- Automation commands dry-run by default, require `--apply` for mutation, enforce ID/max-change/max-bid/max-budget guardrails, suppress duplicates, write audit records, and do not claim remote mutation when `PRINTING_PRESS_VERIFY=1` short-circuits mutating HTTP.
- MCP runtime mirror exposes novel commands that are not present in the static endpoint-only tools manifest, and automation tools are marked destructive/open-world.

Live validation still required:

- `amazon-ads-pp-cli auth login --port 8085`
- `/v2/profiles` profile retrieval with real Login with Amazon credentials
- At least one real scoped Amazon Ads endpoint smoke test using the selected profile
- Novel command smoke tests against real downloaded Amazon Ads reports
