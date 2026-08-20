## Absorb Manifest

### Absorbed (match or beat everything that exists)
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-----------|-------------------|-------------|
| 1 | Create collection | weaviate-cli | (generated endpoint) collections create | --dry-run, --json, agent-native |
| 2 | Get collection config | weaviate-cli, mcp-weaviate get_schema | (generated endpoint) collections get | offline cache via sync |
| 3 | List collections | weaviate-cli, mcp-weaviate list_collections | (generated endpoint) collections list | local SQLite + search |
| 4 | Update collection | weaviate-cli | (generated endpoint) collections update | --dry-run |
| 5 | Delete collection | weaviate-cli | (generated endpoint) collections delete | --dry-run guard |
| 6 | Add property | weaviate-cli | (generated endpoint) collections add-property | typed |
| 7 | List property indexes | REST API only | (generated endpoint) collections list-indexes | typed |
| 8 | Rebuild/cancel property index | REST API only | (generated endpoint) collections rebuild-index / cancel-index | typed |
| 9 | Set property tokenization | REST API only | (generated endpoint) collections set-tokenization | typed |
| 10 | Delete vector index | REST API only | (generated endpoint) collections delete-vector-index | typed |
| 11 | List/reassign shards | REST API only | (generated endpoint) collections shards / shards update | typed |
| 12 | Multi-tenancy CRUD (list/create/update/delete tenants) | weaviate-cli | (generated endpoint) tenants list/create/update/delete | typed |
| 13 | Tenant existence check | REST API only | (generated endpoint) tenants exists | typed |

### Transcendence (only possible with our approach)
| # | Feature | Command | Buildability | Why Only We Can Do This | Long Description |
|---|---------|---------|--------------|------------------------|------------------|
| 1 | Config snapshot history | schema snapshot / schema history | hand-code | No API to store point-in-time collection config; requires local SQLite | none |
| 2 | Schema drift diff | schema diff <collection> [--against <snapshot\|collection>] | hand-code | Requires two configs held locally to diff; Weaviate has no diff endpoint | none |
| 3 | Config lint / best-practice check | collections lint <collection> | hand-code | Requires local rule engine over fetched config; no such endpoint exists | none |
| 4 | Cross-collection tenant audit | tenants audit | hand-code | Requires fan-out across every collection's tenants in one view; no bulk endpoint | none |
| 5 | Schema export/import bundle | schema export / schema import | hand-code | Portable JSON bundle of all collections for environment promotion; no native backup-schema-only feature | none |

Buildability: all 5 transcendence rows are hand-code (Go logic layered on the generated typed client + local SQLite store).
