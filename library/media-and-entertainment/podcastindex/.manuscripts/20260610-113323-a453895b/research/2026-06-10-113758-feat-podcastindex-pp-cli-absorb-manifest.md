# PodcastIndex CLI Absorb Manifest

## Landscape
No feature-rich PodcastIndex CLI exists. Every tool is a thin language SDK mirroring the endpoints and returning raw JSON in-process: `comster/podcast-index-api` (Node), `podcastindex` (TS), `SarvagyaVaish/python-podcastindex` (Py), `SparrowTek/PodcastIndexKit` (Swift), `mr3y/PodcastIndex-SDK` (Kotlin), `jasonyork/podcast-index` (Ruby), `brb3/PodcastIndexSharp` (C#), `kilobit/podcast-index-client` (Go). "Absorb" = match the full endpoint surface; "beat" = offline store + FTS + agent-native output + compound commands, none of which any wrapper has.

## Architecture decision (load-bearing — confirm at gate)
PodcastIndex auth = 4 per-request computed headers (`X-Auth-Key`, `X-Auth-Date`=unix-now, `Authorization`=sha1hex(key+secret+now), `User-Agent`). The generator's `composed` auth emits a *static* header value and cannot recompute the SHA1 per call. So: **a hand-authored signer (`internal/podcastindex/signer.go`) + signed sibling client (`internal/podcastindex/client.go`) back every user-facing command as a novel command.** Generated raw endpoint-mirror commands cannot sign and are not the shipping surface. The generator is still used for the framework: store, sync, search/FTS, sql, doctor, MCP cobratree, output helpers, README/SKILL scaffold.

## Absorbed (match or beat everything that exists)
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|------------|--------------------|-------------|
| 1 | Search podcasts by term | all SDK wrappers `search.byterm` | `podcastindex-pp-cli find term <q>` | offline-cacheable, `--json/--select/--csv`, `--max/--clean/--fulltext/--similar` |
| 2 | Search episodes by person | wrappers `search.byperson` | `podcastindex-pp-cli find person <q>` | agent-native, FTS-storable |
| 3 | Search by title | wrappers `search.bytitle` | `podcastindex-pp-cli find title <q>` | composable output |
| 4 | Music search by term | wrappers `search.music.byterm` | `podcastindex-pp-cli find music <q>` | same |
| 5 | Resolve show → episodes | wrappers `episodes.byfeedid` | `podcastindex-pp-cli episodes by-feed <id>` | `--since/--newest/--max`, store-backed |
| 6 | Episode by id | wrappers `episodes.byid` | `podcastindex-pp-cli episodes get <id>` | agent-native |
| 7 | Episodes by guid / itunesid / podcastguid | wrappers `episodes.by*` | `podcastindex-pp-cli episodes by-guid/by-itunes/by-podcastguid` | uniform flags |
| 8 | Random episodes | wrappers `episodes.random` | `podcastindex-pp-cli episodes random` | `--cat/--lang/--max` |
| 9 | Live episodes | wrappers `episodes.live` | `podcastindex-pp-cli episodes live` | agent-native |
| 10 | Podcast by feedid / guid / feedurl / itunesid | wrappers `podcasts.by*` | `podcastindex-pp-cli podcasts by-feed/by-guid/by-url/by-itunes` | store-backed lookups |
| 11 | Trending podcasts | wrappers `podcasts.trending` | `podcastindex-pp-cli podcasts trending` | `--cat/--notcat/--lang/--since` |
| 12 | Podcasts by tag / medium | wrappers `podcasts.bytag/bymedium` | `podcastindex-pp-cli podcasts by-tag/by-medium` | uniform flags |
| 13 | Dead podcasts | wrappers `podcasts.dead` | `podcastindex-pp-cli podcasts dead` | feeds state-diff input |
| 14 | Recent feeds / episodes / newfeeds / soundbites | wrappers `recent.*` | `podcastindex-pp-cli recent feeds/episodes/newfeeds/soundbites` | `--before/--since` windows |
| 15 | Categories list | wrappers `categories.list` | `podcastindex-pp-cli categories` | cached locally |
| 16 | Stats current | wrappers `stats.current` | `podcastindex-pp-cli stats` | agent-native |
| 17 | Value4Value by feed / episode / guid | wrappers `value.*` | `podcastindex-pp-cli value by-feed/by-episode/by-guid` | feeds v4v leaderboard input |
| 18 | iTunes lookup / search passthrough | wrappers `itunes.*` | `(generated endpoint) itunes lookup/search` | parity |

## Transcendence (approved scope: tgrep only)
| # | Feature | Command | Buildability | Why Only We Can Do This | Long Description | Score |
|---|---------|---------|--------------|-------------------------|-----------------|-------|
| 1 | transcript-grep | `tgrep "<rx>" --cat=X` | hand-code | downloads `transcripts[]` URLs + FTS-indexes; API indexes only titles/descriptions | Search what was actually said inside episodes, not just metadata. | 9 |

Deferred (product-weak per gate decision 2026-06-10): dead-watch, drift, resurrect, cadence, guest-graph, value-rank, dedup. Snapshot-history trio dropped — temporal podcast diffing serves no real user job. cadence/guest-graph deferred as speculative nice-to-haves.

Stubs: none.