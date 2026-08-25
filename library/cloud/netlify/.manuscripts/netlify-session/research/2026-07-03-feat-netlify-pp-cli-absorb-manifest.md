# Netlify CLI Absorb Manifest

## Absorbed (match or beat everything that exists)
Generator emits typed commands for all 179 operations from the official spec. Highlights:

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | Site CRUD | netlify-cli / API `site` | (generated endpoint) sites list/get/create/update/delete | offline mirror, --json, --select |
| 2 | Deploy list/get/restore/lock | API `deploy` | (generated endpoint) deploys ... | local snapshots enable diff/timeline |
| 3 | Env var get/set/update/delete | netlify-cli env | (generated endpoint) environmentVariables ... | cross-site drift detection |
| 4 | DNS zone + record CRUD | API `dnsZone` | (generated endpoint) dnsZones ... | account-wide audit |
| 5 | Forms + submissions | API `form`/`submission` | (generated endpoint) forms/submissions ... | offline FTS across all forms |
| 6 | Build hooks | API `buildHook` | (generated endpoint) buildHooks ... | scriptable CI triggers |
| 7 | Deploy keys | API `deployKey` | (generated endpoint) deployKeys ... | agent-native output |
| 8 | Split tests, snippets, functions, members, accounts | API | (generated endpoint) per-resource | full surface parity |

All absorbed rows ship with `--json`, `--select`, `--dry-run`, typed exit codes, and SQLite persistence.

## Transcendence (only possible with our approach)
| # | Feature | Command | Buildability | Why Only We Can Do This | Long Description |
|---|---------|---------|--------------|-------------------------|------------------|
| 1 | Cross-site health dashboard | overview | hand-code | Local join across sites+deploys+forms; API is per-site | none |
| 2 | Environment variable drift | env-drift | hand-code | Local join over every site's env vars/contexts | Use before a release to catch a secret set on one site but missing on another. |
| 3 | Offline form submission search | submissions search | hand-code | FTS over local submission mirror; no cross-form API search | none |
| 4 | DNS audit | dns-audit | hand-code | Join dns_zones+records+sites+certs locally | Use to find dangling DNS records after tearing down sites. |
| 5 | Deploy timeline | since | hand-code | Time-windowed aggregation over local deploys across all sites | none |
| 6 | Deploy diff | deploy-diff | hand-code | Two locally-stored deploy snapshots joined; API returns one per call | none |

No stubs. All 6 transcendence rows are shipping scope.
