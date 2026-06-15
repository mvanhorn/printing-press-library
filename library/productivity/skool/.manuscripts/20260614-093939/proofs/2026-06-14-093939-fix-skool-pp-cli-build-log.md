# skool CLI — Build Log

## Generated (Phase 2)
- Native OpenAPI 3.1 (reconstructed from official docs) → sessions, posts, chats, webhooks endpoint commands; store/sync/search/analytics; MCP cobratree mirror; doctor; agent-context.
- Auth wired: `x-api-secret` header from `SKOOL_API_SECRET`.

## Hand-built (Phase 3)
New package `internal/skoolx` (survives regen): extended config (`~/.skool/config.env` + env), session cache (`~/.skool/session.json`), Apify actor client (cookie login cache, retry/backoff, structured-error envelope). Test file included.

New command groups (`internal/cli/skool_*.go`, registered via root.go AddCommand — regen-merge re-injects):
- `session` create/status/refresh/delete — poll-to-active, cache, auto-inject session_id, auto-refresh.
- `config` set/show — `~/.skool/config.env`, secrets masked.
- `post` list (+`--unanswered`), unanswered (Apify posts:filter, `--older-than`→until), draft (markdown→Apify posts:create, `--schedule`→local queue), cadence (Apify posts:list bucketed by ISO week).
- `chat` inbox (native + local read-state unread inference), reply (send+mark read), read-all, waiting (transcendence triage).
- `member` list/`--pending`, approve, reject, approve-all (pending→batchApprove), ban `--reason`, export (CSV), funnel (local snapshot time series).
- `webhook` list, create, delete, watch (real local HTTP listener, auto-register+cleanup).
- `group status` (Apify groups:get + recent-post counts), `autodm set` (groups:setAutoDM, 300-char validation).
- `course publish <folder/>` (classroom createCourse/createFolder/createPage/setBody from markdown tree).
- `digest today` (cross-source: unanswered posts + pending members + unread chats, degrades per available auth).
- `sql` (SELECT-only over local mirror).

Verify-friendly: every hand command validates in RunE (no MarkFlagRequired/MinimumNArgs), short-circuits on `dryRunOK`, and side-effect commands guard on `cliutil.IsVerifyEnv()`/`IsDogfoodEnv()`.

## Intentionally deferred / changed
- **`chat response-time`** (was in manifest): DROPPED — native `MessageOut` has no timestamp, so reply latency is uncomputable. Removed from research.json so it isn't claimed.
- **`group set-description`** (absorbed row 26): OMITTED — the Apify actions reference only confirms `groups:get` and `groups:setAutoDM`; no confirmed description-update action. Avoided shipping a guessed action name that could 404.
- Native config path stays `~/.config/skool-pp-cli/config.toml` (generated); extended skool config uses `~/.skool/config.env` (dotenv) — YAML support deferred to avoid adding a dependency.

## Reachability
- `api.skoolapi.com` 503/Cloudflare to anonymous probes (auth-edge artifact). No creds available → live testing skipped; verified via build + dry-run + mock.

## Build status
- `go build ./...` exit 0; `go vet ./...` clean; `go test ./internal/skoolx/` pass. All 25+ hand commands resolve `--help` and `--dry-run` (exit 0).
