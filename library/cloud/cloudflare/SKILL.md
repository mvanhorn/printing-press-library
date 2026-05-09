---
name: pp-cloudflare
description: "The unified Cloudflare CLI agents and operators both want — every product covered, with offline cache,... Trigger phrases: `set up dns for`, `purge cloudflare cache`, `redirect to`, `propagation check`, `where is this domain`, `use cloudflare`, `run cloudflare-pp-cli`."
author: "Alex Osti"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - cloudflare-pp-cli
---

# Cloudflare — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `cloudflare-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install cloudflare --cli-only
   ```
2. Verify: `cloudflare-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails before this CLI has a public-library category, install Node or use the category-specific Go fallback after publish.

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

Wrangler is Workers-only, flarectl is dormant, Terraform is declarative-only. This CLI absorbs all three and adds cross-product transcendence: idempotent DNS apply, semantic redirects, multi-resolver propagation watch, where-is, zone drift diff. MCP exposes the entire ~3000-endpoint surface through a thin search+execute pair (the Cloudflare pattern) so agents don't burn tokens loading per-endpoint tools.

## When to Use This CLI

Use this CLI when you need to manage Cloudflare imperatively across multiple products in one place: DNS + Workers + Page Rules + Access + Cache + WAF. It also doubles as an MCP server with the Cloudflare-pattern search+execute pair, so agents acting on infrastructure can navigate the full ~3000-endpoint API surface without burning tokens on per-endpoint tool definitions.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Idempotent infra
- **`dns apply`** — Apply a single DNS record with no-op-if-identical semantics, so the same command is safe to run repeatedly.

  _Reach for this when you need to provision DNS from a script or agent — guaranteed safe to run again with the same args._

  ```bash
  cloudflare-pp-cli dns apply --zone klinikalarm.dk --type A --name @ --content 157.173.102.165 --json
  ```
- **`redirect set`** — Create a 301/302 redirect from a URL pattern to a URL template — the Page Rule under the hood is composed for you.

  _Reach for this when you want a domain-level 301 redirect without hand-rolling a Page Rule body._

  ```bash
  cloudflare-pp-cli redirect set "mitlæge.dk/*" "https://klinikalarm.dk/$1" --status 301 --json
  ```

### Verification
- **`propagate watch`** — Verify a DNS record has propagated by querying multiple public resolvers (Cloudflare 1.1.1.1, Google 8.8.8.8, Quad9 9.9.9.9).

  _Reach for this immediately after `dns apply` to confirm the record is visible globally before kicking off downstream steps._

  ```bash
  cloudflare-pp-cli propagate watch klinikalarm.dk A --expect 157.173.102.165 --watch
  ```
- **`cache purge release`** — Purge cache by URL/tag/hostname and verify with cf-cache-status header probes (MISS then HIT).

  _Reach for this in deploy/release scripts where downstream steps depend on cached content actually being purged._

  ```bash
  cloudflare-pp-cli cache purge release --zone klinikalarm.dk --tags release-v1 --probe https://klinikalarm.dk/
  ```

### Cross-product
- **`where-is`** — Find every place a hostname appears across DNS records, Worker routes, and Page Rules in one command.

  _Reach for this before deleting or changing a domain — check that nothing else depends on it._

  ```bash
  cloudflare-pp-cli where-is klinikalarm.dk --json
  ```
- **`zones diff`** — Diff two zones across settings, page rules, and DNS records (semantic name+type match) to find drift.

  _Reach for this before promoting staging to prod, during incident review, or when onboarding a tenant zone from a template._

  ```bash
  cloudflare-pp-cli zones diff staging.makertoo.win prod.makertoo.win --json
  ```
- **`worker bindings show`** — Show every binding (KV, R2, D1, queue, secret, cron, route, custom domain) for one Worker in a single table.

  _Reach for this when debugging a Worker in production — see everything wired to it without 10 dashboard tabs._

  ```bash
  cloudflare-pp-cli worker bindings show my-worker --account <account_id> --json
  ```

### Composition
- **`setup_zone`** — End-to-end zone setup: A record, optional redirect, SSL strict, Always-Use-HTTPS — one command instead of four dashboard tabs.

  _Reach for this when wiring a new domain end-to-end — the primary intent for a freshly-deployed app._

  ```bash
  cloudflare-pp-cli setup_zone klinikalarm.dk --origin 157.173.102.165 --redirect-from "mitlæge.dk/*" --json
  ```

## Command Reference

**accounts** — Manage accounts

- `cloudflare-pp-cli accounts batch-move` — Batch move a collection of accounts to a specific organization. ⚠️ Not implemented.
- `cloudflare-pp-cli accounts creation` — Create an account (only available for tenant admins at this time)
- `cloudflare-pp-cli accounts deletion` — Delete a specific account (only available for tenant admins at this time). This is a permanent operation that will...
- `cloudflare-pp-cli accounts details` — Get information about a specific account that you are a member of.
- `cloudflare-pp-cli accounts list` — List all accounts you have ownership or verified access to.
- `cloudflare-pp-cli accounts update` — Update an existing account.

**certificates** — Manage certificates

- `cloudflare-pp-cli certificates origin-ca-create` — Create an Origin CA certificate. You can use an Origin CA Key as your User Service Key or an API token when calling...
- `cloudflare-pp-cli certificates origin-ca-get` — Get an existing Origin CA certificate by its serial number. You can use an Origin CA Key as your User Service Key or...
- `cloudflare-pp-cli certificates origin-ca-list` — List all existing Origin CA certificates for a given zone. You can use an Origin CA Key as your User Service Key or...
- `cloudflare-pp-cli certificates origin-ca-revoke` — Revoke an existing Origin CA certificate by its serial number. You can use an Origin CA Key as your User Service Key...

**internal** — Manage internal

- `cloudflare-pp-cli internal` — Internal route for testing URL submissions

**ips** — Manage ips

- `cloudflare-pp-cli ips` — Get IPs used on the Cloudflare/JD Cloud network, see https://www.cloudflare.com/ips for Cloudflare IPs or...

**live** — Manage live

- `cloudflare-pp-cli live` — Return a success message after running liveness checks

**memberships** — Manage memberships

- `cloudflare-pp-cli memberships user-s-account-delete` — Remove the associated member from an account.
- `cloudflare-pp-cli memberships user-s-account-details` — Get a specific membership.
- `cloudflare-pp-cli memberships user-s-account-list` — List memberships of accounts the user can access.
- `cloudflare-pp-cli memberships user-s-account-update` — Accept or reject this account invitation.

**organizations** — Manage organizations

- `cloudflare-pp-cli organizations create-user` — Create a new organization for a user. (Currently in Closed Beta - see...
- `cloudflare-pp-cli organizations delete` — Delete an organization. The organization MUST be empty before deleting. It must not contain any sub-organizations,...
- `cloudflare-pp-cli organizations list` — Retrieve a list of organizations a particular user has access to. (Currently in Closed Beta - see...
- `cloudflare-pp-cli organizations modify` — Modify organization. (Currently in Closed Beta - see https://developers.cloudflare.com/fundamentals/organizations/)
- `cloudflare-pp-cli organizations retrieve` — Retrieve the details of a certain organization. (Currently in Closed Beta - see...

**radar** — Manage radar

- `cloudflare-pp-cli radar get-agent-readiness-summary` — Returns a summary of AI agent readiness scores across scanned domains, grouped by the specified dimension. Data is...
- `cloudflare-pp-cli radar get-ai-bots-summary` — Retrieves an aggregated summary of AI bots HTTP requests grouped by the specified dimension.
- `cloudflare-pp-cli radar get-ai-bots-summary-by-user-agent` — Retrieves the distribution of traffic by AI user agent.
- `cloudflare-pp-cli radar get-ai-bots-timeseries` — Retrieves AI bots HTTP request volume over time.
- `cloudflare-pp-cli radar get-ai-bots-timeseries-group` — Retrieves the distribution of HTTP requests from AI bots, grouped by the specified dimension over time.
- `cloudflare-pp-cli radar get-ai-bots-timeseries-group-by-user-agent` — Retrieves the distribution of traffic by AI user agent over time.
- `cloudflare-pp-cli radar get-ai-inference-summary` — Retrieves an aggregated summary of unique accounts using Workers AI inference grouped by the specified dimension.
- `cloudflare-pp-cli radar get-ai-inference-summary-by-model` — Retrieves the distribution of unique accounts by model.
- `cloudflare-pp-cli radar get-ai-inference-summary-by-task` — Retrieves the distribution of unique accounts by task.
- `cloudflare-pp-cli radar get-ai-inference-timeseries-group` — Retrieves the distribution of unique accounts using Workers AI inference, grouped by the specified dimension over time.
- `cloudflare-pp-cli radar get-ai-inference-timeseries-group-by-model` — Retrieves the distribution of unique accounts by model over time.
- `cloudflare-pp-cli radar get-ai-inference-timeseries-group-by-task` — Retrieves the distribution of unique accounts by task over time.
- `cloudflare-pp-cli radar get-ai-markdown-for-agents-summary` — Retrieves the overall median HTML-to-markdown reduction ratio for AI agent requests over the given date range.
- `cloudflare-pp-cli radar get-ai-markdown-for-agents-timeseries` — Retrieves the median HTML-to-markdown reduction ratio over time for AI agent requests.
- `cloudflare-pp-cli radar get-annotations` — Retrieves the latest annotations.
- `cloudflare-pp-cli radar get-annotations-outages` — Retrieves the latest Internet outages and anomalies.
- `cloudflare-pp-cli radar get-annotations-outages-top` — Retrieves the number of outages by location.
- `cloudflare-pp-cli radar get-as-botnet-threat-feed` — Retrieves a ranked list of Autonomous Systems based on their presence in the Cloudflare Botnet Threat Feed. Rankings...
- `cloudflare-pp-cli radar get-attacks-layer3-summary` — Retrieves the distribution of layer 3 attacks by the specified dimension.
- `cloudflare-pp-cli radar get-attacks-layer3-summary-by-bitrate` — Retrieves the distribution of layer 3 attacks by bitrate.
- `cloudflare-pp-cli radar get-attacks-layer3-summary-by-duration` — Retrieves the distribution of layer 3 attacks by duration.
- `cloudflare-pp-cli radar get-attacks-layer3-summary-by-industry` — Retrieves the distribution of layer 3 attacks by targeted industry.
- `cloudflare-pp-cli radar get-attacks-layer3-summary-by-ip-version` — Retrieves the distribution of layer 3 attacks by IP version.
- `cloudflare-pp-cli radar get-attacks-layer3-summary-by-protocol` — Retrieves the distribution of layer 3 attacks by protocol.
- `cloudflare-pp-cli radar get-attacks-layer3-summary-by-vector` — Retrieves the distribution of layer 3 attacks by vector.
- `cloudflare-pp-cli radar get-attacks-layer3-summary-by-vertical` — Retrieves the distribution of layer 3 attacks by targeted vertical.
- `cloudflare-pp-cli radar get-attacks-layer3-timeseries-by-bytes` — Get layer 3 attacks by bytes time series
- `cloudflare-pp-cli radar get-attacks-layer3-timeseries-group` — Retrieves the distribution of layer 3 attacks grouped by dimension over time.
- `cloudflare-pp-cli radar get-attacks-layer3-timeseries-group-by-bitrate` — Retrieves the distribution of layer 3 attacks by bitrate over time.
- `cloudflare-pp-cli radar get-attacks-layer3-timeseries-group-by-duration` — Retrieves the distribution of layer 3 attacks by duration over time.
- `cloudflare-pp-cli radar get-attacks-layer3-timeseries-group-by-industry` — Retrieves the distribution of layer 3 attacks by targeted industry over time.
- `cloudflare-pp-cli radar get-attacks-layer3-timeseries-group-by-ip-version` — Retrieves the distribution of layer 3 attacks by IP version over time.
- `cloudflare-pp-cli radar get-attacks-layer3-timeseries-group-by-protocol` — Retrieves the distribution of layer 3 attacks by protocol over time.
- `cloudflare-pp-cli radar get-attacks-layer3-timeseries-group-by-vector` — Retrieves the distribution of layer 3 attacks by vector over time.
- `cloudflare-pp-cli radar get-attacks-layer3-timeseries-group-by-vertical` — Retrieves the distribution of layer 3 attacks by targeted vertical over time.
- `cloudflare-pp-cli radar get-attacks-layer3-top-attacks` — Retrieves the top layer 3 attacks from origin to target location. Values are a percentage out of the total layer 3...
- `cloudflare-pp-cli radar get-attacks-layer3-top-industries` — This endpoint is deprecated. To continue getting this data, switch to the summary by industry endpoint.
- `cloudflare-pp-cli radar get-attacks-layer3-top-verticals` — This endpoint is deprecated. To continue getting this data, switch to the summary by vertical endpoint.
- `cloudflare-pp-cli radar get-attacks-layer7-summary` — Retrieves the distribution of layer 7 attacks by the specified dimension.
- `cloudflare-pp-cli radar get-attacks-layer7-summary-by-http-method` — Retrieves the distribution of layer 7 attacks by HTTP method.
- `cloudflare-pp-cli radar get-attacks-layer7-summary-by-http-version` — Retrieves the distribution of layer 7 attacks by HTTP version.
- `cloudflare-pp-cli radar get-attacks-layer7-summary-by-industry` — Retrieves the distribution of layer 7 attacks by targeted industry.
- `cloudflare-pp-cli radar get-attacks-layer7-summary-by-ip-version` — Retrieves the distribution of layer 7 attacks by IP version.
- `cloudflare-pp-cli radar get-attacks-layer7-summary-by-managed-rules` — Retrieves the distribution of layer 7 attacks by managed rules.
- `cloudflare-pp-cli radar get-attacks-layer7-summary-by-mitigation-product` — Retrieves the distribution of layer 7 attacks by mitigation product.
- `cloudflare-pp-cli radar get-attacks-layer7-summary-by-vertical` — Retrieves the distribution of layer 7 attacks by targeted vertical.
- `cloudflare-pp-cli radar get-attacks-layer7-timeseries` — Retrieves layer 7 attacks over time.
- `cloudflare-pp-cli radar get-attacks-layer7-timeseries-group` — Retrieves the distribution of layer 7 attacks grouped by dimension over time.
- `cloudflare-pp-cli radar get-attacks-layer7-timeseries-group-by-http-method` — Retrieves the distribution of layer 7 attacks by HTTP method over time.
- `cloudflare-pp-cli radar get-attacks-layer7-timeseries-group-by-http-version` — Retrieves the distribution of layer 7 attacks by HTTP version over time.
- `cloudflare-pp-cli radar get-attacks-layer7-timeseries-group-by-industry` — Retrieves the distribution of layer 7 attacks by targeted industry over time.
- `cloudflare-pp-cli radar get-attacks-layer7-timeseries-group-by-ip-version` — Retrieves the distribution of layer 7 attacks by IP version used over time.
- `cloudflare-pp-cli radar get-attacks-layer7-timeseries-group-by-managed-rules` — Retrieves the distribution of layer 7 attacks by managed rules over time.
- `cloudflare-pp-cli radar get-attacks-layer7-timeseries-group-by-mitigation-product` — Retrieves the distribution of layer 7 attacks by mitigation product over time.
- `cloudflare-pp-cli radar get-attacks-layer7-timeseries-group-by-vertical` — Retrieves the distribution of layer 7 attacks by targeted vertical over time.
- `cloudflare-pp-cli radar get-attacks-layer7-top-attacks` — Retrieves the top attacks from origin to target location. Values are percentages of the total layer 7 attacks (with...
- `cloudflare-pp-cli radar get-attacks-layer7-top-industries` — This endpoint is deprecated. To continue getting this data, switch to the summary by industry endpoint.
- `cloudflare-pp-cli radar get-attacks-layer7-top-verticals` — This endpoint is deprecated. To continue getting this data, switch to the summary by vertical endpoint.
- `cloudflare-pp-cli radar get-bgp-hijacks-events` — Retrieves the BGP hijack events.
- `cloudflare-pp-cli radar get-bgp-ips-timeseries` — Retrieves time series data for the announced IP space count, represented as the number of IPv4 /24s and IPv6 /48s,...
- `cloudflare-pp-cli radar get-bgp-ips-top-ases` — Returns the top-N autonomous systems by announced IP space at the nearest 8-hour RIB boundary at or before the...
- `cloudflare-pp-cli radar get-bgp-pfx2as` — Retrieves the prefix-to-ASN mapping from global routing tables.
- `cloudflare-pp-cli radar get-bgp-pfx2as-moas` — Retrieves all Multi-Origin AS (MOAS) prefixes in the global routing tables.
- `cloudflare-pp-cli radar get-bgp-route-leak-events` — Retrieves the BGP route leak events.
- `cloudflare-pp-cli radar get-bgp-routes-asns` — Retrieves all ASes in the current global routing tables with routing statistics.
- `cloudflare-pp-cli radar get-bgp-routes-realtime` — Retrieves real-time BGP routes for a prefix, using public real-time data collectors (RouteViews and RIPE RIS).
- `cloudflare-pp-cli radar get-bgp-routes-stats` — Retrieves the BGP routing table stats.
- `cloudflare-pp-cli radar get-bgp-rpki-aspa-changes` — Retrieves ASPA (Autonomous System Provider Authorization) changes over time. Returns daily aggregated changes...
- `cloudflare-pp-cli radar get-bgp-rpki-aspa-snapshot` — Retrieves current or historical ASPA (Autonomous System Provider Authorization) objects. ASPA objects define which...
- `cloudflare-pp-cli radar get-bgp-rpki-aspa-timeseries` — Retrieves ASPA (Autonomous System Provider Authorization) object count over time. Supports filtering by RIR or...
- `cloudflare-pp-cli radar get-bgp-timeseries` — Retrieves BGP updates over time. When requesting updates for an autonomous system, only BGP updates of type...
- `cloudflare-pp-cli radar get-bgp-top-ases` — Retrieves the top autonomous systems by BGP updates (announcements only).
- `cloudflare-pp-cli radar get-bgp-top-prefixes` — Retrieves the top network prefixes by BGP updates.
- `cloudflare-pp-cli radar get-bot-details` — Retrieves the requested bot information.
- `cloudflare-pp-cli radar get-bots` — Retrieves a list of bots.
- `cloudflare-pp-cli radar get-bots-summary` — Retrieves an aggregated summary of bots HTTP requests grouped by the specified dimension.
- `cloudflare-pp-cli radar get-bots-timeseries` — Retrieves bots HTTP request volume over time.
- `cloudflare-pp-cli radar get-bots-timeseries-group` — Retrieves the distribution of HTTP requests from bots, grouped by the specified dimension over time.
- `cloudflare-pp-cli radar get-certificate-authorities` — Retrieves a list of certificate authorities.
- `cloudflare-pp-cli radar get-certificate-authority-details` — Retrieves the requested CA information.
- `cloudflare-pp-cli radar get-certificate-log-details` — Retrieves the requested certificate log information.
- `cloudflare-pp-cli radar get-certificate-logs` — Retrieves a list of certificate logs.
- `cloudflare-pp-cli radar get-ct-summary` — Retrieves an aggregated summary of certificates grouped by the specified dimension.
- `cloudflare-pp-cli radar get-ct-timeseries` — Retrieves certificate volume over time.
- `cloudflare-pp-cli radar get-ct-timeseries-group` — Retrieves the distribution of certificates grouped by the specified dimension over time.
- `cloudflare-pp-cli radar get-dns-as112-summary` — Retrieves the distribution of AS112 queries by the specified dimension.
- `cloudflare-pp-cli radar get-dns-as112-timeseries` — Retrieves the AS112 DNS queries over time.
- `cloudflare-pp-cli radar get-dns-as112-timeseries-by-dnssec` — Retrieves the distribution of DNS queries to AS112 by DNSSEC (DNS Security Extensions) support.
- `cloudflare-pp-cli radar get-dns-as112-timeseries-by-edns` — Retrieves the distribution of DNS queries to AS112 by EDNS (Extension Mechanisms for DNS) support.
- `cloudflare-pp-cli radar get-dns-as112-timeseries-by-ip-version` — Retrieves the distribution of DNS queries to AS112 by IP version.
- `cloudflare-pp-cli radar get-dns-as112-timeseries-by-protocol` — Retrieves the distribution of DNS queries to AS112 by protocol.
- `cloudflare-pp-cli radar get-dns-as112-timeseries-by-query-type` — Retrieves the distribution of DNS queries to AS112 by type.
- `cloudflare-pp-cli radar get-dns-as112-timeseries-by-response-codes` — Retrieves the distribution of AS112 DNS requests classified by response code.
- `cloudflare-pp-cli radar get-dns-as112-timeseries-group` — Retrieves the distribution of AS112 queries grouped by dimension over time.
- `cloudflare-pp-cli radar get-dns-as112-timeseries-group-by-dnssec` — Retrieves the distribution of AS112 DNS queries by DNSSEC (DNS Security Extensions) support over time.
- `cloudflare-pp-cli radar get-dns-as112-timeseries-group-by-edns` — Retrieves the distribution of AS112 DNS queries by EDNS (Extension Mechanisms for DNS) support over time.
- `cloudflare-pp-cli radar get-dns-as112-timeseries-group-by-ip-version` — Retrieves the distribution of AS112 DNS queries by IP version over time.
- `cloudflare-pp-cli radar get-dns-as112-timeseries-group-by-protocol` — Retrieves the distribution of AS112 DNS requests classified by protocol over time.
- `cloudflare-pp-cli radar get-dns-as112-timeseries-group-by-query-type` — Retrieves the distribution of AS112 DNS queries by type over time.
- `cloudflare-pp-cli radar get-dns-as112-timeseries-group-by-response-codes` — Retrieves the distribution of AS112 DNS requests classified by response code over time.
- `cloudflare-pp-cli radar get-dns-as112-top-locations` — Retrieves the top locations by AS112 DNS queries.
- `cloudflare-pp-cli radar get-dns-summary` — Retrieves the distribution of DNS queries by the specified dimension.
- `cloudflare-pp-cli radar get-dns-summary-by-cache-hit-status` — Retrieves the distribution of DNS queries by cache status.
- `cloudflare-pp-cli radar get-dns-summary-by-dnssec` — Retrieves the distribution of DNS responses by DNSSEC (DNS Security Extensions) support.
- `cloudflare-pp-cli radar get-dns-summary-by-dnssec-awareness` — Retrieves the distribution of DNS queries by DNSSEC (DNS Security Extensions) client awareness.
- `cloudflare-pp-cli radar get-dns-summary-by-dnssec-e2e-version` — Retrieves the distribution of DNSSEC-validated answers by end-to-end security status.
- `cloudflare-pp-cli radar get-dns-summary-by-ip-version` — Retrieves the distribution of DNS queries by IP version.
- `cloudflare-pp-cli radar get-dns-summary-by-matching-answer-status` — Retrieves the distribution of DNS queries by matching answers.
- `cloudflare-pp-cli radar get-dns-summary-by-protocol` — Retrieves the distribution of DNS queries by DNS transport protocol.
- `cloudflare-pp-cli radar get-dns-summary-by-query-type` — Retrieves the distribution of DNS queries by type.
- `cloudflare-pp-cli radar get-dns-summary-by-response-code` — Retrieves the distribution of DNS queries by response code.
- `cloudflare-pp-cli radar get-dns-summary-by-response-ttl` — Retrieves the distribution of DNS queries by minimum response TTL.
- `cloudflare-pp-cli radar get-dns-timeseries` — Retrieves normalized query volume to the 1.1.1.1 DNS resolver over time.
- `cloudflare-pp-cli radar get-dns-timeseries-group` — Retrieves the distribution of DNS queries grouped by dimension over time.
- `cloudflare-pp-cli radar get-dns-timeseries-group-by-cache-hit-status` — Retrieves the distribution of DNS queries by cache status over time.
- `cloudflare-pp-cli radar get-dns-timeseries-group-by-dnssec` — Retrieves the distribution of DNS responses by DNSSEC (DNS Security Extensions) support over time.
- `cloudflare-pp-cli radar get-dns-timeseries-group-by-dnssec-awareness` — Retrieves the distribution of DNS queries by DNSSEC (DNS Security Extensions) client awareness over time.
- `cloudflare-pp-cli radar get-dns-timeseries-group-by-dnssec-e2e-version` — Retrieves the distribution of DNSSEC-validated answers by end-to-end security status over time.
- `cloudflare-pp-cli radar get-dns-timeseries-group-by-ip-version` — Retrieves the distribution of DNS queries by IP version over time.
- `cloudflare-pp-cli radar get-dns-timeseries-group-by-matching-answer-status` — Retrieves the distribution of DNS queries by matching answers over time.
- `cloudflare-pp-cli radar get-dns-timeseries-group-by-protocol` — Retrieves the distribution of DNS queries by DNS transport protocol over time.
- `cloudflare-pp-cli radar get-dns-timeseries-group-by-query-type` — Retrieves the distribution of DNS queries by type over time.
- `cloudflare-pp-cli radar get-dns-timeseries-group-by-response-code` — Retrieves the distribution of DNS queries by response code over time.
- `cloudflare-pp-cli radar get-dns-timeseries-group-by-response-ttl` — Retrieves the distribution of DNS queries by minimum answer TTL over time.
- `cloudflare-pp-cli radar get-dns-top-ases` — Retrieves the top autonomous systems by DNS queries made to 1.1.1.1 DNS resolver.
- `cloudflare-pp-cli radar get-dns-top-locations` — Retrieves the top locations by DNS queries made to 1.1.1.1 DNS resolver.
- `cloudflare-pp-cli radar get-entities-asn-by-id` — Retrieves the requested autonomous system information. (A confidence level below `5` indicates a low level of...
- `cloudflare-pp-cli radar get-entities-asn-by-ip` — Retrieves the requested autonomous system information based on IP address. Population estimates come from APNIC...
- `cloudflare-pp-cli radar get-entities-asn-list` — Retrieves a list of autonomous systems.
- `cloudflare-pp-cli radar get-entities-ip` — Retrieves IP address information.
- `cloudflare-pp-cli radar get-entities-location-by-alpha2` — Retrieves the requested location information. (A confidence level below `5` indicates a low level of confidence in...
- `cloudflare-pp-cli radar get-entities-locations` — Retrieves a list of locations.
- `cloudflare-pp-cli radar get-geolocation-details` — Retrieves the requested Geolocation information. Geolocation names can be localized by sending an `Accept-Language`...
- `cloudflare-pp-cli radar get-geolocations` — Retrieves a list of geolocations. Geolocation names can be localized by sending an `Accept-Language` HTTP header...
- `cloudflare-pp-cli radar get-http-summary` — Retrieves the distribution of HTTP requests by the specified dimension.
- `cloudflare-pp-cli radar get-http-summary-by-bot-class` — Retrieves the distribution of bot-generated HTTP requests to genuine human traffic, as classified by Cloudflare....
- `cloudflare-pp-cli radar get-http-summary-by-device-type` — Retrieves the distribution of HTTP requests generated by mobile, desktop, and other types of devices.
- `cloudflare-pp-cli radar get-http-summary-by-http-protocol` — Retrieves the distribution of HTTP requests by HTTP protocol (HTTP vs. HTTPS).
- `cloudflare-pp-cli radar get-http-summary-by-http-version` — Retrieves the distribution of HTTP requests by HTTP version.
- `cloudflare-pp-cli radar get-http-summary-by-ip-version` — Retrieves the distribution of HTTP requests by IP version.
- `cloudflare-pp-cli radar get-http-summary-by-operating-system` — Retrieves the distribution of HTTP requests by operating system (Windows, macOS, Android, iOS, and others).
- `cloudflare-pp-cli radar get-http-summary-by-post-quantum` — Retrieves the distribution of HTTP requests by post-quantum support.
- `cloudflare-pp-cli radar get-http-summary-by-tls-version` — Retrieves the distribution of HTTP requests by TLS version.
- `cloudflare-pp-cli radar get-http-timeseries` — Retrieves the HTTP requests over time.
- `cloudflare-pp-cli radar get-http-timeseries-group` — Retrieves the distribution of HTTP requests grouped by dimension.
- `cloudflare-pp-cli radar get-http-timeseries-group-by-bot-class` — Retrieves the distribution of HTTP requests classified as automated or human over time. Visit...
- `cloudflare-pp-cli radar get-http-timeseries-group-by-browser-families` — Retrieves the distribution of HTTP requests by user agent family over time.
- `cloudflare-pp-cli radar get-http-timeseries-group-by-browsers` — Retrieves the distribution of HTTP requests by user agent over time.
- `cloudflare-pp-cli radar get-http-timeseries-group-by-device-type` — Retrieves the distribution of HTTP requests by device type over time.
- `cloudflare-pp-cli radar get-http-timeseries-group-by-http-protocol` — Retrieves the distribution of HTTP requests by HTTP protocol (HTTP vs. HTTPS) over time.
- `cloudflare-pp-cli radar get-http-timeseries-group-by-http-version` — Retrieves the distribution of HTTP requests by HTTP version over time.
- `cloudflare-pp-cli radar get-http-timeseries-group-by-ip-version` — Retrieves the distribution of HTTP requests by IP version over time.
- `cloudflare-pp-cli radar get-http-timeseries-group-by-operating-system` — Retrieves the distribution of HTTP requests by operating system over time.
- `cloudflare-pp-cli radar get-http-timeseries-group-by-post-quantum` — Retrieves the distribution of HTTP requests by post-quantum support over time.
- `cloudflare-pp-cli radar get-http-timeseries-group-by-tls-version` — Retrieves the distribution of HTTP requests by TLS version over time.
- `cloudflare-pp-cli radar get-http-top-ases-by-http-requests` — Retrieves the top autonomous systems by HTTP requests.
- `cloudflare-pp-cli radar get-http-top-browser-families` — Retrieves the top user agents, aggregated in families, by HTTP requests.
- `cloudflare-pp-cli radar get-http-top-browsers` — Retrieves the top user agents by HTTP requests.
- `cloudflare-pp-cli radar get-http-top-locations-by-http-requests` — Retrieves the top locations by HTTP requests.
- `cloudflare-pp-cli radar get-leaked-credential-checks-summary` — Retrieves an aggregated summary of HTTP authentication requests grouped by the specified dimension.
- `cloudflare-pp-cli radar get-leaked-credential-checks-summary-by-bot-class` — Retrieves the distribution of HTTP authentication requests by bot class.
- `cloudflare-pp-cli radar get-leaked-credential-checks-summary-by-compromised` — Retrieves the distribution of HTTP authentication requests by compromised credential status.
- `cloudflare-pp-cli radar get-leaked-credential-checks-timeseries-group` — Retrieves the distribution of HTTP authentication requests, grouped by the specified dimension over time.
- `cloudflare-pp-cli radar get-leaked-credential-checks-timeseries-group-by-bot-class` — Retrieves the distribution of HTTP authentication requests by bot class over time.
- `cloudflare-pp-cli radar get-leaked-credential-checks-timeseries-group-by-compromised` — Retrieves the distribution of HTTP authentication requests by compromised credential status over time.
- `cloudflare-pp-cli radar get-netflows-summary` — Retrieves the distribution of network traffic (NetFlows) by the specified dimension.
- `cloudflare-pp-cli radar get-netflows-summary-deprecated` — Retrieves the distribution of network traffic (NetFlows) by HTTP vs other protocols.
- `cloudflare-pp-cli radar get-netflows-timeseries` — Retrieves network traffic (NetFlows) over time.
- `cloudflare-pp-cli radar get-netflows-timeseries-group` — Retrieves the distribution of NetFlows traffic, grouped by the specified dimension over time.
- `cloudflare-pp-cli radar get-netflows-top-ases` — Retrieves the top autonomous systems by network traffic (NetFlows).
- `cloudflare-pp-cli radar get-netflows-top-locations` — Retrieves the top locations by network traffic (NetFlows).
- `cloudflare-pp-cli radar get-origin-details` — Retrieves the requested origin information with its regions.
- `cloudflare-pp-cli radar get-origins` — Retrieves a list of origins with their regions.
- `cloudflare-pp-cli radar get-origins-summary` — Retrieves an aggregated summary of origin metrics grouped by the specified dimension.
- `cloudflare-pp-cli radar get-origins-timeseries` — Retrieves the time series of origin metrics for the specified origin.
- `cloudflare-pp-cli radar get-origins-timeseries-group` — Retrieves the distribution of origin metrics grouped by the specified dimension over time.
- `cloudflare-pp-cli radar get-post-quantum-tls-support` — Tests whether a hostname or IP address supports Post-Quantum (PQ) TLS key exchange. Returns information about the...
- `cloudflare-pp-cli radar get-quality-index-summary` — Retrieves a summary (percentiles) of bandwidth, latency, or DNS response time from the Radar Internet Quality Index...
- `cloudflare-pp-cli radar get-quality-index-timeseries-group` — Retrieves a time series (percentiles) of bandwidth, latency, or DNS response time from the Radar Internet Quality...
- `cloudflare-pp-cli radar get-quality-speed-histogram` — Retrieves a histogram from the previous 90 days of Cloudflare Speed Test data, split into fixed bandwidth (Mbps),...
- `cloudflare-pp-cli radar get-quality-speed-summary` — Retrieves a summary of bandwidth, latency, jitter, and packet loss, from the previous 90 days of Cloudflare Speed...
- `cloudflare-pp-cli radar get-ranking-domain-details` — Retrieves domain rank details. Cloudflare provides an ordered rank for the top 100 domains, but for the remainder it...
- `cloudflare-pp-cli radar get-ranking-domain-timeseries` — Retrieves domains rank over time.
- `cloudflare-pp-cli radar get-ranking-internet-services-categories` — Retrieves the list of Internet services categories.
- `cloudflare-pp-cli radar get-ranking-internet-services-timeseries` — Retrieves Internet Services rank update changes over time.
- `cloudflare-pp-cli radar get-ranking-top-domains` — Retrieves the top or trending domains based on their rank. Popular domains are domains of broad appeal based on how...
- `cloudflare-pp-cli radar get-ranking-top-internet-services` — Retrieves top Internet services based on their rank.
- `cloudflare-pp-cli radar get-reports-dataset-download` — Retrieves the CSV content of a given dataset by alias or ID. When getting the content by alias the latest dataset is...
- `cloudflare-pp-cli radar get-reports-datasets` — Retrieves a list of datasets.
- `cloudflare-pp-cli radar get-robots-txt-top-domain-categories-by-files-parsed` — Retrieves the top domain categories by the number of robots.txt files parsed.
- `cloudflare-pp-cli radar get-search-global` — Searches for locations, autonomous systems, reports, bots, certificate logs, certificate authorities, industries and...
- `cloudflare-pp-cli radar get-tcp-resets-timeouts-summary` — Retrieves the distribution of connection stage by TCP connections terminated within the first 10 packets by a reset...
- `cloudflare-pp-cli radar get-tcp-resets-timeouts-timeseries-group` — Retrieves the distribution of connection stage by TCP connections terminated within the first 10 packets by a reset...
- `cloudflare-pp-cli radar get-tld-details` — Retrieves the requested TLD information.
- `cloudflare-pp-cli radar get-tlds` — Retrieves a list of TLDs.
- `cloudflare-pp-cli radar get-traffic-anomalies` — Retrieves the latest Internet traffic anomalies, which are signals that might indicate an outage. These alerts are...
- `cloudflare-pp-cli radar get-traffic-anomalies-top` — Retrieves the sum of Internet traffic anomalies, grouped by location. These anomalies are signals that might...
- `cloudflare-pp-cli radar get-verified-bots-top-by-http-requests` — Retrieves the top verified bots by HTTP requests, with owner and category.
- `cloudflare-pp-cli radar get-verified-bots-top-categories-by-http-requests` — Retrieves the top verified bot categories by HTTP requests, along with their corresponding percentage, over the...
- `cloudflare-pp-cli radar post-reports-dataset-download-url` — Retrieves an URL to download a single dataset.

**ready** — Manage ready

- `cloudflare-pp-cli ready` — Return a success message after running readiness checks

**signed-url** — Manage signed url

- `cloudflare-pp-cli signed-url` — Internal route for testing signed URLs

**system** — Manage system

- `cloudflare-pp-cli system secrets-store-create` — Creates a store in the account on behalf of the calling service. The store will be marked as managed by the...
- `cloudflare-pp-cli system secrets-store-delete-bulk` — Deletes one or more secrets from a store managed by the calling service. Returns 404 if the store doesn't exist or...
- `cloudflare-pp-cli system secrets-store-delete-by-id` — Deletes a store managed by the calling service. Returns 404 if the store doesn't exist or is not managed by the...
- `cloudflare-pp-cli system secrets-store-duplicate-by-id` — Duplicates a secret in a store managed by the calling service, keeping the value. Returns 404 if the store doesn't...
- `cloudflare-pp-cli system secrets-store-get-by-id` — Returns details of a single secret from a store managed by the calling service. Returns 404 if the store doesn't...
- `cloudflare-pp-cli system secrets-store-get-store-by-id` — Returns details of a single store managed by the calling service. Returns 404 if the store doesn't exist or is not...
- `cloudflare-pp-cli system secrets-store-list` — Lists all stores in an account that are managed by the calling service. Only returns stores where managed_by matches...
- `cloudflare-pp-cli system secrets-store-patch-by-id` — Updates a single secret in a store managed by the calling service. Returns 404 if the store doesn't exist or is not...
- `cloudflare-pp-cli system secrets-store-secret-create` — Creates one or more secrets in a store managed by the calling service. Returns 404 if the store doesn't exist or is...
- `cloudflare-pp-cli system secrets-store-secret-delete-by-id` — Deletes a single secret from a store managed by the calling service. Returns 404 if the store doesn't exist or is...
- `cloudflare-pp-cli system secrets-store-secrets-list` — Lists all secrets in a store managed by the calling service. Returns 404 if the store doesn't exist or is not...

**tenants** — Manage tenants

- `cloudflare-pp-cli tenants <tenant_id>` — Retrieves a Tenant by Tenant ID.

**user** — Manage user

- `cloudflare-pp-cli user api-tokens-create-token` — Create a new access token.
- `cloudflare-pp-cli user api-tokens-delete-token` — Destroy a token.
- `cloudflare-pp-cli user api-tokens-list-tokens` — List all access tokens you created.
- `cloudflare-pp-cli user api-tokens-roll-token` — Roll the token secret.
- `cloudflare-pp-cli user api-tokens-token-details` — Get information about a specific token.
- `cloudflare-pp-cli user api-tokens-update-token` — Update an existing token.
- `cloudflare-pp-cli user api-tokens-verify-token` — Test whether a token works.
- `cloudflare-pp-cli user audit-logs-get-audit-logs` — Gets a list of audit logs for a user account. Can be filtered by who made the change, on which zone, and the...
- `cloudflare-pp-cli user billing-history-deprecated-billing-history-details` — Accesses your billing history object.
- `cloudflare-pp-cli user billing-profile-deprecated-billing-profile-details` — Accesses your billing profile object.
- `cloudflare-pp-cli user details` — User Details
- `cloudflare-pp-cli user edit` — Edit part of your user details.
- `cloudflare-pp-cli user ip-access-rules-for-a-create-an-ip-access-rule` — Creates a new IP Access rule for all zones owned by the current user. Note: To create an IP Access rule that applies...
- `cloudflare-pp-cli user ip-access-rules-for-a-delete-an-ip-access-rule` — Deletes an IP Access rule at the user level. Note: Deleting a user-level rule will affect all zones owned by the user.
- `cloudflare-pp-cli user ip-access-rules-for-a-list-ip-access-rules` — Fetches IP Access rules of the user. You can filter the results using several optional parameters.
- `cloudflare-pp-cli user ip-access-rules-for-a-update-an-ip-access-rule` — Updates an IP Access rule defined at the user level. You can only update the rule action (`mode` parameter) and notes.
- `cloudflare-pp-cli user load-balancer-healthcheck-events-list-healthcheck-events` — List origin health changes.
- `cloudflare-pp-cli user load-balancer-monitors-create-monitor` — Create a configured monitor.
- `cloudflare-pp-cli user load-balancer-monitors-delete-monitor` — Delete a configured monitor.
- `cloudflare-pp-cli user load-balancer-monitors-list-monitor-references` — Get the list of resources that reference the provided monitor.
- `cloudflare-pp-cli user load-balancer-monitors-list-monitors` — List configured monitors for a user.
- `cloudflare-pp-cli user load-balancer-monitors-monitor-details` — List a single configured monitor for a user.
- `cloudflare-pp-cli user load-balancer-monitors-patch-monitor` — Apply changes to an existing monitor, overwriting the supplied properties.
- `cloudflare-pp-cli user load-balancer-monitors-preview-monitor` — Preview pools using the specified monitor with provided monitor details. The returned preview_id can be used in the...
- `cloudflare-pp-cli user load-balancer-monitors-preview-result` — Get the result of a previous preview operation using the provided preview_id.
- `cloudflare-pp-cli user load-balancer-monitors-update-monitor` — Modify a configured monitor.
- `cloudflare-pp-cli user load-balancer-pools-create-pool` — Create a new pool.
- `cloudflare-pp-cli user load-balancer-pools-delete-pool` — Delete a configured pool.
- `cloudflare-pp-cli user load-balancer-pools-list-pool-references` — Get the list of resources that reference the provided pool.
- `cloudflare-pp-cli user load-balancer-pools-list-pools` — List configured pools.
- `cloudflare-pp-cli user load-balancer-pools-patch-pool` — Apply changes to an existing pool, overwriting the supplied properties.
- `cloudflare-pp-cli user load-balancer-pools-patch-pools` — Apply changes to a number of existing pools, overwriting the supplied properties. Pools are ordered by ascending...
- `cloudflare-pp-cli user load-balancer-pools-pool-details` — Fetch a single configured pool.
- `cloudflare-pp-cli user load-balancer-pools-pool-health-details` — Fetch the latest pool health status for a single pool.
- `cloudflare-pp-cli user load-balancer-pools-preview-pool` — Preview pool health using provided monitor details. The returned preview_id can be used in the preview endpoint to...
- `cloudflare-pp-cli user load-balancer-pools-update-pool` — Modify a configured pool.
- `cloudflare-pp-cli user permission-groups-list-permission-groups` — Find all available permission groups for API Tokens.
- `cloudflare-pp-cli user s-invites-invitation-details` — Gets the details of an invitation.
- `cloudflare-pp-cli user s-invites-list-invitations` — Lists all invitations associated with my user.
- `cloudflare-pp-cli user s-invites-respond-to-invitation` — Responds to an invitation.
- `cloudflare-pp-cli user s-organizations-leave-organization` — Removes association to an organization.
- `cloudflare-pp-cli user s-organizations-list-organizations` — Lists organizations the user is associated with.
- `cloudflare-pp-cli user s-organizations-organization-details` — Gets a specific organization the user is associated with.
- `cloudflare-pp-cli user subscription-delete-subscription` — Deletes a user's subscription.
- `cloudflare-pp-cli user subscription-get-subscriptions` — Lists all of a user's subscriptions.
- `cloudflare-pp-cli user subscription-update-subscription` — Updates a user's subscriptions.

**users** — Manage users

- `cloudflare-pp-cli users` — Retrieves list of tenants the authenticated user / method has access to.

**workers** — Manage workers

- `cloudflare-pp-cli workers <deploy_hook_uuid>` — Trigger a build using a deploy hook. This endpoint does not require authentication - the deploy_hook_uuid acts as a...

**zones** — Manage zones

- `cloudflare-pp-cli zones 0-delete` — Deletes an existing zone.
- `cloudflare-pp-cli zones 0-get` — Zone Details
- `cloudflare-pp-cli zones 0-patch` — Edits a zone. Only one zone property can be changed at a time.
- `cloudflare-pp-cli zones get` — Lists, searches, sorts, and filters your zones. Listing zones across more than 500 accounts is currently not allowed.
- `cloudflare-pp-cli zones post` — Create Zone


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
cloudflare-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### End-to-end klinikalarm.dk setup

```bash
cloudflare-pp-cli setup_zone klinikalarm.dk --origin 157.173.102.165 --redirect-from "mitlæge.dk/*" --json
```

Provisions A record, optional redirect Page Rule, SSL strict, and Always-Use-HTTPS in one shot.

### Audit zone drift before promoting staging to prod

```bash
cloudflare-pp-cli zones diff staging.makertoo.win prod.makertoo.win --json --select drift,settings,page_rules,dns
```

Returns settings/page-rule/DNS deltas with --select narrowing the response so agents see only the relevant nested fields.

### Find every place a domain is wired

```bash
cloudflare-pp-cli where-is klinikalarm.dk --json
```

Live API calls across DNS records, Worker routes, and Page Rules (no local cache required).

### Idempotent DNS provisioning from a script

```bash
cloudflare-pp-cli dns apply --zone klinikalarm.dk --type A --name @ --content 157.173.102.165 --dry-run
```

Shows what would change without applying. Drop --dry-run to commit.

### Cache purge with verification

```bash
cloudflare-pp-cli cache purge release --zone klinikalarm.dk --tags release-v1 --probe https://klinikalarm.dk/ --json
```

Purges by tag, then probes the URL and asserts cf-cache-status transitions from MISS to HIT.

## Auth Setup

Cloudflare uses scoped API tokens (Bearer) — preferred over the legacy email+global-key pair. Set `CLOUDFLARE_API_TOKEN` once; the CLI auto-resolves account_id and zone_id from names so most commands take human-readable inputs.

Run `cloudflare-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  cloudflare-pp-cli accounts list --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag
- **Explicit retries** — use `--idempotent` only when an already-existing create should count as success, and `--ignore-missing` only when a missing delete target should count as success

### Response envelope

Commands that read from the local store or the API wrap output in a provenance envelope:

```json
{
  "meta": {"source": "live" | "local", "synced_at": "...", "reason": "..."},
  "results": <data>
}
```

Parse `.results` for data and `.meta.source` to know whether it's live or local. A human-readable `N results (live)` summary is printed to stderr only when stdout is a terminal — piped/agent consumers get pure JSON on stdout.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
cloudflare-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
cloudflare-pp-cli feedback --stdin < notes.txt
cloudflare-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.cloudflare-pp-cli/feedback.jsonl`. They are never POSTed unless `CLOUDFLARE_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `CLOUDFLARE_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

Write what *surprised* you, not a bug report. Short, specific, one line: that is the part that compounds.

## Output Delivery

Every command accepts `--deliver <sink>`. The output goes to the named sink in addition to (or instead of) stdout, so agents can route command results without hand-piping. Three sinks are supported:

| Sink | Effect |
|------|--------|
| `stdout` | Default; write to stdout only |
| `file:<path>` | Atomically write output to `<path>` (tmp + rename) |
| `webhook:<url>` | POST the output body to the URL (`application/json` or `application/x-ndjson` when `--compact`) |

Unknown schemes are refused with a structured error naming the supported set. Webhook failures return non-zero and log the URL + HTTP status on stderr.

## Named Profiles

A profile is a saved set of flag values, reused across invocations. Use it when a scheduled agent calls the same command every run with the same configuration - HeyGen's "Beacon" pattern.

```
cloudflare-pp-cli profile save briefing --json
cloudflare-pp-cli --profile briefing accounts list
cloudflare-pp-cli profile list --json
cloudflare-pp-cli profile show briefing
cloudflare-pp-cli profile delete briefing --yes
```

Explicit flags always win over profile values; profile values win over defaults. `agent-context` lists all available profiles under `available_profiles` so introspecting agents discover them at runtime.

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 2 | Usage error (wrong arguments) |
| 3 | Resource not found |
| 4 | Authentication required |
| 5 | API error (upstream issue) |
| 7 | Rate limited (wait and retry) |
| 10 | Config error |

## Argument Parsing

Parse `$ARGUMENTS`:

1. **Empty, `help`, or `--help`** → show `cloudflare-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add cloudflare-pp-mcp -- cloudflare-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which cloudflare-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   cloudflare-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `cloudflare-pp-cli <command> --help`.
