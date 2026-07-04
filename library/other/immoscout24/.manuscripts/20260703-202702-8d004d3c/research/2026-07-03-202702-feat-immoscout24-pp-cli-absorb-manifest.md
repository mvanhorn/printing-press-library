# ImmoScout24 Absorb Manifest

## Absorbed

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | Count matching listings | Fredy mobile API notes | immoscout24-pp-cli immoscout24-mobile-search total | Direct count before paginating; works without browser |
| 2 | Retrieve listing cards | Fredy provider | immoscout24-pp-cli immoscout24-mobile-search list | Agent JSON, `--select`, local sync support |
| 3 | Retrieve map markers | Fredy translator examples | immoscout24-pp-cli immoscout24-mobile-search map | Broad geo overview without HTML scraping |
| 4 | Retrieve expose detail | Fredy provider and wiestju wrapper | immoscout24-pp-cli expose | Structured detail sections and contact metadata |
| 5 | Use mobile API User-Agent | Fredy provider and wiestju wrapper | (behavior in immoscout24-pp-cli immoscout24-mobile-search list) default mobile User-Agent and no-auth config | Keeps replayable mobile API behavior explicit |

## Transcendence

| # | Feature | Command | Buildability | Why Only We Can Do This | Long Description |
|---|---------|---------|--------------|-------------------------|------------------|
| 1 | Web search URL translator | translate-url | hand-code | Requires mapping web URL path/query shapes to mobile API parameters | Convert an ImmoScout24 web search URL into the equivalent mobile API endpoint. |
| 2 | New listing watcher | watch | hand-code | Requires local seen-ID state across repeated mobile searches | Track a saved query and return only listings not seen before. |
| 3 | Expose text digest | expose digest | hand-code | Requires parsing detail sections into a compact agent-safe summary | Summarize long expose detail JSON into title, address, price, rooms, features, and description sections. |
| 4 | Query explain | query explain | hand-code | Requires translating opaque mobile filters into human-readable constraints | Explain a mobile query string or web URL in plain German/English. |
| 5 | Saved search diff | saved-search diff | hand-code | Requires local snapshots and comparison over time | Compare two saved search snapshots and show new, removed, and changed listings. |

## Approved Initial Scope

Generate the endpoint surface first. Hand-code transcendence after the generated CLI builds cleanly.
