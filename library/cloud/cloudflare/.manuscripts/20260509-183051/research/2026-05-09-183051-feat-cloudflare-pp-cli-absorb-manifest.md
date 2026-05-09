# Cloudflare CLI — Absorb Manifest

The Cloudflare API surface is enormous (~3000 OpenAPI endpoints). Endpoint-mirror commands cover the absorbed feature universe automatically; this manifest groups them by product area for review and adds the transcendence layer.

## Absorbed (match or beat everything that exists)

| # | Area | Feature Set | Best Source(s) | Our Implementation |
|---|------|-------------|----------------|--------------------|
| 1 | Zones | list / create / delete / get / activation-check / hold | flarectl, terraform-provider, OpenAPI | endpoint-mirror + offline cache |
| 2 | Zone Settings | SSL, security level, cache TTL, dev mode, http2/3, IPv6, rocket loader, etc. (~50 settings) | OpenAPI | endpoint-mirror, all settings reachable |
| 3 | DNS Records | A/AAAA/CNAME/MX/TXT/SRV/NS/CAA/PTR + custom; CRUD + bulk batch endpoint | flarectl, cloudflare-cli, terraform-provider, OpenAPI | endpoint-mirror + bulk + offline FTS |
| 4 | DNSSEC | enable/disable, get keys, transfer | OpenAPI | endpoint-mirror |
| 5 | Cache | purge by URL/tag/hostname/all, cache-rules, browser cache TTL | cloudflare-cli, flarectl, OpenAPI | endpoint-mirror |
| 6 | Workers Scripts | list / get / put (deploy) / delete, content + metadata, multi-part bundles | wrangler, OpenAPI | endpoint-mirror |
| 7 | Workers Routes | route CRUD, custom domains, dispatch namespaces | wrangler, OpenAPI | endpoint-mirror |
| 8 | Workers Secrets / Env | secret put/list/delete, env vars, plain-text bindings | wrangler | endpoint-mirror |
| 9 | Workers KV | namespace CRUD, key list, get/put/delete, bulk read+write+delete, expiration, metadata | wrangler partial, OpenAPI | endpoint-mirror + bulk |
| 10 | Workers Durable Objects | namespace + DO list/get | OpenAPI | endpoint-mirror |
| 11 | Workers Queues | queue CRUD, consumer config | wrangler, OpenAPI | endpoint-mirror |
| 12 | Workers Cron Triggers | list / set / clear | wrangler, OpenAPI | endpoint-mirror |
| 13 | R2 | bucket CRUD, settings, lifecycle, CORS, notifications; objects via S3-compat | wrangler R2, OpenAPI | endpoint-mirror (REST surface) |
| 14 | D1 | database CRUD, raw query, exec, dumps, time-travel | wrangler D1, OpenAPI | endpoint-mirror |
| 15 | Pages | project CRUD, deployment list/get/retry/rollback, env vars, custom domains, deploy-hook trigger | wrangler Pages, OpenAPI | endpoint-mirror |
| 16 | Page Rules | CRUD, priority reorder, enable/disable | flarectl, terraform-provider | endpoint-mirror |
| 17 | Rulesets / WAF | rulesets CRUD, rules CRUD, managed rulesets, rate limit, custom rules, transform rules | terraform-provider, OpenAPI | endpoint-mirror |
| 18 | Access (Zero Trust) | apps + policies + groups + IDPs + service tokens + certificates + audit | terraform-provider, OpenAPI | endpoint-mirror |
| 19 | Access Tunnels (cloudflared) | tunnel CRUD, configurations, routes, credentials, virtual networks | wrangler partial, OpenAPI | endpoint-mirror |
| 20 | Access Gateway (Zero Trust DNS) | locations, lists, rules, settings | OpenAPI | endpoint-mirror |
| 21 | Custom Hostnames | hostname CRUD, certificates, settings | terraform-provider, OpenAPI | endpoint-mirror |
| 22 | SSL/TLS Certificates | edge SSL, origin CA, mTLS, client cert, universal SSL | flarectl Origin CA, OpenAPI | endpoint-mirror |
| 23 | Load Balancing | LBs, monitors, pools, origins, health checks, regions | terraform-provider, OpenAPI | endpoint-mirror |
| 24 | Notifications | policies CRUD, destinations (email/webhook/pagerduty), alert types | OpenAPI | endpoint-mirror |
| 25 | Logpush | jobs CRUD, datasets, ownership tokens, fields | OpenAPI | endpoint-mirror |
| 26 | Audit Logs | account audit, zone audit, query | OpenAPI | endpoint-mirror |
| 27 | Account | members, roles, invites, subscriptions, billing profile | OpenAPI | endpoint-mirror |
| 28 | Stream | video CRUD, signing keys, embedding, watermarks, thumbnails, captions | OpenAPI | endpoint-mirror |
| 29 | Images | image CRUD, variants, signing keys, stats | OpenAPI | endpoint-mirror |
| 30 | Spectrum | apps CRUD, policies | OpenAPI | endpoint-mirror |
| 31 | Magic Transit / Magic WAN | sites, connectors, routes, IPSec/GRE | OpenAPI | endpoint-mirror |
| 32 | Healthchecks | checks CRUD, statuses | OpenAPI | endpoint-mirror |
| 33 | Vectorize | index CRUD, query | OpenAPI | endpoint-mirror |
| 34 | AI / Workers AI | models, runs | OpenAPI | endpoint-mirror |
| 35 | DEX (Digital Experience) | tests, fleet status | OpenAPI | endpoint-mirror |
| 36 | API Tokens | list / verify / create / delete; permission groups | OpenAPI | endpoint-mirror |

**Coverage note:** 36 product areas, ~3000 OpenAPI endpoints. Every area maps to typed Cobra commands at generation time. We match every published incumbent CLI's feature set (wrangler / flarectl / cloudflare-cli / terraform-provider) plus all areas none of them cover.

**Stubs:** none. Every area lands as a working endpoint-mirror command via the printing-press generator.

## Transcendence (only possible with our approach)

| # | Feature | Command | Score | How It Works | Evidence |
|---|---------|---------|-------|-------------|----------|
| 1 | Idempotent DNS apply | `cloudflare dns apply --zone <z> --type <t> --name <n> --content <c> [--proxied]` | 10/10 | `GET /zones/{id}/dns_records?name=&type=` then PATCH-or-POST based on content equality; no-op if identical | Brief Top Workflow #1; flarectl errors on duplicate; user mid-sprint on klinikalarm.dk |
| 2 | Semantic redirect shortcut | `cloudflare redirect set "<from-pattern>" "<to-template>" --status 301` | 10/10 | Composes a `/zones/{id}/pagerules` POST with forwarding_url action + correct $1 templating, places at priority 1 | Brief User Vision; Page Rules JSON shape is friction nobody wraps |
| 3 | DNS propagation verifier | `cloudflare propagate watch <name> <type> --expect <value>` | 9/10 | Fans out DoH queries to 1.1.1.1, 8.8.8.8, 9.9.9.9 (free public no-auth); exits 0 when all match | Brief lists verification as part of sprint; no incumbent does multi-resolver verification |
| 4 | Cross-product domain locator | `cloudflare where-is <hostname>` | 10/10 | Local SQLite join across `zones`, `dns_records`, `worker_routes`, `page_rules`, `access_apps`, `custom_hostnames` | Brief Build Priorities → Transcend; no incumbent joins silos |
| 5 | Zone drift diff | `cloudflare zones diff <zone-a> <zone-b>` | 10/10 | Local diff over zone settings + page rules + DNS records (semantic name+type match) | Brief Top Workflow #5 explicit; Persona 2 weekly ritual |
| 6 | Worker single-pane bindings | `cloudflare worker bindings show <script>` | 8/10 | One call to `/accounts/{id}/workers/scripts/{name}` + local resolution of binding names → namespace IDs / bucket names / DB names + routes + secrets + queues + crons | Wrangler exposes pieces but never unifies; Persona 3 explicit |
| 7 | Cache purge with verify | `cloudflare cache purge release --zone <z> [--tags\|--hosts\|--urls] --probe <url>` | 9/10 | POST to purge endpoint, then HEAD `--probe` URL and assert `cf-cache-status: MISS` then `HIT` | Brief names `cache_purge_release` as headline MCP intent |
| 8 | Setup-zone named intent | `cloudflare setup_zone <zone> --origin <ip> [--redirect-from <pattern>]` | 10/10 | Composition over #1, #2, #3 plus zone-settings PATCH (SSL strict + Always-Use-HTTPS) | Brief User Vision is this workflow; named MCP intent |

All 8 survivors >= 8/10. Buildability proofs and persona attribution recorded in `2026-05-09-183051-novel-features-brainstorm.md`.

## MCP Surface Strategy — adopt the Cloudflare pattern

Surface count for this CLI:
- ~3000 endpoint-mirror tools (typed)
- + ~13 framework tools (sql, search, context, sync, stale, doctor, reconcile, etc.)
- + 8 transcendence commands

**Total ≈ 3021 tools.** This vastly exceeds the >50 threshold; printing-press recommends the Cloudflare pattern automatically. We adopt it explicitly:

```yaml
mcp:
  transport: [stdio, http]    # remote-capable, since this CLI runs as MCP server for cloud-hosted agents
  orchestration: code         # thin cloudflare_search + cloudflare_execute pair
  endpoint_tools: hidden      # suppress raw per-endpoint mirrors
  intents:
    - name: setup_zone
      description: "Set up a zone end-to-end: A record, optional redirect, SSL strict, Always-Use-HTTPS"
    - name: bulk_dns_apply
      description: "Idempotent batch DNS apply across one or more zones"
    - name: cache_purge_release
      description: "Purge by URL/tag/hostname and verify with cf-cache-status probes"
    - name: deploy_worker
      description: "Build and deploy a Worker, then tail logs"
    - name: audit_zone_drift
      description: "Diff settings + page rules + DNS between two zones"
```

This is symmetric with Cloudflare's own Code Mode MCP at `mcp.cloudflare.com`. The CLI doubles as the canonical example of the printing-press's "Cloudflare pattern" in the public library.
