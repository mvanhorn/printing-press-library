# Government Data absorb manifest

Read-only user-approved workflows; administrative upload/update/delete and generic CKAN admin methods are outside scope.

### Absorbed
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---|---|---|---|
| 1 | Dataset search | ckanapi + Data.gov MCP | government-data-pp-cli catalog find | Common source selector, explicit per-source counts/errors |
| 2 | Dataset metadata | Data.gov MCP source | government-data-pp-cli dataset show | Normalized view plus raw metadata/provenance |
| 3 | Resource listing | California docs + Data.gov MCP | government-data-pp-cli dataset resources | Download URLs and formats |
| 4 | Resource download | Data.gov MCP resource template | government-data-pp-cli resource download | Explicit output path and bounded download |
| 5 | Organizations/publishers | California docs + federal v4 | government-data-pp-cli catalog organizations | Peer discovery |
| 6 | Topics/keywords | Data.gov MCP group_list/tag_list | government-data-pp-cli catalog topics | Source-native categories clearly labeled |
| 7 | Pagination/export | ckanapi search/dump | government-data-pp-cli catalog export | Bounded pages with resume/cursor output |
| 8 | Local metadata sync | ckanapi metadata export | government-data-pp-cli sync | Persist query scope with source identity |
| 9 | Offline lookup | Printing Press framework | government-data-pp-cli search | Local FTS and stale/unsynced hints |
