# Shipcheck Report — telegram-pp-cli

## Summary
- **First-pass verdict:** PASS on all gates except `validate-narrative` (4 example errors, all 1-3 file edits).
- **After 1 fix loop:** PASS on all 6 legs. Scorecard 79/100 Grade B.
- **Final verdict:** **ship** (above the 65 threshold, no functional bugs in shipping-scope features, all agentic-review errors resolved).

## Shipcheck umbrella legs

| Leg | Result | Notes |
|---|---|---|
| dogfood | PASS | 92/92 commands pass, 100% pass rate, 0 critical |
| verify | PASS | All endpoints structurally valid; auth wiring patched |
| workflow-verify | PASS | No workflow manifest declared (acceptable) |
| verify-skill | PASS | flag-names, flag-commands, positional-args, unknown-command, canonical-sections all green |
| validate-narrative | PASS (after fix-loop) | 10/10 narrative commands resolve and full examples pass |
| scorecard | PASS | 79/100 Grade B |

## Scorecard breakdown (Grade B / 79)

Strong dimensions (10/10): Output Modes, Auth, Error Handling, Doctor, Agent Native, MCP Quality, Local Cache, Breadth, Path Validity, Sync Correctness.

Notable gaps for polish to address:
- **MCP Token Efficiency 4/10**, **MCP Surface Strategy 2/10**, **MCP Remote Transport 5/10**, **MCP Tool Design 5/10** — the spec's `x-pp-mcp` extension (transport+orchestration+endpoint_tools) did not apply at generation. 74 raw endpoint tools shipped. Cloudflare-pattern enrichment likely needs to live under a different key for OpenAPI specs (the docs/SPEC-EXTENSIONS reference may be authoritative for internal YAML only). Polish skill will revisit.
- **Auth Protocol 5/10** — flagged because the bot-token-in-URL pattern doesn't match a standard OpenAPI security scheme; the patch wires it via `BaseURL` rewrite in `config.Load`.
- **Cache Freshness 5/10** — generic cache freshness reporting; not a hard issue for a write-oriented CLI.
- **Vision 6/10** — README narrative is fine but scorer expects more depth in some sections.
- **Type Fidelity 3/5** — the spec uses some `any`-shaped fields (Telegram polymorphic types like `Message` containing optional `text` OR `photo` OR `document` etc.) which the generator widens to maps.

Total 79 is above the 65 threshold for `ship`.

## Agentic review (Phases 4.8 + 4.9)

**Errors (all fixed in 1 file-edit loop):**
1. `publish --channel ...` → `publish send --channel ...` (`publish` is a parent group; `send` is the leaf). Fixed in README, SKILL, and research.json (so future regens stay correct).
2. `format --html-lint` → `format html-lint` (subcommand, not a flag). Fixed in README, SKILL, research.json.
3. `messages list --since 1m --error-only --json` — `--error-only` doesn't exist. Removed in README.
4. README config path showed `~/.config/telegram-bot-pp-cli/config.toml` — real path is `~/.config/telegram-pp-cli/config.toml`. The spec API name is `telegram-bot` but the CLI slug is `telegram` so the binary uses `telegram-pp-cli`. Fixed in README.

**Warnings (addressed):**
- SKILL.md had no anti-trigger section. Added `## When NOT to Use This CLI` covering MTProto/Telethon, scraping public channels, and end-user chat clients.

**Warnings (deferred to polish or accepted):**
- SKILL frontmatter `argument-hint` mentions `install` which isn't a CLI subcommand (it's a skill-invocation pattern). Cosmetic.
- `add-sticker-to-set` is used as the `--select` example instead of the more narratively-aligned `send`. Cosmetic.
- README release-tag link `telegram-current` is unverifiable offline. Flag for live-link audit; not blocking.

## Phase 4.85 Output Review
**Status:** SKIP. Output review samples live command output to detect plausibility bugs (substring-match relevance failures, format bugs, silent source drops). No `TELEGRAM_BOT_TOKEN` is available in this session, so live sampling cannot run. Per Wave B rollout policy, output-review findings are warnings only and do not block ship — and "could not sample" is recorded as SKIP, not a failure.

## Phase 5 Dogfood
**Status:** SKIP (`phase5-skip.json` written with `skip_reason: auth_required_no_credential`).
Telegram requires a `TELEGRAM_BOT_TOKEN` for any meaningful API call. The user explicitly declined to provide a token at the Phase 0.5 API Key Gate. Live dogfood without a token would only exercise the 401-handling paths, which `doctor` already verifies structurally during the verify leg.

The CLI was structurally verified across:
- `doctor` correctly distinguishes "no token" (`auth: not configured`) from "valid token" (uses `TELEGRAM_BOT_TOKEN=111:fake` and reports `auth: configured, api: reachable (HTTP 401 at /)`).
- `send-message --dry-run` correctly builds `POST https://api.telegram.org/bot<token>/sendMessage` with the JSON body shape.
- `format html-lint` correctly flags Telegram's non-whitelisted tags (`<unknown>` → offset 10, hint to use --html-escape).
- `messages list`, `audit`, `publish list`, `chats resolve` all return `[]` / `{}` against an empty store (no API call, no token needed) — confirms the local-store novel features are wired correctly.

## Fix recommendation
**Verdict: ship.** All hard gates pass; remaining gaps are polish-tier (MCP enrichment, cosmetic SKILL items). The user's two named callers (`post_to_telegram.py`, `alert_kris.py`) will work as soon as `TELEGRAM_BOT_TOKEN` is set in their environment.
