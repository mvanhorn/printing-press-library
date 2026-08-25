# authentik-pp-cli Absorb Manifest

## Ecosystem scan
- Official: authentik admin UI + REST API at /api/v3/ (524 paths, OpenAPI 3.0.3)
- Python: goauthentik/client-python (auto-generated from OpenAPI)
- Go: no known mature wrapper
- Terraform: goauthentik/terraform-provider-authentik (mature, declarative only)
- MCPs: none widely-published as of 2026-05; goauthentik has issue threads on community MCP attempts

## Absorbed (match or beat)
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | List users with status / last login | admin UI users page | (generated endpoint) core users_list | --json, --select, --csv, SQL composable |
| 2 | List groups with member counts | admin UI groups page | (generated endpoint) core groups_list | Offline join via SQLite |
| 3 | List applications + provider binding | admin UI applications | (generated endpoint) core applications_list | Pipe to jq |
| 4 | List providers (OAuth2 / SAML / LDAP / Proxy) | admin UI providers | (generated endpoint) providers_list | --json --select |
| 5 | List flows with stage bindings | admin UI flows | (generated endpoint) flows_instances_list | offline SQLite |
| 6 | List policies and bindings | admin UI policies | (generated endpoint) policies_all_list | offline SQLite |
| 7 | List tokens with expiry | admin UI tokens | (generated endpoint) core tokens_list | --json --select expiring |
| 8 | Create / revoke API token | admin UI tokens | (generated endpoint) core tokens_create / _destroy | dry-run, JSON in/out |
| 9 | List stages | admin UI stages | (generated endpoint) stages_all_list | offline SQLite |
| 10 | List events | admin UI events | (generated endpoint) events_events_list | --json --select |
| 11 | Server status / version | admin UI status | (generated endpoint) admin system_retrieve | health command |
| 12 | List sources (LDAP/OAuth/SAML import) | admin UI sources | (generated endpoint) sources_all_list | offline SQLite |
| 13 | List tenants / brands | admin UI tenants | (generated endpoint) core brands_list | --json |
| 14 | List property mappings | admin UI mappings | (generated endpoint) propertymappings_all_list | --json |
| 15 | RBAC role + permission inspection | admin UI rbac | (generated endpoint) rbac_roles_list | offline join |

### Excluded per HomeLab Phase 4 brief
- crypto / certificate-key internals (`crypto`)
- outpost low-level service config (`outposts`)
- blueprint YAML import/export (`managed`)
- raw event stream / debug / metrics endpoints (none surfaced as separate tag; admins handled via `events` only)

## Transcendence
| # | Feature | Command | Buildability | Why Only We Can Do This |
|---|---------|---------|--------------|------------------------|
| 1 | One-shot operator health snapshot | `authentik-pp-cli health` | hand-code | Joins admin/system + tasks + workers + version into a single agent-readable summary |
| 2 | Stale token audit | `authentik-pp-cli tokens stale --days 90` | hand-code | Local SQLite join: tokens table + last-used events |
| 3 | Unused application audit | `authentik-pp-cli apps unused --days 30` | hand-code | Local join: applications + events filtered to login events |
| 4 | User group flatten | `authentik-pp-cli users groups <user>` | hand-code | Recursive group expansion local to SQLite, no N+1 API calls |
| 5 | Flow stage map | `authentik-pp-cli flows map <flow-slug>` | hand-code | Local join: flow_stage_bindings + stages, render ordered tree |

## Status
Approved scope for HomeLab Phase 4. Internal CLI; not for public-library publish.
