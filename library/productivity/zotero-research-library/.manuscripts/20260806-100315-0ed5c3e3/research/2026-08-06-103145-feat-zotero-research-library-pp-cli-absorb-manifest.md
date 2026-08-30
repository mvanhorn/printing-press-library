# Zotero Research Library — Absorb Manifest

## Absorbed (match or beat everything that exists)
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-----------|-------------------|-------------|
| 1 | API key auth + userID discovery | pyzotero | zotero-research-library-pp-cli auth set-token | /keys/current doctor probe resolves userID; config-written library prefix |
| 2 | Items list/search (q, qmode=everything, tag/itemType, \|\| OR, - negation) | pyzotero | (generated endpoint) items list / items top / items trash | Offline cache, --json/--select |
| 3 | Item by key + children + citation includes | pyzotero | (generated endpoint) items get / items children | --select |
| 4 | Collections tree + items | pyzotero | (generated endpoint) collections list/top/get/subcollections/items | Offline |
| 5 | Tags | pyzotero | (generated endpoint) tags list | Offline FTS |
| 6 | Saved searches | pyzotero | (generated endpoint) searches list / searches get | Synced for tombstones |
| 7 | Fulltext content + changed index | pyzotero | (generated endpoint) items fulltext / fulltext changed | Feeds local FTS |
| 8 | Deletion tombstones | pyzotero | (generated endpoint) deleted list | Sync correctness |
| 9 | Version-based incremental sync into SQLite | prior CLI v0.1 | framework sync + version cursors | The unique substrate no other Zotero tool has |
| 10 | Export formats (bibtex, ris, csljson) | zotero-cli tools | (generated endpoint) items list --format bibtex | Explicit limit handling |

### Transcendence (only possible with our approach)
| # | Feature | Command | Score | Buildability | How It Works | Evidence | Long Description |
|---|---------|---------|-------|--------------|--------------|----------|------------------|
| 1 | Offline research-grounding search | ground | 10/10 | hand-code | SQLite FTS5 over title/abstract/creators/tags/fulltext with rank+snippet; --fulltext resolves attachment hits to parent items; --live passthrough to ?q=&qmode=everything | Prior SKILL told agents to curl; the v0.1 gap this reprint exists to close; no tool pairs offline FTS with version sync | Use this for "what does my library say about X" — answers offline from the synced cache. Do NOT use 'items list' or the framework 'search' for grounding queries unless you need unsynced server state; run 'sync' first instead. |
| 2 | Backoff-correct transport + mid-sync version guard | (behavior in zotero-research-library-pp-cli sync) | 9/10 | hand-code | HTTP layer honors Backoff header on 200s AND Retry-After on 429/503, <=4 concurrency; restarts fetch phase when Last-Modified-Version shifts mid-pagination | Official docs; pyzotero issue #98 (reference wrapper honored only 429) | none |
| 3 | Recently-added triage | items recent | 8/10 | hand-code | ORDER BY dateAdded DESC from local cache with --days cutoff; keys pipe into export | Brief workflow #5 (Friday triage + manuscript citations) | Use for time-based triage of new additions. For topic queries use 'ground'; for full listings use 'items list'. |
| 4 | Citekey bridge (BBT backfill + lookup) | cite | 7/10 | hand-code | sync backfills Better BibTeX citekeys via JSON-RPC 127.0.0.1:23119 (graceful skip, --no-bbt); cite resolves citekey->item locally, exports citekey-faithful bibtex | Prior v0.1 shipped backfill (user: keep); Web API export cannot know BBT citekeys | Use 'cite' to resolve a Better BibTeX citekey. For bulk export without citekeys use 'items list --format bibtex'. |
| 5 | Cached collection tree with counts | collections tree | 7/10 | hand-code | Recursive parentCollection assembly + item counts from join table — a shape the flat API cannot return in one call | Brief workflow #3; pyzotero needs N+1 calls | Use for the hierarchical overview. For raw membership or live state use 'collections list' / 'collections items'. |
| 6 | Cache health + offline reindex | cache status | 7/10 | hand-code | Reads sync_state cursors, row/FTS counts, last-sync age; 'cache reindex' rebuilds FTS5 from base tables, zero network | Brief marks FTS rebuildable; v0.1's only remedy was full network resync | Use 'cache status' for offline introspection; 'doctor' for the live probe. 'cache reindex' never touches the network. |
