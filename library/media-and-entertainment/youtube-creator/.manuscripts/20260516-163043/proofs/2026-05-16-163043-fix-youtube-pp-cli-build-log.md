# YouTube CLI Build Log

## Generated (Phase 2)
- printing-press v4.8.0
- 3 specs merged: youtube/v3 (39 paths, 76 ops), youtubeAnalytics/v2 (3 paths, 8 ops), youtubereporting/v1 (6 paths, 8 ops)
- Total: 92 endpoint commands
- Output: `~/printing-press/.runstate/starhire-91f0fa6c/runs/20260516-163043/working/youtube-pp-cli/`
- Module: `youtube-pp-cli`
- Resource conflicts: `youtube-reporting/jobs` renamed to `youtube-reporting-jobs` to avoid framework `jobs` shadow
- Generator quality gates: ALL PASS (go mod tidy, govulncheck, go vet, go build, runnable binary)

## Built (Phase 3)
13 transcendence features as hand-authored commands:

### Files (all under internal/cli/)
- `quota.go` — `quota meter|costs|reset`, per-call log at ~/.config/youtube-pp-cli/quota.json
- `pubsub.go` — `pubsub subscribe|unsubscribe|verify` (PubSubHubbub WebSub, hand-built POST)
- `mod.go` — `mod queue|approve|reject|auto` (YAML rules, batch setModerationStatus, --ban-author)
- `digest.go` — `digest analytics|video` (Markdown + JSON output, multi-query Analytics API)
- `bulk_metadata.go` — `bulk metadata` (cheap playlistItems enumeration, predicate filtering, mutations)
- `playlist_hygiene.go` — `playlist hygiene` (YAML pattern→playlist auto-add, reorder-by-views)
- `backup.go` — `backup` (yt-dlp subprocess wrap, captions/thumbnails/info-json)
- `ab_thumbnails.go` — `ab thumbnails start|rotate|report|list|stop` (state at ~/.config/youtube-pp-cli/ab-thumbnails.json, z-test significance)
- `reporting_sync.go` — `reporting sync` (idempotent jobs-create + reports-list + CSV download)
- `chapters.go` — `chapters auto` (yt-dlp captions + LLM hook + write back to description)
- `recipes.go` — `recipes n8n list|print` (6 ready-to-paste n8n workflow JSON snippets)

### Wired in root.go
All 11 hand-authored parent commands AddCommand'd into rootCmd.

### Dependency added
- `gopkg.in/yaml.v3` for rules YAML parsing (mod auto, playlist hygiene)

## Intentionally Deferred (not in scope this run)
- **InnerTube-backed features**: community posts CRUD, pin-comment, heart-comment, Studio drafts. These require the unofficial youtubei.js Node library — planned as separate TS sibling project.
- **Studio scraping**: A/B test verdict reading, blocked-words list, channel handle change, Studio Editor. Out of scope per user briefing.
- **Real LLM provider integration in chapters auto**: stubbed with heuristic preview; real API calls to Claude/OpenAI would require an additional HTTP client and prompt template — provider hook is in place via env vars.
- **MCP token-efficiency Cloudflare-pattern enrichment**: 92 endpoint tools currently emitted; scorecard mcp_token_efficiency=4/10. Would require regen with x-mcp:{orchestration:code, endpoint_tools:hidden} which would clobber Phase 3 work. Acceptable trade-off.

## Generator Limitations Found
- Google's apis-guru OpenAPI uses `Oauth2` (implicit flow) + `Oauth2c` (auth-code flow) security schemes; the generator picked the auth-code flow correctly and emitted a working localhost-callback OAuth login command. Solid.
- 92 endpoint tools triggers the >50 MCP warning. Future improvement.
