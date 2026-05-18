# Printing Press Brief: podcast-goat

**Invoke as:** `/printing-press podcast-goat`
(Or paste the body of this file at the briefing prompt as USER_BRIEFING_CONTEXT.)

---

## What this is

A combo CLI that pulls full transcripts from long-form podcasts and prints them as speaker-labeled markdown ready for an agent to summarize. Investor (Lan Xuezhao @xuezhao) wants to replace her copy-paste workflow for Hermes agents. Reference output shape: https://chip-supply-chain.fly.dev (built from two Dwarkesh transcripts she pasted in full).

## Binary name + identity

- Binary: `podcast-goat-pp-cli`
- MCP: `podcast-goat-pp-mcp`
- Config: `~/.config/podcast-goat/config.toml`
- DB: `~/.config/podcast-goat/podcast-goat.db` (SQLite + FTS5)
- Cookies: `~/.config/podcast-goat/cookies-<service>.json` (mode 0600)
- Spend log: `~/.config/podcast-goat/spend.jsonl`

## Sources, priority-ordered (for the Multi-Source Priority Gate)

Cookie-first, free-next, paid-last. Each row gets its own browser-sniff marker entry.

| # | Source | Type | Auth | What PP must figure out |
|---|---|---|---|---|
| 1 | `member.huberman` | cookie | Supercast cookie via `auth login --chrome --service huberman` | Authenticated Supercast transcript page HTML shape |
| 2 | `member.acquired` | cookie | Memberstack cookie via `auth login --chrome --service acquired` | Authenticated Acquired transcript surface (Webflow + Memberstack) |
| 3 | `member.founders` | cookie | founderspodcast.com cookie | Authenticated member transcript HTML |
| 4 | `member.peterattia` | cookie | peterattiamd.com cookie | Authenticated show-notes + transcript HTML |
| 5 | `dwarkesh` | free scrape | none | Substack `/p/<slug>` HTML, `<strong>Speaker</strong>` inline labels, `<h2>` section timestamps |
| 6 | `rss` | free | none | Podcasting 2.0 `<podcast:transcript>` tag; parse VTT/SRT/HTML/JSON/text by MIME |
| 7 | `youtube` | free subprocess | none | yt-dlp `--write-auto-subs --sub-langs <lang> --skip-download` + zh-Hans + auto-translate-en for Xiaojun |
| 8 | `spoken` | paid | `x-api-key` (demo key `pt_demo`) | `GET https://spoken.md/search?q=...`, `GET https://spoken.md/transcripts/<id>`, response is `text/markdown` with `**Speaker** (timestamp)` shape, charge in `X-Credits-Charged` header |
| 9 | `taddy` | paid bulk | `TADDY_API_KEY` + `TADDY_USER_ID` | GraphQL: episode-transcripts endpoint per taddy.org/developers/podcast-api/episode-transcripts |
| 10 | `whisperapi` | paid audio | provider-pluggable: `OPENAI_API_KEY` / `DEEPGRAM_API_KEY` / `ELEVENLABS_API_KEY` | Per-minute audio transcription; ElevenLabs Scribe preferred (diarization included, $0.004/min) |

## Six target shows (must all work end-to-end)

Coverage check happens in browser-sniff (Phase 1.7), one row per show:

1. **Dwarkesh** — `dwarkesh.com/p/<slug>` (free, source #5)
2. **Acquired** — `acquired.fm/episodes/<slug>` (cookie #2 → YouTube #7 → spoken #8)
3. **Huberman Lab** — `hubermanlab.com/episode/<slug>` (cookie #1 → YouTube #7 → spoken #8)
4. **Founders (David Senra)** — `founderspodcast.com/episodes/<slug>` (cookie #3 → spoken #8 → taddy #9)
5. **Peter Attia — The Drive** — `peterattiamd.com/podcast/<slug>` (cookie #4 → spoken #8)
6. **Zhang Xiaojun** — `youtube.com/@xiaojunpodcast` (source #7 with zh-Hans + en auto-translate)

## Command tree

```
podcast-goat episode get <url>        # fetch one transcript, cheapest-first chain
podcast-goat episode latest --source <name>
podcast-goat episode search "<query>" # FTS5 over local cache
podcast-goat feeds add <rss-url>      # subscribe to a show
podcast-goat feeds list
podcast-goat feeds sync               # pull new episodes for subscribed feeds
podcast-goat cache list
podcast-goat cache export --format jsonl
podcast-goat cache clear [--source X]
podcast-goat magic "<topic>"          # bundle N relevant episodes into one prompt-shaped MD file
podcast-goat auth login --chrome --service <huberman|acquired|founders|peterattia>
podcast-goat doctor                   # per-source + per-provider health
podcast-goat budget show              # running paid-call spend from spend.jsonl
podcast-goat budget reset
```

Every command takes `--json`, `--text`, `--md`. Default for transcript output is `--md`.

## Killer feature manifest (Phase 1.5 must approve)

1. **Cookie-first dispatch chain.** For URLs matching one of the four paid publishers, try the user's cookie file first. Fall through to free/paid only on miss.
2. **spoken.md primary universal paid path.** Takes any URL, returns speaker-labeled markdown matching the canonical format. Demo key `pt_demo` works without signup.
3. **yt-dlp shared adapter.** Covers Huberman free path, Acquired free path, generic YouTube, Xiaojun Chinese (zh-Hans + en auto-translate).
4. **`auth login --chrome --service <name>`** generalized — works for the 4 bundled publishers and accepts user-added recipes via config (v1.5 nice-to-have; v1 ships the 4).
5. **`magic <topic>`** recipe — bundles cached episodes into one markdown file with per-episode YAML frontmatter (source, URL, host, guests, date, provider, cost) + speaker-labeled body. This is the chip-supply-chain.fly.dev workflow as one command.
6. **Cost preview before every paid call.** Mirror contact-goat's Deepline credit-confirm UX. `--yes` bypasses prompt. `PODCAST_GOAT_AUTO_PAID=1` env auto-confirms.
7. **MCP wrapper auto-generated.** One MCP tool per top-level command. Descriptions audited by `printing-press-output-review`.
8. **Local SQLite + FTS5.** Offline search across all pulled transcripts. No service dependency.

## Output format (CANONICAL — all adapters must normalize to this)

Speaker-labeled markdown, matching spoken.md's native shape and Dwarkesh's Substack shape:

```markdown
**Speaker Name** (12:34)

Their sentence or paragraph. Multiple paragraphs from one speaker stay together until the next speaker.

**Other Speaker** (13:02)

Their response.
```

Plus optional `<h2>` section timestamps if the source provides them (Dwarkesh does, spoken.md does).

For `--format jsonl`: one segment per line, `{ "ts_sec": 754, "speaker": "Speaker Name", "text": "..." }`.

## References (mirror these patterns; don't re-invent)

- **`printing-press/library/contact-goat/`** — combo CLI with cookie auth + paid API + credit-confirm UX. Closest existing prior art.
- **`printing-press/library/contact-goat/internal/chromecookies/`** — Chrome cookie extraction shape. Reuse the agentcookie/pp-cli-bridge-sidecar work too (per `~/docs/plans/2026-05-17-001` and `2026-05-17-003`); Chrome 127+ needs 32-byte-host-prefix strip (per `reference_chrome_app_bound_encryption` memory).
- **`last30days-skill/.../youtube_yt.py`** — yt-dlp invocation, subtitle parsing, zh-Hans + auto-translate. Production-tested.
- **`printing-press/library/instacart/internal/store/`** — SQLite + FTS5 pattern.
- **`printing-press/library/granola/internal/cli/`** — cache + export commands.

## Non-goals

- No paywall bypass. Member-cookie path is the user's own legit access; spoken.md is publisher-sanctioned third-party; no third path.
- No audio storage. Cache holds text transcripts only.
- No in-CLI LLM summarization. `magic` produces the bundle; the agent does the summary.
- No re-distribution surface. CLI is for the user's local agent use.

## Validation gates (PP must enforce)

- `browser-sniff-gate.json` has 10 entries (one per source), all `confirmed: true`.
- `spoken/coverage-check.md` is a 6-row table (one per target show) explicitly marking covered / partial / unsupported. No silent skip.
- Per-source rate-limit dogfood rows in the Phase 5 matrix (one per source).
- README opens with a Hermes-targeted 5-line quickstart: install, doctor, `episode get <url>`, `magic <topic>`, `cache export | hermes summarize`.
- Scorecard ≥ A-.

## Live smoke test (before declaring ready)

1. `episode get https://www.dwarkesh.com/p/<recent>` — free path.
2. `auth login --chrome --service huberman` then `episode get https://www.hubermanlab.com/episode/<premium-slug>` — cookie path.
3. `episode get https://www.acquired.fm/episodes/<recent> --paid --key pt_demo` — spoken.md path.
4. `episode get https://www.youtube.com/watch?v=<xiaojun-ep>` — yt-dlp Chinese + en translate.
5. `cache list`, then `magic "AI chip supply chain"` produces a single bundle file usable as one agent prompt.
6. MCP wrapper boots in Claude Code; `episode_get` tool call works.

## Notes on origin

Generated from X DM thread 2026-05-17 between @mvanhorn and @xuezhao. Reference user output: https://chip-supply-chain.fly.dev. Lan said she'll pay for legit subscriptions and wants the CLI to use her logged-in cookies — that's why cookie-first dispatch is non-negotiable.
