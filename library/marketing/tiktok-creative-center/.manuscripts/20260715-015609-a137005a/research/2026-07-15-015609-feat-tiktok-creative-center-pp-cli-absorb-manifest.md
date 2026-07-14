# TikTok Creative Center CLI — Absorb Manifest

## Absorbed (match / beat everything that exists)
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-----------|-------------------|-------------|
| 1 | Trending hashtags list | Creative Center web + Apify hashtag actor | tiktok-creative-center-pp-cli hashtags list | Offline, --json, --select, SQLite-backed, any region/days |
| 2 | Hashtag popularity curve | Creative Center hashtag detail | tiktok-creative-center-pp-cli hashtags detail | + audience age & geo profile in one call, machine-readable |
| 3 | Top Ads / top content library | Creative Center Top Ads + PiPiADS/BigSpy | tiktok-creative-center-pp-cli top-ads list | region/sort/period filters, --json, syncable, free vs paid spy tools |
| 4 | Top Ads filter metadata | Creative Center overview | tiktok-creative-center-pp-cli top-ads overview | offline reference for industry/objective IDs |
| 5 | Hashtag publish count + rank | Creative Center list | (behavior in tiktok-creative-center-pp-cli hashtags list) publishCnt, rankIndex fields | sortable offline |
| 6 | Top creators per hashtag | Creative Center hashtag list | (behavior in tiktok-creative-center-pp-cli hashtags list) topCreators[] | extracted, joinable in SQLite |

## Transcendence (only possible with a local store)
| # | Feature | Command | Buildability | Why Only We Can Do This |
|---|---------|---------|--------------|------------------------|
| 1 | Niche report | niche | hand-code | Cross-entity join: given a niche keyword/industry, pull trending hashtags + their top creators + representative videos into one ranked brief — no UI does this |
| 2 | Trend velocity | velocity | hand-code | Computes growth % from popularityCurve snapshots stored across syncs; the web UI only shows one window at a time |
| 3 | Competitor sweep | competitor | hand-code | Given a brand handle/keyword, search Top Ads + match against trending hashtags to summarize a competitor's creative + positioning |
| 4 | New since last sync | since | hand-code | Snapshot diff over synced hashtags/top-ads — "what's newly trending since I last looked" requires historical rows in SQLite |
| 5 | Watch / alerts | watch | hand-code | Track a set of hashtags; report which crossed a popularity threshold since the last snapshot |
| 6 | Related hashtags | similar | hand-code | From stored hashtag industries + co-occurring top creators, surface similar/rising tags to a given one |
| 7 | Viral / underserved score | viral | hand-code | Ranks hashtags & content by an "opportunity score" = high popularity + low publish count (underserved but rising) — the exact signal for "what to create before it saturates." Computed from stored publishCnt + popularityCurve, no API gives it |
| 8 | Trending/viral content discovery | content | hand-code | Pulls trending content from THREE sources into one ranked feed: Top Ads library + representative videos from hashtag detail + top creators' recent work. The web UI silos these; one ranked viral feed across all of them doesn't exist |
| 9 | Content/account decision brief | decide | hand-code | THE killer feature for the stated goal: given a niche, synthesizes trending tags, viral content, saturation gaps, and competitor positioning into a concrete recommendation — what content to make, which hashtags to ride, what account angle is open. Pure synthesis over the local store; nothing upstream produces this |

## Stubs
- None. All shipping-scope. (Creator trends tab is "coming soon" upstream — not modeled.)

## Notes
- Auth: cookie + X-CSRFToken composed; `auth login --chrome` captures from the persistent Chrome profile.
- Two hosts: ads.tiktok.com (hashtags, trends) and ads.us.tiktok.com (top-ads) — modeled with per-resource base_url override.
