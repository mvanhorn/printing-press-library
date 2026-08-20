# podcast-goat CLI Brief

Pulls full transcripts from long-form podcasts and prints them as speaker-labeled
markdown ready for an agent to summarize. Cookie-first, free-next, paid-last across
ten sources. Six target shows must work end to end.

Source of truth for this CLI: user's brief at
`/Users/mvanhorn/docs/plans/2026-05-17-005-pp-brief-podcast-goat.md`
(also copied into this run as `user-briefing-context.md`).

## API Identity

- Domain: long-form podcast transcripts, normalized to one speaker-labeled markdown shape
- Users: agentic researchers (Lan Xuezhao / Hermes is the canonical user), founders, investors who
  currently copy-paste from member sites into LLMs
- Data profile: episodes (id, url, source, show, host, guests, date, duration, content_md,
  content_jsonl, provider, cost_credits), feeds (rss, source), spend_log (timestamp, provider, cost)

## Reachability Risk

- Low. All six publisher landing pages return `200` to plain `curl` with a desktop User-Agent
  (verified 2026-05-17): dwarkesh.com, hubermanlab.com, acquired.fm, founderspodcast.com,
  peterattiamd.com, spoken.md.
- Per-page transcript surfaces for sources #1-#4 are member-gated and return login redirects
  without the user's cookie; this is expected and is the reason cookie-first dispatch is
  the primary path. Cleared-browser challenges are not anticipated for any source.
- Verified API contracts:
  - **spoken.md** (source #8): `GET https://spoken.md/search?q=...`, `GET https://spoken.md/transcripts/{id}`
    with `x-api-key: pt_demo` (demo) returns `Content-Type: text/markdown` with
    `**Speaker Name** (MM:SS)` shape; `X-Credits-Charged`, `X-Credits-Remaining` headers.
    Demo key explicitly advertised; pricing $0.08-$0.15 per transcript credit.
  - **Taddy** (source #9): GraphQL, requires `X-USER-ID` + `X-API-KEY` headers; query
    `getEpisodeTranscript(uuid)` returns `[ { id, text, speaker, startTimecode, endTimecode } ]`.
    Free tier is "podcast-provided transcripts" only (<1% of podcasts).
  - **Podcasting 2.0** (source #6): `<podcast:transcript url type />` namespace tag inside
    `<item>` blocks; MIME types `text/vtt`, `application/x-subrip`, `text/html`,
    `application/json`, `text/plain`. Specified at podcasting2.org.

## Top Workflows

1. `episode get <url>` — paste a public or member URL, walk the cheapest-first chain, write
   speaker-labeled markdown to `~/.config/podcast-goat/cache/...`, print path to stdout.
2. `magic "<topic>"` — pick N relevant cached episodes (FTS5 match) and bundle them into one
   markdown file with per-episode YAML frontmatter for one-shot agent summarization. This is
   the chip-supply-chain.fly.dev workflow distilled to one command.
3. `feeds add/sync` — subscribe to a show's RSS, pull new episodes' transcripts as they ship.
4. `episode search "<query>"` — FTS5 over the entire local cache, return episode hits with
   speaker-tagged matched lines.
5. `auth login --chrome --service <huberman|acquired|founders|peterattia>` — capture the
   logged-in browser cookie once; subsequent `episode get` invocations use it for free.

## Table Stakes

Every adapter MUST:
- Output the canonical markdown shape (`**Speaker** (MM:SS)\n\nbody\n\n**Other** (MM:SS) ...`)
- Support `--md` (default), `--text`, `--json`, `--jsonl`
- Stamp the speaker, ts_sec, and source provider on every segment
- Respect per-source rate limits via `cliutil.AdaptiveLimiter` and emit `*cliutil.RateLimitError`
  on exhaustion (per `feedback_no_extra_setup_prescriptions` and the source-client-check gate)
- Charge nothing for cookie/free hits; preview cost before any paid hit

## Data Layer

Primary entities:
- `episodes` — id (SHA-256(url)), source, show, url, host, guests TEXT[], date, duration_sec,
  fetched_at, provider, cost_credits, transcript_md, transcript_jsonl
- `feeds` — id, rss_url, source, show_title, added_at, last_sync_at
- `cookies` — service (huberman|acquired|founders|peterattia), captured_at, cookie_count,
  expires_at (the cookie itself is in a `~/.config/podcast-goat/cookies-<service>.json` file mode 0600)
- `spend_log` — id, ts, provider, episode_url, cost_credits, cost_usd_estimate

Sync cursor: per-feed `last_sync_at`; magic uses cache only (no live calls).

FTS/search: SQLite FTS5 virtual table over `episodes(content_md, show, guests)` with porter
stemmer, mirroring the `instacart` pattern.

## Codebase Intelligence (prior art to mirror)

- `printing-press/library/contact-goat/` — closest prior art: combo CLI, cookie auth
  (Clerk session for happenstance), paid API (Deepline), credit-confirm UX. We mirror
  command-structure (verb-first), chromecookies helper, cost-preview UX.
- `printing-press/library/contact-goat/internal/chromecookies/` — Chrome cookie extraction
  (Chrome 127+ requires 32-byte-host-prefix strip per `reference_chrome_app_bound_encryption`
  memory). Reuse `chromecookies.go` + `store.go` shape.
- `last30days-skill/scripts/lib/youtube_yt.py` — production-tested yt-dlp invocation,
  subtitle parsing, zh-Hans + en auto-translate flow for Xiaojun episodes.
- `printing-press/library/instacart/internal/store/` — SQLite + FTS5 pattern.
- `printing-press/library/granola/` — cache list/export commands.

## User Vision

(Verbatim USER_BRIEFING_CONTEXT excerpt — full file at `user-briefing-context.md`.)

> A combo CLI that pulls full transcripts from long-form podcasts and prints them as
> speaker-labeled markdown ready for an agent to summarize. Investor (Lan Xuezhao @xuezhao)
> wants to replace her copy-paste workflow for Hermes agents. Reference output shape:
> https://chip-supply-chain.fly.dev (built from two Dwarkesh transcripts she pasted in full).
> Lan said she'll pay for legit subscriptions and wants the CLI to use her logged-in cookies —
> that's why cookie-first dispatch is non-negotiable.

User-named hard constraints:
- Binary `podcast-goat-pp-cli`, MCP `podcast-goat-pp-mcp`
- Config under `~/.config/podcast-goat/`
- Output format = `**Speaker** (MM:SS)` markdown (= spoken.md native shape = Dwarkesh inline shape)
- Cookie-first dispatch is the headline. Cookie sources before free sources before paid.
- Six target shows (Dwarkesh, Acquired, Huberman, Founders, Peter Attia, Zhang Xiaojun)
  all work end-to-end.
- No paywall bypass; no audio storage; no in-CLI LLM summarization.

## Source Priority (combo CLI, confirmed in source-priority.json)

| # | Source | Tier | Auth | Replayable surface |
|---|---|---|---|---|
| 1 | member.huberman | cookie | Supercast cookie via `auth login --chrome --service huberman` | Authenticated transcript HTML, gated |
| 2 | member.acquired | cookie | Memberstack cookie | Authenticated Webflow transcript HTML |
| 3 | member.founders | cookie | founderspodcast.com cookie | Authenticated member HTML |
| 4 | member.peterattia | cookie | peterattiamd.com cookie | Authenticated show-notes + transcript HTML |
| 5 | dwarkesh | free | none | Substack `/p/<slug>` HTML, `<strong>Speaker</strong>` inline labels |
| 6 | rss | free | none | Podcasting 2.0 `<podcast:transcript>` URLs (VTT/SRT/HTML/JSON/text) |
| 7 | youtube | free subprocess | none | yt-dlp `--write-auto-subs`, zh-Hans + en translate for Xiaojun |
| 8 | spoken | paid | `x-api-key` (demo `pt_demo`) | `GET /search`, `GET /transcripts/{id}`, returns text/markdown |
| 9 | taddy | paid bulk | `TADDY_API_KEY` + `TADDY_USER_ID` | GraphQL `getEpisodeTranscript(uuid)` |
| 10 | whisperapi | paid audio | provider-pluggable: OpenAI / Deepgram / ElevenLabs | yt-dlp audio extract -> upload -> transcribe |

**Economics:** Primary chain (#1-#4 cookie + #5-#7 free) needs no paid key. `spoken`, `taddy`,
and `whisperapi` are scoped to commands that explicitly opt into them (`episode get --paid`,
`episode get --provider <name>`, or a configured fallback chain). The default chain stops
before any paid hit unless `--paid` or `--auto-paid` is passed (mirrors `contact-goat`'s
Deepline credit-confirm UX).

**Inversion risk:** None expected. spoken.md is the most "spec-complete" source (real API
contract, demo key), but it is intentionally tier 8 because it costs money. The primary
shows (Dwarkesh free, Huberman/Acquired/Founders/Peter Attia cookie) are the headline.

## Product Thesis

- Name: `podcast-goat-pp-cli` (slug `podcast-goat`)
- Display name: `Podcast GOAT`
- Why it should exist: agentic users (Hermes, Claude Code, GPT) waste minutes per session
  copy-pasting transcripts from member sites. There is no CLI that walks a paid-publisher
  cookie chain first, falls through to free RSS + YouTube auto-subs, and then to a $0.08
  paid provider only when nothing free works — and emits the same speaker-labeled markdown
  shape regardless of source. spoken.md gets close at $0.08/episode but it isn't free for
  episodes that the user already subscribes to. podcast-goat closes the loop.

## Build Priorities

1. **Data layer + canonical markdown normalizer.** SQLite + FTS5; all adapters write through
   one `Normalize(rawTranscript, format) -> SpeakerSegment[]` helper that produces the canonical
   markdown shape. Mirror `instacart` store.
2. **Cookie-first dispatch chain.** `chromecookies` helper (with Chrome 127+ 32-byte-prefix
   strip), per-service `cookies-<service>.json` files, `auth login --chrome --service <name>`,
   and a dispatcher that walks `cookie -> free -> paid` per URL. Mirror `contact-goat`.
3. **Killer paid path: spoken.md.** Pure HTTP, demo key works, return is already canonical
   markdown. Smallest+highest-leverage paid adapter.
4. **yt-dlp shared adapter.** Subprocess invocation, `--write-auto-subs --sub-langs zh-Hans,en
   --skip-download`, VTT parser. Mirror `last30days-skill/scripts/lib/youtube_yt.py`.
5. **Per-source adapters for the 4 cookie publishers + Dwarkesh + RSS.** Each is a small
   `internal/source/<name>/` package with `Fetch(url) (Transcript, error)` and a per-source
   rate limiter.
6. **Magic bundle command.** `magic "<topic>"` -> FTS5 query -> assemble top-N matches into
   one markdown file with YAML frontmatter per episode. This IS the chip-supply-chain.fly.dev
   workflow.
7. **Cost preview UX + budget tracking.** Print "this will cost N credits ($X.XX)" before any
   paid hit; require confirmation; respect `--yes` and `PODCAST_GOAT_AUTO_PAID=1`. Persist to
   `spend.jsonl`. Mirror contact-goat Deepline confirm.
8. **MCP cobra-tree mirror.** Every user-facing command becomes an MCP tool automatically
   (annotate `mcp:read-only: true` on episode get/search, magic, cache list, feeds list,
   doctor, budget show). Mutation tools (auth login, feeds add/sync, cache clear) stay
   un-annotated.
9. **Taddy + whisperapi adapters.** Lower priority; they're tier 9/10 fallbacks. Wire them
   so the dispatcher can include them in `--auto-paid` mode but they aren't the headline.
10. **Doctor.** Per-source health: cookie expiry, RSS reachability, yt-dlp present, paid keys
    set (or demo key working).

## Killer Features (8 from user brief, all in shipping scope)

1. **Cookie-first dispatch chain** — try user's cookie file first for paid-publisher URLs.
2. **spoken.md universal paid path** — demo key `pt_demo` works without signup.
3. **yt-dlp shared adapter** — Huberman free, Acquired free, generic YouTube, Xiaojun Chinese+EN.
4. **`auth login --chrome --service <name>`** — generalized; 4 bundled publishers, user-recipes
   in config (v1.5 nice-to-have, brief calls v1 = 4 builtin).
5. **`magic "<topic>"`** — bundle N FTS5-matched cached episodes into one prompt-shaped MD.
6. **Cost preview before every paid call** — mirror contact-goat Deepline UX.
7. **MCP wrapper auto-generated** — one MCP tool per top-level command via cobratree.
8. **Local SQLite + FTS5** — offline search across pulled transcripts.

## Six Target Shows (Phase 1.7 coverage check)

Each row gets its own browser-sniff marker entry; coverage check must explicitly mark
covered/partial/unsupported. No silent skip.

1. Dwarkesh — `dwarkesh.com/p/<slug>` (free, source #5)
2. Acquired — `acquired.fm/episodes/<slug>` (cookie #2 -> YouTube #7 -> spoken #8)
3. Huberman Lab — `hubermanlab.com/episode/<slug>` (cookie #1 -> YouTube #7 -> spoken #8)
4. Founders (David Senra) — `founderspodcast.com/episodes/<slug>` (cookie #3 -> spoken #8 -> taddy #9)
5. Peter Attia The Drive — `peterattiamd.com/podcast/<slug>` (cookie #4 -> spoken #8)
6. Zhang Xiaojun — `youtube.com/@xiaojunpodcast` (source #7 with zh-Hans + en auto-translate)

## Validation Gates (PP must enforce)

- `browser-sniff-gate.json` has 10 entries (one per source).
- `discovery/coverage-check.md` is a 6-row table (one per target show) covered/partial/unsupported.
- Per-source dogfood rows in the Phase 5 matrix (one per source).
- README opens with a Hermes-targeted 5-line quickstart.
- Scorecard >= A-.

## Live Smoke Test Plan (Phase 5)

1. `episode get https://www.dwarkesh.com/p/<recent>` — free Substack path.
2. `auth login --chrome --service huberman` then `episode get https://www.hubermanlab.com/episode/<premium-slug>`
   — cookie path. Requires user to be logged in.
3. `episode get https://www.acquired.fm/episodes/<recent> --paid --provider spoken --key pt_demo`
   — spoken.md path with demo key.
4. `episode get https://www.youtube.com/watch?v=<xiaojun-ep>` — yt-dlp Chinese + EN translate.
5. `cache list`, then `magic "AI chip supply chain"` produces single bundle file.
6. MCP wrapper boots in Claude Code; `episode_get` tool call works.

## Non-Goals

- No paywall bypass.
- No audio storage (text transcripts only).
- No in-CLI LLM summarization.
- No re-distribution surface (local agent use only).
