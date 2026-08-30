# Absorb Manifest: rapidapi-pp-cli

**Run:** 20260828-090622-dd9c3930

## Shipping scope (approved features)

### Resource: marketplace (public)
| Command | Operation | Notes |
|---|---|---|
| `rapidapi search <term>` | searchApis | --category, --tags, --limit, --sort, --offset, --json, --csv |
| `rapidapi categories` | getCategoriesByCtx / GetTopCategories | --limit, --json |
| `rapidapi collections` | GetCollectionsCollapsed | --limit, --json |
| `rapidapi collection show <slug>` | getCollectionBySlug | --json, --limit |
| `rapidapi api show <owner>/<slug>` | getApiBySlugAndOwner | full detail + endpoints, --json |
| `rapidapi user show <username>` | getUserProfile | --json |
| `rapidapi metrics` | getHubMetrics | --from --to, --json |

### Resource: account (auth)
| Command | Operation | Notes |
|---|---|---|
| `rapidapi whoami` | activeUser | --json |
| `rapidapi saved list` | getUserSavedApis | --json |
| `rapidapi saved add <apiId>` | favoriteApi | auth |
| `rapidapi saved remove <apiId>` | favoriteApi status=0 | auth |
| `rapidapi subscriptions list` | getApiSubscriptions | --status --limit, --json |
| `rapidapi notifications` | getNotifications | --limit --offset, --json |
| `rapidapi workspace` | getWorkspaceData | --from --to, --json |
| `rapidapi auth login / logout / status` | /authentication/* + csrf | cookie session mgmt |

### Novel / offline
- Local SQLite cache + offline search of cached data
- `--json` / `--csv` / table output everywhere
- Rate-limit-aware client (respect hub pacing)
- Stable exit codes

## Absorbed from landscape (not shipping)
- Provider API-key calling (out of scope — this CLI wraps the hub, not provider APIs)
- Console/Studio analytics dashboards (documented in catalog for future)

## Deprioritized (documented, not built)
- 200+ remaining GraphQL ops from bundle catalog (transactions, billing, org admin, 2FA, tutorials, issues, NAC, certificates, workflows) — available as extension points in README.

## Runtime decisions
- Transport: standard HTTP (rapidapi.com reachable directly)
- Client pattern: graphql-bff (POST /gateway/graphql)
- Auth: csrf-token bootstrap + cookie session + rapid-client header
- Spec: hand-built OpenAPI (graphql-bff style: POST /gateway/graphql#Operation per op)
