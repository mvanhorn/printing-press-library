# podcast-goat Absorb Manifest

Combo CLI across 10 sources. Cookie-first dispatch is the headline. Every absorbed
feature in any competitor (timf34/podscript, steipete/summarize, youwhisper-cli,
yt-transcript, faster-whisper, spoken.md, Taddy) is matched. Eight transcendence rows
on top.

## Absorb sources (research surface)

| Source | Type | Why we care |
|--------|------|-------------|
| timf34/podscript | CLI | Closest OSS competitor: YouTube + RSS + ElevenLabs + Whisper; speaker diarization; --search/--list/--latest |
| steipete/summarize | CLI | Broader (URL/file/YouTube/podcast/PDF) + LLM summary; "one input at a time" model |
| spoken.md | paid SaaS API | Returns canonical `**Speaker** (MM:SS)` markdown; `pt_demo` key; agent skill via npx |
| Taddy | paid GraphQL API | Bulk episode-transcripts query; few free podcast-provided transcripts |
| yt-dlp | foundation subprocess | Subtitle extraction, auto-subs, multi-lang, translation |
| Podcasting 2.0 | spec | `<podcast:transcript>` namespace tag with MIME-routed parsers |
| youwhisper-cli, yt-transcript, faster-whisper | CLI | yt-dlp + Whisper variants; provider plurality |
| (no equivalent) | — | Member-cookie dispatch — no OSS prior art for the four target publishers |

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|---------------------|--------------|
| 1 | Fetch one transcript from URL | spoken.md API | `episode get <url>` walks cookie -> free -> paid | Cookie-first, per-source fallback, canonical markdown |
| 2 | Search episodes by query | podscript --search | `episode search "<query>"` FTS5 over local cache | Offline, regex, JSON |
| 3 | Transcribe YouTube via Whisper | youwhisper-cli | `episode get <yt-url>` -> auto-subs OR provider | Avoids Whisper cost when auto-subs exist |
| 4 | Speaker diarization | podscript --hf-token / ElevenLabs Scribe | spoken.md or ElevenLabs via `--provider` | No HuggingFace token wrangling |
| 5 | List recent episodes from RSS | podscript --list/--latest | `feeds add/sync/list` then `episode latest --source <name>` | Persistent subscriptions, FTS5 hit |
| 6 | Speaker-labeled markdown w/ timestamps | spoken.md native | All adapters normalize to canonical shape | Same shape across 10 sources |
| 7 | Output to file | podscript --output | `--output <path>` + auto cache path | Idempotent path = SHA-256(url) |
| 8 | LLM-friendly token size | spoken.md | Full transcript + `magic` bundle (multi-ep) | Multi-episode bundling |
| 9 | Apple/Spotify URL lookup | summarize | `episode get <apple/spotify-url>` -> spoken.md search | Universal URL |
| 10 | Generic RSS Podcasting 2.0 transcript | summarize / podscript | Source #6 adapter, MIME-routed | VTT/SRT/HTML/JSON/text |
| 11 | Multi-provider audio transcription | yt-transcript / quickwhisper | `--provider openai|deepgram|elevenlabs` | Pluggable + cost preview |
| 12 | Cache transcripts locally | podscript .md output | SQLite+FTS5 + per-episode .md + .jsonl | Searchable, exportable |
| 13 | Export cache | summarize | `cache export --format jsonl|md|zip` | Multi-format |
| 14 | Cost-aware paid calls | manual user discipline | `--yes` + spend.jsonl + `budget show` | contact-goat Deepline mirror |
| 15 | MCP / agent integration | spoken.md npx skills | Native cobratree mirror + tools-manifest.json | Auto-exposed; mcp:read-only annotations |
| 16 | One-time browser auth | none in OSS | `auth login --chrome --service <name>` | Generalized across 4 publishers |
| 17 | Doctor / health check | none in OSS | `doctor` per-source + per-provider | Cookie freshness, RSS reachability, yt-dlp install, keys |
| 18 | Latest episode shortcut | podscript --latest | `episode latest --source <name>` | Per-source AND multi-source default |

## Transcendence (only possible with our approach)

Eight survivors from the novel-features subagent (full personas + killed candidates in
`-novel-features-brainstorm.md`).

| # | Feature | Command | Score | Persona | Why Only We Can Do This |
|---|---------|---------|-------|---------|------------------------|
| 1 | Topic bundle for one-shot agent runs | `magic "<topic>"` | 9/10 | Lan / Hermes | Requires a local FTS5 cross-source corpus + fixed-schema YAML frontmatter emit; no remote API can do this |
| 2 | Dispatcher decision trace | `episode get --explain <url>` | 8/10 | Hermes / all | Requires knowledge of the cookie -> free -> paid chain plus local cookie state and budget state; only our dispatcher knows |
| 3 | Local quote grep with speaker context | `episode quote "<phrase>"` | 8/10 | Trevin | Requires FTS5 over `content_jsonl` segments + segment-aware context windowing; no SaaS exposes phrase-level grep across user's corpus |
| 4 | Cross-source content diff | `source compare <url>` | 7/10 | All | Requires fanning out to 2+ adapters in the same dispatcher run and structurally comparing the normalized outputs |
| 5 | Bilingual zh-Hans + en aligned transcript | `episode get --bilingual zh-Hans,en` | 7/10 | Xiaojun listener | Requires yt-dlp dual-language VTT + greedy nearest-neighbor segment alignment; no CLI ships this |
| 6 | Per-service cookie freshness table | `auth status` | 6/10 | Lan | Requires local knowledge of stored cookies per publisher + remediation hints; no equivalent in OSS prior art |
| 7 | Distinct-speaker corpus index | `speakers list` | 6/10 | Lan / Trevin | Requires SQL aggregation over `content_jsonl` speaker fields across all cached episodes; impossible without local store |
| 8 | Spend pivot by show + provider | `budget show --by-show` | 6/10 | Lan | Requires joining `spend.jsonl` to `episodes` by URL + pivoting; reveals cookie wins vs paid hits |

## Status (no stubs at v1)

Every shipping-scope feature is implementable in-session. No `(stub)` rows. Two
features are gated on user runtime action and so are scoped accordingly:

- Sources #1-#4 (cookie-based) require the user to run `auth login --chrome` once
  per service. The dispatcher and HTML parsers ship; live capture happens at user
  invocation, not at generation time.
- Source #10 (whisperapi) is provider-pluggable; ships behind `--provider whisper`
  flag with `doctor` check on key presence. No keys are stored on this machine
  during build; Phase 5 will smoke spoken.md (demo key) and yt-dlp (free) only.

## Priority inversion check

`source-priority.json` confirms `member.huberman` is source #1 and the cookie chain
sources #1-#4 lead. The synthetic spec gives the cookie chain four command-paths
each (one resource per publisher, registered as adapters under the unified
`episode` command), free sources (#5-#7) get one each, paid (#8-#10) get one each.
**No inversion**: the cookie chain has the most adapter coverage and the headline
command paths (`auth login`, `episode get`, `magic`) operate primarily on the
cookie chain.

## Phase Gate 1.5 readiness

- 18 absorbed features (full prior-art coverage)
- 8 transcendence features scoring 6-9/10
- 0 stubs
- Priority confirmed; no inversion
- User briefing context captured under `## User Vision` of the brief
- novel-features subagent ran cleanly with concrete personas

Ready to present to user.
