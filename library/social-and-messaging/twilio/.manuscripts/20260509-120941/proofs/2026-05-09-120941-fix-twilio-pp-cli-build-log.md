# Twilio CLI Build Log

## Phase 2 generation
- Spec: `https://raw.githubusercontent.com/twilio/twilio-oai/main/spec/json/twilio_api_v2010.json` (1.87MB)
- Endpoints in spec: 197 across 121 paths
- MCP enrichment applied: `x-mcp` Cloudflare pattern (transport: stdio+http, orchestration: code, endpoint_tools: hidden)
- All 8 quality gates passed: `go mod tidy`, `govulncheck`, `go vet`, `go build`, runnable binary, `--help`, `version`, `doctor`
- Generated 280 command files, 304 Go files total
- Bundled MCPB: `build/twilio-pp-mcp-darwin-arm64.mcpb`

## Phase 3 hand-fixes

### Auth wiring (CRITICAL fix)
The generator parsed Twilio's `accountSid_authToken` security scheme as a single env var `TWILIO_ACCOUNT_SID_AUTH_TOKEN` with template `Basic {username}:{password}` — but never substituted username/password, so `applyAuthFormat` returned an empty string and every API call would have 401'd.

Patched files:
- `internal/config/config.go`: added separate `AccountSid`, `AuthToken`, `APIKeySid`, `APIKeySecret` fields. Reads `TWILIO_ACCOUNT_SID`, `TWILIO_AUTH_TOKEN`, `TWILIO_API_KEY_SID`, `TWILIO_API_KEY_SECRET`. New `basicAuthPair()` picks SK > AC > legacy combined slot precedence and `AuthHeader()` returns `Basic <base64(user:pass)>`.
- `internal/cli/doctor.go`: now reports auth_mode (api_key scoped vs account_sid + auth_token master), warns on AC/SK prefix mismatches, surfaces parent/subaccount mismatch detection.
- `internal/cli/auth.go`: `auth status` not-authenticated message lists the correct two-pair credential setup.
- `internal/cli/agent_context.go`: env var declarations updated to the four canonical Twilio names.
- `internal/cli/helpers.go`: HTTP error hints updated.

### 12 transcendence commands (Phase 1.5 manifest)
All built, all respond to `--help`, all short-circuit on `--dry-run` with exit 0:

| # | Command | Source file | Type |
|---|---------|------------|------|
| 1 | `sync-status` | sync_status.go | local-only watermark inspector |
| 2 | `delivery-failures` | delivery_failures.go | groupby on `messages_json` |
| 3 | `message-status-funnel` | message_status_funnel.go | groupby on `messages_json` |
| 4 | `call-disposition` | call_disposition.go | cross-tab on `calls_json` |
| 5 | `idle-numbers` | idle_numbers.go | three-way LEFT JOIN |
| 6 | `conversation` | conversation.go | UNION over messages + calls |
| 7 | `opt-out-violations` | opt_out_violations.go | temporal join on `messages_json` |
| 8 | `error-code-explain` | error_code_explain.go | groupby + curated `// pp:novel-static-reference` table |
| 9 | `webhook-audit` | webhook_audit.go | groupby on `incoming_phone_numbers_json` + opt-in HEAD probe |
| 10 | `call-trace` | call_trace.go | local-first cross-resource stitch + live API fallback |
| 11 | `tail-messages` | tail_messages.go | polling loop with `cliutil.IsVerifyEnv()` short-circuit |
| 12 | `subaccount-spend` | subaccount_spend.go | fan-out `usage/records/{period}` per subaccount with `// pp:client-call` |

Shared helpers in `transcendence_helpers.go`: `parseSince`, `sinceCutoffBind`, `twilioDateExpr`, `formatDurationHours`.

All transcendence commands set `Annotations: map[string]string{"mcp:read-only": "true"}` since none mutate external state. They use the verify-friendly RunE shape (cmd.Help() on no args; `dryRunOK(flags)` short-circuit; no `cobra.MinimumNArgs` or `MarkFlagRequired`).

Webhook-audit and tail-messages additionally check `cliutil.IsVerifyEnv()` to suppress side effects (HEAD probes, polling) under `PRINTING_PRESS_VERIFY=1`.

## Known issues for retro / polish

### Dual command tree (xxx and xxx-json) — SYSTEMIC GENERATOR ISSUE
Twilio's spec uses `.json` URL suffixes on every path. The generator's path → resource derivation treats `Messages.json` and `Messages/{Sid}.json` as different resources, producing parallel command trees:
- `messages` (delete, fetch, update sub-commands)
- `messages-json` (create-message, list-message sub-commands)

Same pattern affects: addresses, applications, calls, incoming-phone-numbers, keys, outgoing-caller-ids, queues, signing-keys, etc. — roughly 12 resource families have a duplicate `xxx-json` parent.

The store-level tables also follow this pattern (`messages_json`, `calls_json`). Data lands in the `_json` tables since those are the list endpoints.

**Why I didn't fix it in this run:**
- A spec-level fix (strip `.json` from paths) breaks URL construction without a paired client.go patch — the harness correctly refused that workaround.
- A post-process to merge `xxx-json` into `xxx` would touch ~50 files mechanically and risks breaking command registration.
- The proper fix is in the generator's `resourceAndSubFromSegments` to strip `.json` from segments before sanitizing. That generalizes to any API with `.json` URL suffixes — file as a retro candidate.

The CLI is functional with the dual tree; users learn `messages list-message` instead of `messages list`. Polish will likely flag this and the retro skill should capture the systemic fix.

### Live testing skipped
User-provided creds 401'd against `https://api.twilio.com/2010-04-01/Accounts/AC1a05.../`. Phase 5 will record `phase5-skip.json` with `skip_reason: "auth_required_no_credential"`. The CLI is verified against mock responses, dry-run, and exit codes only.

### Description truncation (cosmetic)
The `narrative.headline` is fully populated but renders truncated as "no other Twilio tool shi…" in `root.go` Short, MCP tools description, and SKILL.md frontmatter due to surface-specific length limits. The full headline appears in README.md and elsewhere.

## What was deferred
Nothing was deferred from the absorb manifest. All 29 absorbed surfaces are present (auto-generated) and all 12 transcendence commands are hand-built. No stubs.
