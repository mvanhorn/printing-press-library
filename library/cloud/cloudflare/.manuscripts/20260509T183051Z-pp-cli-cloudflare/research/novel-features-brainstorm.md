# Cloudflare CLI — Novel Features Brainstorm (Audit Trail)

This is the full output from the novel-features subagent (Phase 1.5 Step 1.5c.5). Survivors flow into the absorb manifest's transcendence table; Customer model and Killed candidates persist here for retro/dogfood debugging.

---

## Customer model

**Persona 1 — Multi-zone solo operator**
- **Today:** runs a small fleet of self-hosted apps, ships frequently, owns a handful of zones under one Cloudflare account. Switches between dashboard tabs to copy A records between zones, set up redirects, purge cache after deploys.
- **Weekly ritual:** every new deploy needs a Cloudflare A record, often a page rule, and a cache purge. Currently three dashboard tabs and a curl scratchpad — origin IP re-typed from memory.
- **Frustration:** "I just shipped a project and I need (a) A record, (b) page-rule redirect from a legacy hostname, (c) confirm propagation, (d) purge cache. That's four tabs and 15 minutes. Why isn't this `cloudflare dns add` + `cloudflare redirect set` + done?"

**Persona 2 — Platform engineer with N zones (the drift hunter)**
- **Today:** owns 5-50 zones across staging/prod/client tenants. Cloudflare dashboard has no diff view between zones. Settings, page rules, WAF rules drift silently — staging has dev mode on, prod has it off, nobody knows when that changed.
- **Weekly ritual:** pre-launch checks, tenant-onboarding, "is staging configured the same as prod?" questions in PR review. Today this is opening two dashboard tabs and eyeballing.
- **Frustration:** Terraform exists but nobody actually maintains drift detection between live state and code. Wrangler doesn't speak DNS or page rules. flarectl is dormant. "I just want `cloudflare zones diff staging.com prod.com` to spit out the deltas."

**Persona 3 — Worker dev shipping multi-binding apps (the deploy-and-trace operator)**
- **Today:** uses wrangler for Workers, but the moment a Worker is wired to KV + R2 + D1 + a custom domain + a page rule, the truth is scattered. Worker says it has a KV binding called `CACHE`; what KV namespace ID does that point to? What's currently in it? What custom hostname routes traffic to this script?
- **Weekly ritual:** deploy → tail logs → debug → check bindings → check routes. Half of it is in wrangler, half in the dashboard.
- **Frustration:** "I need to see, for ONE worker, every binding + every route + every secret name + every queue + every cron — in one place. Nothing gives me that."

**Persona 4 — Agent / LLM acting on infrastructure (the read-then-write caller)**
- **Today:** an agent (Claude Code, Cursor, an autonomous orchestrator) is told "set up DNS for example.com." It doesn't know the zone_id, doesn't know if the record exists, and doesn't want to burn 1k tokens loading per-endpoint MCP tools. Cloudflare's own Code Mode MCP solved this for itself but other CLIs haven't.
- **Weekly ritual:** every imperative-infra session involves resolving names → IDs, listing existing state, then deciding to create/update/delete.
- **Frustration:** wrangler is dev-machine-only, flarectl is unmaintained, terraform is declarative-only — none of them give an agent a single `search` + `execute` pair backed by a local cache that can answer "does record X already exist on zone Y?" without 5 round-trips.

---

## Candidates (pre-cut)

(See subagent output for the full Pass 2 list with rubric verdicts inline. 16 candidates generated; 8 survived the cut.)

---

## Survivors

| # | Feature | Command | Score | How It Works | Evidence |
|---|---------|---------|-------|-------------|----------|
| 1 | Idempotent DNS apply | `cloudflare dns apply --zone <z> --type <t> --name <n> --content <c> [--proxied]` | 10/10 | `GET /zones/{id}/dns_records?name=&type=` then PATCH-or-POST based on content equality; no-op if identical | Brief Top Workflow #1; flarectl errors on duplicate; user mid-sprint on example.com needs this exact command |
| 2 | Semantic redirect shortcut | `cloudflare redirect set "<from-pattern>" "<to-template>" --status 301` | 10/10 | Composes a `/zones/{id}/pagerules` POST with forwarding_url action + correct $1 templating, places at priority 1 | Brief User Vision section names legacy.example.com → example.com redirect; Page Rules JSON shape is the friction nobody wraps |
| 3 | DNS propagation verifier | `cloudflare propagate watch <name> <type> --expect <value>` | 9/10 | Fans out DoH queries to 1.1.1.1, 8.8.8.8, 9.9.9.9 (free public no-auth resolvers); exits 0 when all match | Brief lists verification as part of example.com sprint; no incumbent CLI does multi-resolver verification |
| 4 | Cross-product domain locator | `cloudflare where-is <hostname>` | 10/10 | Local SQLite join across `zones`, `dns_records`, `worker_routes`, `page_rules`, `access_apps`, `custom_hostnames` indexed by hostname | Brief lists "find a domain wherever it appears" under Build Priorities → Transcend; no incumbent CLI joins across silos |
| 5 | Zone drift diff | `cloudflare zones diff <zone-a> <zone-b>` | 10/10 | Local diff over zone settings + page rules + DNS records (semantic name+type match) from cached snapshots | Brief Top Workflow #5 explicit ("painful, dashboard-only today"); Persona 2 weekly ritual |
| 6 | Worker single-pane bindings | `cloudflare worker bindings show <script>` | 8/10 | Single call to `/accounts/{id}/workers/scripts/{name}` + resolves binding name→namespace_id/bucket_name/db_name + lists routes, secrets, queues, crons in one table | Wrangler exposes the pieces but never unifies them; Persona 3 frustration explicit in brief |
| 7 | Cache purge with verify | `cloudflare cache purge release --zone <z> [--tags\|--hosts\|--urls] --probe <url>` | 9/10 | POST to purge endpoint, then HEAD `--probe` URL twice and assert `cf-cache-status: MISS` then `HIT` | Brief names `cache_purge_release` as a headline MCP intent; current absorbed purge has no verification step |
| 8 | Setup-zone named intent | `cloudflare setup_zone <zone> --origin <ip> [--redirect-from <pattern>]` | 10/10 | Composition: zone create-or-attach → A record (via #1) → optional redirect (via #2) → SSL strict + Always-Use-HTTPS → returns watch command (via #3) | Brief User Vision section is literally this workflow for example.com; named in the strategic-choice MCP intents list |

## Killed candidates

| Feature | Kill reason | Closest survivor |
|---------|------------|------------------|
| `coolify-attach` recipe | Subsumed by `setup_zone` — generic recipe serves same persona without leaking a third-party tool name into the CLI surface | #8 setup_zone |
| `account inventory` dump | Weak weekly use; duplicates Cloudflare dashboard overview; no transcendence proof | #4 where-is |
| `dns export/import` BIND roundtrip | Migration-time, not weekly. Zone diff covers "what changed?" Defer to v2 | #5 zones diff |
| `audit access export` | Persona-narrow (Zero Trust admins only). Real but not v1 with focus on Personas 1-3 | none |
| `audit waf rules` flatten | Persona-2-only weekly cadence unclear; Terraform users have a workable answer; defer | none |
| `worker tail --json` | Reframe to absorbed — just a flag on wrangler-equivalent tail, not a transcend feature | absorbed (Workers) |
| `iac export` Terraform HCL emission | Brief lists as transcend goal but real scope is large; not weekly; defer to v2 once v1 stable | none |
