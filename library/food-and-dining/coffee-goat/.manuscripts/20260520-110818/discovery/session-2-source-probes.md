# Session 2 source probes — integration-blocking findings

All probed during Phase 1 research; results encoded here for the spec / generator / Phase 3 builders.

## Confirmed working

### YouTube channel discovery via RSS (no-auth)

YouTube exposes a no-auth RSS feed per channel:

```
https://www.youtube.com/feeds/videos.xml?channel_id=<CHANNEL_ID>
```

Returns last ~15 uploads with `<title>`, `<yt:videoId>`, `<published>`. Tested with Hoffmann's channel: 200 OK, 42KB, valid XML.

**Critical integration note:** youtube-pp-cli's `channel-uploads` subcommand requires the YouTube Data API key (`YOUTUBE_API_KEY`). Since coffee-goat is no-auth, coffee-goat must NOT call `channel-uploads`. Instead, fetch the RSS feed directly and parse out video IDs, then hand each ID to `youtube-pp-cli videos-transcript` (which uses the unauthenticated timedtext endpoint).

**Channel IDs confirmed:**
- James Hoffmann: `UCMb0O2CdPBNi-QqPk5T3gsQ`
- Lance Hedrick: `UCvNpZQzurSNZQ8e2QNGNXsA` (resolved via `youtube.com/@LanceHedrick`)

### youtube-pp-cli videos-transcript

Verified end-to-end. Two test invocations:
- `youtube-pp-cli youtube videos-transcript mMwscUNKbPk --json` ("How To Avoid A Bad Pour Over Brew") → exit 0, returned `{videoId, language, kind, segments: [{start_ms, duration_ms, text}, ...]}` shape
- `youtube-pp-cli youtube videos-transcript LZnAQ-PWQdg --json` ("Why Coffee Makes You Poop") → exit 0, full transcript text

Cleartext transcripts. No quota errors. Uses the InnerTube/timedtext path under the hood (no auth).

### Overpass API (OSM)

```bash
curl -A "coffee-goat-pp-cli/0.1 (https://github.com/...)" \
  -G --data-urlencode 'data=[out:json][timeout:5];node[amenity=cafe](around:200,40.7138,-74.006);out 3;' \
  "https://overpass-api.de/api/interpreter"
```

Returns 200 OK with proper JSON (`version`, `generator`, `osm3s`, `elements: [{type, id, lat, lon, tags}, ...]`).

**Critical:** Overpass returns **406 Not Acceptable** with curl's default User-Agent. Setting any custom UA (e.g., `coffee-goat-pp-cli/0.1`) makes it work. The generated Go client MUST set a User-Agent header for the Overpass adapter.

### Wikipedia REST API

`https://en.wikipedia.org/api/rest_v1/page/summary/World_Barista_Championship` → 200 OK, 2.7KB JSON. Public, no auth. Good source for clean winners table extraction.

## Known limitations

### wcc.coffee (official WCC site) has no usable past-rankings page

- Site is hosted on Squarespace.
- `/wbc-past-rankings` returns **HTTP 404 with full homepage HTML** (Squarespace soft-404). The page does NOT exist on the canonical site.
- Sister pages `/world-barista-championship`, `/world-brewers-cup`, etc. exist but contain current-season content only, not historical results.
- No structured rankings data available at the official site.

**Implication:** the WBC adapter must source winners from Wikipedia (clean), and supplement bean/recipe metadata from Sprudge editorial coverage (Chrome-cookie path already in prior research).

### jameshoffmann.com (blog) unreachable

- ECONNREFUSED on plain HTTPS probe during Hoffmann research.
- Defer blog integration; YouTube alone covers Hoffmann's value as a source.

## Source surface summary (Session 2 additions)

| Source | Endpoint | Auth | UA required | Notes |
|---|---|---|---|---|
| Hoffmann YouTube RSS | `youtube.com/feeds/videos.xml?channel_id=UCMb0O2CdPBNi-QqPk5T3gsQ` | none | no | Returns last 15 video IDs |
| Hedrick YouTube RSS | `youtube.com/feeds/videos.xml?channel_id=UCvNpZQzurSNZQ8e2QNGNXsA` | none | no | Same shape |
| YouTube transcripts | `youtube-pp-cli youtube videos-transcript <id>` (subprocess) | none | n/a | youtube-pp-cli must be on PATH |
| Wikipedia REST | `en.wikipedia.org/api/rest_v1/page/summary/<title>` | none | no | Clean JSON |
| Wikipedia HTML | `en.wikipedia.org/wiki/World_Barista_Championship` | none | no | For table parsing |
| OSM Overpass | `overpass-api.de/api/interpreter` | none | **YES** | 406 without custom UA |
| Sprudge guides | `sprudge.com/guides` + per-city URLs | none | Chrome cookie (Cloudflare) | Prior Session 1 path |
| wcc.coffee | (no rankings page exists) | n/a | n/a | Skip — Wikipedia covers it |

## Runtime dependencies surfaced

The CLI will need to detect youtube-pp-cli on PATH in its `doctor` command and emit a clear install command if missing.
