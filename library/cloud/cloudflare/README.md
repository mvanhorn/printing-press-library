# Cloudflare CLI

**The unified Cloudflare CLI agents and operators both want — every product covered, with offline cache, agent-native output, and the Cloudflare-pattern MCP surface.**

Wrangler is Workers-only, flarectl is dormant, Terraform is declarative-only. This CLI absorbs all three and adds cross-product transcendence: idempotent DNS apply, semantic redirects, multi-resolver propagation watch, where-is, zone drift diff. MCP exposes the entire ~3000-endpoint surface through a thin search+execute pair (the Cloudflare pattern) so agents don't burn tokens loading per-endpoint tools.

Learn more at [Cloudflare](https://developers.cloudflare.com/radar/).

## Install

The recommended path installs both the `cloudflare-pp-cli` binary and the `pp-cloudflare` agent skill in one shot:

```bash
npx -y @mvanhorn/printing-press install cloudflare
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install cloudflare --cli-only
```


### Without Node

The generated install path is category-agnostic until this CLI is published. If `npx` is not available before publish, install Node or use the category-specific Go fallback from the public-library entry after publish.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/cloudflare-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-cloudflare --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-cloudflare --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-cloudflare skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-cloudflare. The skill defines how its required CLI can be installed.
```

## Authentication

Cloudflare uses scoped API tokens (Bearer) — preferred over the legacy email+global-key pair. Set `CLOUDFLARE_API_TOKEN` once; the CLI auto-resolves account_id and zone_id from names so most commands take human-readable inputs.

## Quick Start

```bash
# verify your token works and capture account/zone IDs into local cache
cloudflare-pp-cli doctor


# snapshot zones, DNS records, workers, page rules, etc. for offline cross-product queries
cloudflare-pp-cli sync --full


# end-to-end zone setup in one command
cloudflare-pp-cli setup_zone klinikalarm.dk --origin 157.173.102.165


# verify global DNS propagation across public resolvers
cloudflare-pp-cli propagate watch klinikalarm.dk A --expect 157.173.102.165


# see every place this domain is wired across Cloudflare products
cloudflare-pp-cli where-is klinikalarm.dk --json

```

## Unique Features

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

## Usage

Run `cloudflare-pp-cli --help` for the full command reference and flag list.

## Commands

### accounts

Manage accounts

- **`cloudflare-pp-cli accounts batch-move`** - Batch move a collection of accounts to a specific organization. ⚠️ Not implemented.
- **`cloudflare-pp-cli accounts creation`** - Create an account (only available for tenant admins at this time)
- **`cloudflare-pp-cli accounts deletion`** - Delete a specific account (only available for tenant admins at this time). This is a permanent operation that will delete any zones or other resources under the account
- **`cloudflare-pp-cli accounts details`** - Get information about a specific account that you are a member of.
- **`cloudflare-pp-cli accounts list`** - List all accounts you have ownership or verified access to.
- **`cloudflare-pp-cli accounts update`** - Update an existing account.

### certificates

Manage certificates

- **`cloudflare-pp-cli certificates origin-ca-create`** - Create an Origin CA certificate. You can use an Origin CA Key as your User Service Key or an API token when calling this endpoint ([see above](#requests)).
- **`cloudflare-pp-cli certificates origin-ca-get`** - Get an existing Origin CA certificate by its serial number. You can use an Origin CA Key as your User Service Key or an API token when calling this endpoint ([see above](#requests)).
- **`cloudflare-pp-cli certificates origin-ca-list`** - List all existing Origin CA certificates for a given zone. You can use an Origin CA Key as your User Service Key or an API token when calling this endpoint ([see above](#requests)).
- **`cloudflare-pp-cli certificates origin-ca-revoke`** - Revoke an existing Origin CA certificate by its serial number. You can use an Origin CA Key as your User Service Key or an API token when calling this endpoint ([see above](#requests)).

### internal

Manage internal

- **`cloudflare-pp-cli internal create`** - Internal route for testing URL submissions

### ips

Manage ips

- **`cloudflare-pp-cli ips cloudflare-cloudflare-details`** - Get IPs used on the Cloudflare/JD Cloud network, see https://www.cloudflare.com/ips for Cloudflare IPs or https://developers.cloudflare.com/china-network/reference/infrastructure/ for JD Cloud IPs.

### live

Manage live

- **`cloudflare-pp-cli live list`** - Return a success message after running liveness checks

### memberships

Manage memberships

- **`cloudflare-pp-cli memberships user-s-account-delete`** - Remove the associated member from an account.
- **`cloudflare-pp-cli memberships user-s-account-details`** - Get a specific membership.
- **`cloudflare-pp-cli memberships user-s-account-list`** - List memberships of accounts the user can access.
- **`cloudflare-pp-cli memberships user-s-account-update`** - Accept or reject this account invitation.

### organizations

Manage organizations

- **`cloudflare-pp-cli organizations create-user`** - Create a new organization for a user. (Currently in Closed Beta - see https://developers.cloudflare.com/fundamentals/organizations/)
- **`cloudflare-pp-cli organizations delete`** - Delete an organization. The organization MUST be empty before deleting.
It must not contain any sub-organizations, accounts, members or users. (Currently in Closed Beta - see https://developers.cloudflare.com/fundamentals/organizations/)
- **`cloudflare-pp-cli organizations list`** - Retrieve a list of organizations a particular user has access to. (Currently in Closed Beta - see https://developers.cloudflare.com/fundamentals/organizations/)
- **`cloudflare-pp-cli organizations modify`** - Modify organization. (Currently in Closed Beta - see https://developers.cloudflare.com/fundamentals/organizations/)
- **`cloudflare-pp-cli organizations retrieve`** - Retrieve the details of a certain organization. (Currently in Closed Beta - see https://developers.cloudflare.com/fundamentals/organizations/)

### radar

Manage radar

- **`cloudflare-pp-cli radar get-agent-readiness-summary`** - Returns a summary of AI agent readiness scores across scanned domains, grouped by the specified dimension. Data is sourced from weekly bulk scans. All values are raw domain counts.
- **`cloudflare-pp-cli radar get-ai-bots-summary`** - Retrieves an aggregated summary of AI bots HTTP requests grouped by the specified dimension.
- **`cloudflare-pp-cli radar get-ai-bots-summary-by-user-agent`** - Retrieves the distribution of traffic by AI user agent.
- **`cloudflare-pp-cli radar get-ai-bots-timeseries`** - Retrieves AI bots HTTP request volume over time.
- **`cloudflare-pp-cli radar get-ai-bots-timeseries-group`** - Retrieves the distribution of HTTP requests from AI bots, grouped by the specified dimension over time.
- **`cloudflare-pp-cli radar get-ai-bots-timeseries-group-by-user-agent`** - Retrieves the distribution of traffic by AI user agent over time.
- **`cloudflare-pp-cli radar get-ai-inference-summary`** - Retrieves an aggregated summary of unique accounts using Workers AI inference grouped by the specified dimension.
- **`cloudflare-pp-cli radar get-ai-inference-summary-by-model`** - Retrieves the distribution of unique accounts by model.
- **`cloudflare-pp-cli radar get-ai-inference-summary-by-task`** - Retrieves the distribution of unique accounts by task.
- **`cloudflare-pp-cli radar get-ai-inference-timeseries-group`** - Retrieves the distribution of unique accounts using Workers AI inference, grouped by the specified dimension over time.
- **`cloudflare-pp-cli radar get-ai-inference-timeseries-group-by-model`** - Retrieves the distribution of unique accounts by model over time.
- **`cloudflare-pp-cli radar get-ai-inference-timeseries-group-by-task`** - Retrieves the distribution of unique accounts by task over time.
- **`cloudflare-pp-cli radar get-ai-markdown-for-agents-summary`** - Retrieves the overall median HTML-to-markdown reduction ratio for AI agent requests over the given date range.
- **`cloudflare-pp-cli radar get-ai-markdown-for-agents-timeseries`** - Retrieves the median HTML-to-markdown reduction ratio over time for AI agent requests.
- **`cloudflare-pp-cli radar get-annotations`** - Retrieves the latest annotations.
- **`cloudflare-pp-cli radar get-annotations-outages`** - Retrieves the latest Internet outages and anomalies.
- **`cloudflare-pp-cli radar get-annotations-outages-top`** - Retrieves the number of outages by location.
- **`cloudflare-pp-cli radar get-as-botnet-threat-feed`** - Retrieves a ranked list of Autonomous Systems based on their presence in the Cloudflare Botnet Threat Feed. Rankings can be sorted by offense count or number of bad IPs. Optionally compare to a previous date to see rank changes.
- **`cloudflare-pp-cli radar get-attacks-layer3-summary`** - Retrieves the distribution of layer 3 attacks by the specified dimension.
- **`cloudflare-pp-cli radar get-attacks-layer3-summary-by-bitrate`** - Retrieves the distribution of layer 3 attacks by bitrate.
- **`cloudflare-pp-cli radar get-attacks-layer3-summary-by-duration`** - Retrieves the distribution of layer 3 attacks by duration.
- **`cloudflare-pp-cli radar get-attacks-layer3-summary-by-industry`** - Retrieves the distribution of layer 3 attacks by targeted industry.
- **`cloudflare-pp-cli radar get-attacks-layer3-summary-by-ip-version`** - Retrieves the distribution of layer 3 attacks by IP version.
- **`cloudflare-pp-cli radar get-attacks-layer3-summary-by-protocol`** - Retrieves the distribution of layer 3 attacks by protocol.
- **`cloudflare-pp-cli radar get-attacks-layer3-summary-by-vector`** - Retrieves the distribution of layer 3 attacks by vector.
- **`cloudflare-pp-cli radar get-attacks-layer3-summary-by-vertical`** - Retrieves the distribution of layer 3 attacks by targeted vertical.
- **`cloudflare-pp-cli radar get-attacks-layer3-timeseries-by-bytes`** - Get layer 3 attacks by bytes time series
- **`cloudflare-pp-cli radar get-attacks-layer3-timeseries-group`** - Retrieves the distribution of layer 3 attacks grouped by dimension over time.
- **`cloudflare-pp-cli radar get-attacks-layer3-timeseries-group-by-bitrate`** - Retrieves the distribution of layer 3 attacks by bitrate over time.
- **`cloudflare-pp-cli radar get-attacks-layer3-timeseries-group-by-duration`** - Retrieves the distribution of layer 3 attacks by duration over time.
- **`cloudflare-pp-cli radar get-attacks-layer3-timeseries-group-by-industry`** - Retrieves the distribution of layer 3 attacks by targeted industry over time.
- **`cloudflare-pp-cli radar get-attacks-layer3-timeseries-group-by-ip-version`** - Retrieves the distribution of layer 3 attacks by IP version over time.
- **`cloudflare-pp-cli radar get-attacks-layer3-timeseries-group-by-protocol`** - Retrieves the distribution of layer 3 attacks by protocol over time.
- **`cloudflare-pp-cli radar get-attacks-layer3-timeseries-group-by-vector`** - Retrieves the distribution of layer 3 attacks by vector over time.
- **`cloudflare-pp-cli radar get-attacks-layer3-timeseries-group-by-vertical`** - Retrieves the distribution of layer 3 attacks by targeted vertical over time.
- **`cloudflare-pp-cli radar get-attacks-layer3-top-attacks`** - Retrieves the top layer 3 attacks from origin to target location. Values are a percentage out of the total layer 3 attacks (with billing country). You can optionally limit the number of attacks by origin/target location (useful if all the top attacks are from or to the same location).
- **`cloudflare-pp-cli radar get-attacks-layer3-top-industries`** - This endpoint is deprecated. To continue getting this data, switch to the summary by industry endpoint.
- **`cloudflare-pp-cli radar get-attacks-layer3-top-verticals`** - This endpoint is deprecated. To continue getting this data, switch to the summary by vertical endpoint.
- **`cloudflare-pp-cli radar get-attacks-layer7-summary`** - Retrieves the distribution of layer 7 attacks by the specified dimension.
- **`cloudflare-pp-cli radar get-attacks-layer7-summary-by-http-method`** - Retrieves the distribution of layer 7 attacks by HTTP method.
- **`cloudflare-pp-cli radar get-attacks-layer7-summary-by-http-version`** - Retrieves the distribution of layer 7 attacks by HTTP version.
- **`cloudflare-pp-cli radar get-attacks-layer7-summary-by-industry`** - Retrieves the distribution of layer 7 attacks by targeted industry.
- **`cloudflare-pp-cli radar get-attacks-layer7-summary-by-ip-version`** - Retrieves the distribution of layer 7 attacks by IP version.
- **`cloudflare-pp-cli radar get-attacks-layer7-summary-by-managed-rules`** - Retrieves the distribution of layer 7 attacks by managed rules.
- **`cloudflare-pp-cli radar get-attacks-layer7-summary-by-mitigation-product`** - Retrieves the distribution of layer 7 attacks by mitigation product.
- **`cloudflare-pp-cli radar get-attacks-layer7-summary-by-vertical`** - Retrieves the distribution of layer 7 attacks by targeted vertical.
- **`cloudflare-pp-cli radar get-attacks-layer7-timeseries`** - Retrieves layer 7 attacks over time.
- **`cloudflare-pp-cli radar get-attacks-layer7-timeseries-group`** - Retrieves the distribution of layer 7 attacks grouped by dimension over time.
- **`cloudflare-pp-cli radar get-attacks-layer7-timeseries-group-by-http-method`** - Retrieves the distribution of layer 7 attacks by HTTP method over time.
- **`cloudflare-pp-cli radar get-attacks-layer7-timeseries-group-by-http-version`** - Retrieves the distribution of layer 7 attacks by HTTP version over time.
- **`cloudflare-pp-cli radar get-attacks-layer7-timeseries-group-by-industry`** - Retrieves the distribution of layer 7 attacks by targeted industry over time.
- **`cloudflare-pp-cli radar get-attacks-layer7-timeseries-group-by-ip-version`** - Retrieves the distribution of layer 7 attacks by IP version used over time.
- **`cloudflare-pp-cli radar get-attacks-layer7-timeseries-group-by-managed-rules`** - Retrieves the distribution of layer 7 attacks by managed rules over time.
- **`cloudflare-pp-cli radar get-attacks-layer7-timeseries-group-by-mitigation-product`** - Retrieves the distribution of layer 7 attacks by mitigation product over time.
- **`cloudflare-pp-cli radar get-attacks-layer7-timeseries-group-by-vertical`** - Retrieves the distribution of layer 7 attacks by targeted vertical over time.
- **`cloudflare-pp-cli radar get-attacks-layer7-top-attacks`** - Retrieves the top attacks from origin to target location. Values are percentages of the total layer 7 attacks (with billing country). The attack magnitude can be defined by the number of mitigated requests or by the number of zones affected. You can optionally limit the number of attacks by origin/target location (useful if all the top attacks are from or to the same location).
- **`cloudflare-pp-cli radar get-attacks-layer7-top-industries`** - This endpoint is deprecated. To continue getting this data, switch to the summary by industry endpoint.
- **`cloudflare-pp-cli radar get-attacks-layer7-top-verticals`** - This endpoint is deprecated. To continue getting this data, switch to the summary by vertical endpoint.
- **`cloudflare-pp-cli radar get-bgp-hijacks-events`** - Retrieves the BGP hijack events.
- **`cloudflare-pp-cli radar get-bgp-ips-timeseries`** - Retrieves time series data for the announced IP space count, represented as the number of IPv4 /24s and IPv6 /48s, for a given ASN.
- **`cloudflare-pp-cli radar get-bgp-ips-top-ases`** - Returns the top-N autonomous systems by announced IP space at the nearest 8-hour RIB boundary at or before the requested date. The snapped boundary is returned as `anchor_ts`.
- **`cloudflare-pp-cli radar get-bgp-pfx2as`** - Retrieves the prefix-to-ASN mapping from global routing tables.
- **`cloudflare-pp-cli radar get-bgp-pfx2as-moas`** - Retrieves all Multi-Origin AS (MOAS) prefixes in the global routing tables.
- **`cloudflare-pp-cli radar get-bgp-route-leak-events`** - Retrieves the BGP route leak events.
- **`cloudflare-pp-cli radar get-bgp-routes-asns`** - Retrieves all ASes in the current global routing tables with routing statistics.
- **`cloudflare-pp-cli radar get-bgp-routes-realtime`** - Retrieves real-time BGP routes for a prefix, using public real-time data collectors (RouteViews and RIPE RIS).
- **`cloudflare-pp-cli radar get-bgp-routes-stats`** - Retrieves the BGP routing table stats.
- **`cloudflare-pp-cli radar get-bgp-rpki-aspa-changes`** - Retrieves ASPA (Autonomous System Provider Authorization) changes over time. Returns daily aggregated changes including additions, removals, and modifications of ASPA objects.
- **`cloudflare-pp-cli radar get-bgp-rpki-aspa-snapshot`** - Retrieves current or historical ASPA (Autonomous System Provider Authorization) objects. ASPA objects define which ASNs are authorized upstream providers for a customer ASN.
- **`cloudflare-pp-cli radar get-bgp-rpki-aspa-timeseries`** - Retrieves ASPA (Autonomous System Provider Authorization) object count over time. Supports filtering by RIR or location (country code) to generate multiple named series. If no RIR or location filter is specified, returns total count.
- **`cloudflare-pp-cli radar get-bgp-timeseries`** - Retrieves BGP updates over time. When requesting updates for an autonomous system, only BGP updates of type announcement are returned.
- **`cloudflare-pp-cli radar get-bgp-top-ases`** - Retrieves the top autonomous systems by BGP updates (announcements only).
- **`cloudflare-pp-cli radar get-bgp-top-prefixes`** - Retrieves the top network prefixes by BGP updates.
- **`cloudflare-pp-cli radar get-bot-details`** - Retrieves the requested bot information.
- **`cloudflare-pp-cli radar get-bots`** - Retrieves a list of bots.
- **`cloudflare-pp-cli radar get-bots-summary`** - Retrieves an aggregated summary of bots HTTP requests grouped by the specified dimension.
- **`cloudflare-pp-cli radar get-bots-timeseries`** - Retrieves bots HTTP request volume over time.
- **`cloudflare-pp-cli radar get-bots-timeseries-group`** - Retrieves the distribution of HTTP requests from bots, grouped by the specified dimension over time.
- **`cloudflare-pp-cli radar get-certificate-authorities`** - Retrieves a list of certificate authorities.
- **`cloudflare-pp-cli radar get-certificate-authority-details`** - Retrieves the requested CA information.
- **`cloudflare-pp-cli radar get-certificate-log-details`** - Retrieves the requested certificate log information.
- **`cloudflare-pp-cli radar get-certificate-logs`** - Retrieves a list of certificate logs.
- **`cloudflare-pp-cli radar get-ct-summary`** - Retrieves an aggregated summary of certificates grouped by the specified dimension.
- **`cloudflare-pp-cli radar get-ct-timeseries`** - Retrieves certificate volume over time.
- **`cloudflare-pp-cli radar get-ct-timeseries-group`** - Retrieves the distribution of certificates grouped by the specified dimension over time.
- **`cloudflare-pp-cli radar get-dns-as112-summary`** - Retrieves the distribution of AS112 queries by the specified dimension.
- **`cloudflare-pp-cli radar get-dns-as112-timeseries`** - Retrieves the AS112 DNS queries over time.
- **`cloudflare-pp-cli radar get-dns-as112-timeseries-by-dnssec`** - Retrieves the distribution of DNS queries to AS112 by DNSSEC (DNS Security Extensions) support.
- **`cloudflare-pp-cli radar get-dns-as112-timeseries-by-edns`** - Retrieves the distribution of DNS queries to AS112 by EDNS (Extension Mechanisms for DNS) support.
- **`cloudflare-pp-cli radar get-dns-as112-timeseries-by-ip-version`** - Retrieves the distribution of DNS queries to AS112 by IP version.
- **`cloudflare-pp-cli radar get-dns-as112-timeseries-by-protocol`** - Retrieves the distribution of DNS queries to AS112 by protocol.
- **`cloudflare-pp-cli radar get-dns-as112-timeseries-by-query-type`** - Retrieves the distribution of DNS queries to AS112 by type.
- **`cloudflare-pp-cli radar get-dns-as112-timeseries-by-response-codes`** - Retrieves the distribution of AS112 DNS requests classified by response code.
- **`cloudflare-pp-cli radar get-dns-as112-timeseries-group`** - Retrieves the distribution of AS112 queries grouped by dimension over time.
- **`cloudflare-pp-cli radar get-dns-as112-timeseries-group-by-dnssec`** - Retrieves the distribution of AS112 DNS queries by DNSSEC (DNS Security Extensions) support over time.
- **`cloudflare-pp-cli radar get-dns-as112-timeseries-group-by-edns`** - Retrieves the distribution of AS112 DNS queries by EDNS (Extension Mechanisms for DNS) support over time.
- **`cloudflare-pp-cli radar get-dns-as112-timeseries-group-by-ip-version`** - Retrieves the distribution of AS112 DNS queries by IP version over time.
- **`cloudflare-pp-cli radar get-dns-as112-timeseries-group-by-protocol`** - Retrieves the distribution of AS112 DNS requests classified by protocol over time.
- **`cloudflare-pp-cli radar get-dns-as112-timeseries-group-by-query-type`** - Retrieves the distribution of AS112 DNS queries by type over time.
- **`cloudflare-pp-cli radar get-dns-as112-timeseries-group-by-response-codes`** - Retrieves the distribution of AS112 DNS requests classified by response code over time.
- **`cloudflare-pp-cli radar get-dns-as112-top-locations`** - Retrieves the top locations by AS112 DNS queries.
- **`cloudflare-pp-cli radar get-dns-summary`** - Retrieves the distribution of DNS queries by the specified dimension.
- **`cloudflare-pp-cli radar get-dns-summary-by-cache-hit-status`** - Retrieves the distribution of DNS queries by cache status.
- **`cloudflare-pp-cli radar get-dns-summary-by-dnssec`** - Retrieves the distribution of DNS responses by DNSSEC (DNS Security Extensions) support.
- **`cloudflare-pp-cli radar get-dns-summary-by-dnssec-awareness`** - Retrieves the distribution of DNS queries by DNSSEC (DNS Security Extensions) client awareness.
- **`cloudflare-pp-cli radar get-dns-summary-by-dnssec-e2e-version`** - Retrieves the distribution of DNSSEC-validated answers by end-to-end security status.
- **`cloudflare-pp-cli radar get-dns-summary-by-ip-version`** - Retrieves the distribution of DNS queries by IP version.
- **`cloudflare-pp-cli radar get-dns-summary-by-matching-answer-status`** - Retrieves the distribution of DNS queries by matching answers.
- **`cloudflare-pp-cli radar get-dns-summary-by-protocol`** - Retrieves the distribution of DNS queries by DNS transport protocol.
- **`cloudflare-pp-cli radar get-dns-summary-by-query-type`** - Retrieves the distribution of DNS queries by type.
- **`cloudflare-pp-cli radar get-dns-summary-by-response-code`** - Retrieves the distribution of DNS queries by response code.
- **`cloudflare-pp-cli radar get-dns-summary-by-response-ttl`** - Retrieves the distribution of DNS queries by minimum response TTL.
- **`cloudflare-pp-cli radar get-dns-timeseries`** - Retrieves normalized query volume to the 1.1.1.1 DNS resolver over time.
- **`cloudflare-pp-cli radar get-dns-timeseries-group`** - Retrieves the distribution of DNS queries grouped by dimension over time.
- **`cloudflare-pp-cli radar get-dns-timeseries-group-by-cache-hit-status`** - Retrieves the distribution of DNS queries by cache status over time.
- **`cloudflare-pp-cli radar get-dns-timeseries-group-by-dnssec`** - Retrieves the distribution of DNS responses by DNSSEC (DNS Security Extensions) support over time.
- **`cloudflare-pp-cli radar get-dns-timeseries-group-by-dnssec-awareness`** - Retrieves the distribution of DNS queries by DNSSEC (DNS Security Extensions) client awareness over time.
- **`cloudflare-pp-cli radar get-dns-timeseries-group-by-dnssec-e2e-version`** - Retrieves the distribution of DNSSEC-validated answers by end-to-end security status over time.
- **`cloudflare-pp-cli radar get-dns-timeseries-group-by-ip-version`** - Retrieves the distribution of DNS queries by IP version over time.
- **`cloudflare-pp-cli radar get-dns-timeseries-group-by-matching-answer-status`** - Retrieves the distribution of DNS queries by matching answers over time.
- **`cloudflare-pp-cli radar get-dns-timeseries-group-by-protocol`** - Retrieves the distribution of DNS queries by DNS transport protocol over time.
- **`cloudflare-pp-cli radar get-dns-timeseries-group-by-query-type`** - Retrieves the distribution of DNS queries by type over time.
- **`cloudflare-pp-cli radar get-dns-timeseries-group-by-response-code`** - Retrieves the distribution of DNS queries by response code over time.
- **`cloudflare-pp-cli radar get-dns-timeseries-group-by-response-ttl`** - Retrieves the distribution of DNS queries by minimum answer TTL over time.
- **`cloudflare-pp-cli radar get-dns-top-ases`** - Retrieves the top autonomous systems by DNS queries made to 1.1.1.1 DNS resolver.
- **`cloudflare-pp-cli radar get-dns-top-locations`** - Retrieves the top locations by DNS queries made to 1.1.1.1 DNS resolver.
- **`cloudflare-pp-cli radar get-entities-asn-by-id`** - Retrieves the requested autonomous system information. (A confidence level below `5` indicates a low level of confidence in the traffic data - normally this happens because Cloudflare has a small amount of traffic from/to this AS). Population estimates come from APNIC (refer to https://labs.apnic.net/?p=526).
- **`cloudflare-pp-cli radar get-entities-asn-by-ip`** - Retrieves the requested autonomous system information based on IP address. Population estimates come from APNIC (refer to https://labs.apnic.net/?p=526).
- **`cloudflare-pp-cli radar get-entities-asn-list`** - Retrieves a list of autonomous systems.
- **`cloudflare-pp-cli radar get-entities-ip`** - Retrieves IP address information.
- **`cloudflare-pp-cli radar get-entities-location-by-alpha2`** - Retrieves the requested location information. (A confidence level below `5` indicates a low level of confidence in the traffic data - normally this happens because Cloudflare has a small amount of traffic from/to this location).
- **`cloudflare-pp-cli radar get-entities-locations`** - Retrieves a list of locations.
- **`cloudflare-pp-cli radar get-geolocation-details`** - Retrieves the requested Geolocation information. Geolocation names can be localized by sending an `Accept-Language` HTTP header with a BCP 47 language tag (e.g., `Accept-Language: pt-PT`). The full quality-value chain is supported (e.g., `pt-PT,pt;q=0.9,en;q=0.8`).
- **`cloudflare-pp-cli radar get-geolocations`** - Retrieves a list of geolocations. Geolocation names can be localized by sending an `Accept-Language` HTTP header with a BCP 47 language tag (e.g., `Accept-Language: pt-PT`). The full quality-value chain is supported (e.g., `pt-PT,pt;q=0.9,en;q=0.8`).
- **`cloudflare-pp-cli radar get-http-summary`** - Retrieves the distribution of HTTP requests by the specified dimension.
- **`cloudflare-pp-cli radar get-http-summary-by-bot-class`** - Retrieves the distribution of bot-generated HTTP requests to genuine human traffic, as classified by Cloudflare. Visit https://developers.cloudflare.com/radar/concepts/bot-classes/ for more information.
- **`cloudflare-pp-cli radar get-http-summary-by-device-type`** - Retrieves the distribution of HTTP requests generated by mobile, desktop, and other types of devices.
- **`cloudflare-pp-cli radar get-http-summary-by-http-protocol`** - Retrieves the distribution of HTTP requests by HTTP protocol (HTTP vs. HTTPS).
- **`cloudflare-pp-cli radar get-http-summary-by-http-version`** - Retrieves the distribution of HTTP requests by HTTP version.
- **`cloudflare-pp-cli radar get-http-summary-by-ip-version`** - Retrieves the distribution of HTTP requests by IP version.
- **`cloudflare-pp-cli radar get-http-summary-by-operating-system`** - Retrieves the distribution of HTTP requests by operating system (Windows, macOS, Android, iOS, and others).
- **`cloudflare-pp-cli radar get-http-summary-by-post-quantum`** - Retrieves the distribution of HTTP requests by post-quantum support.
- **`cloudflare-pp-cli radar get-http-summary-by-tls-version`** - Retrieves the distribution of HTTP requests by TLS version.
- **`cloudflare-pp-cli radar get-http-timeseries`** - Retrieves the HTTP requests over time.
- **`cloudflare-pp-cli radar get-http-timeseries-group`** - Retrieves the distribution of HTTP requests grouped by dimension.
- **`cloudflare-pp-cli radar get-http-timeseries-group-by-bot-class`** - Retrieves the distribution of HTTP requests classified as automated or human over time. Visit https://developers.cloudflare.com/radar/concepts/bot-classes/ for more information.
- **`cloudflare-pp-cli radar get-http-timeseries-group-by-browser-families`** - Retrieves the distribution of HTTP requests by user agent family over time.
- **`cloudflare-pp-cli radar get-http-timeseries-group-by-browsers`** - Retrieves the distribution of HTTP requests by user agent over time.
- **`cloudflare-pp-cli radar get-http-timeseries-group-by-device-type`** - Retrieves the distribution of HTTP requests by device type over time.
- **`cloudflare-pp-cli radar get-http-timeseries-group-by-http-protocol`** - Retrieves the distribution of HTTP requests by HTTP protocol (HTTP vs. HTTPS) over time.
- **`cloudflare-pp-cli radar get-http-timeseries-group-by-http-version`** - Retrieves the distribution of HTTP requests by HTTP version over time.
- **`cloudflare-pp-cli radar get-http-timeseries-group-by-ip-version`** - Retrieves the distribution of HTTP requests by IP version over time.
- **`cloudflare-pp-cli radar get-http-timeseries-group-by-operating-system`** - Retrieves the distribution of HTTP requests by operating system over time.
- **`cloudflare-pp-cli radar get-http-timeseries-group-by-post-quantum`** - Retrieves the distribution of HTTP requests by post-quantum support over time.
- **`cloudflare-pp-cli radar get-http-timeseries-group-by-tls-version`** - Retrieves the distribution of HTTP requests by TLS version over time.
- **`cloudflare-pp-cli radar get-http-top-ases-by-http-requests`** - Retrieves the top autonomous systems by HTTP requests.
- **`cloudflare-pp-cli radar get-http-top-browser-families`** - Retrieves the top user agents, aggregated in families, by HTTP requests.
- **`cloudflare-pp-cli radar get-http-top-browsers`** - Retrieves the top user agents by HTTP requests.
- **`cloudflare-pp-cli radar get-http-top-locations-by-http-requests`** - Retrieves the top locations by HTTP requests.
- **`cloudflare-pp-cli radar get-leaked-credential-checks-summary`** - Retrieves an aggregated summary of HTTP authentication requests grouped by the specified dimension.
- **`cloudflare-pp-cli radar get-leaked-credential-checks-summary-by-bot-class`** - Retrieves the distribution of HTTP authentication requests by bot class.
- **`cloudflare-pp-cli radar get-leaked-credential-checks-summary-by-compromised`** - Retrieves the distribution of HTTP authentication requests by compromised credential status.
- **`cloudflare-pp-cli radar get-leaked-credential-checks-timeseries-group`** - Retrieves the distribution of HTTP authentication requests, grouped by the specified dimension over time.
- **`cloudflare-pp-cli radar get-leaked-credential-checks-timeseries-group-by-bot-class`** - Retrieves the distribution of HTTP authentication requests by bot class over time.
- **`cloudflare-pp-cli radar get-leaked-credential-checks-timeseries-group-by-compromised`** - Retrieves the distribution of HTTP authentication requests by compromised credential status over time.
- **`cloudflare-pp-cli radar get-netflows-summary`** - Retrieves the distribution of network traffic (NetFlows) by the specified dimension.
- **`cloudflare-pp-cli radar get-netflows-summary-deprecated`** - Retrieves the distribution of network traffic (NetFlows) by HTTP vs other protocols.
- **`cloudflare-pp-cli radar get-netflows-timeseries`** - Retrieves network traffic (NetFlows) over time.
- **`cloudflare-pp-cli radar get-netflows-timeseries-group`** - Retrieves the distribution of NetFlows traffic, grouped by the specified dimension over time.
- **`cloudflare-pp-cli radar get-netflows-top-ases`** - Retrieves the top autonomous systems by network traffic (NetFlows).
- **`cloudflare-pp-cli radar get-netflows-top-locations`** - Retrieves the top locations by network traffic (NetFlows).
- **`cloudflare-pp-cli radar get-origin-details`** - Retrieves the requested origin information with its regions.
- **`cloudflare-pp-cli radar get-origins`** - Retrieves a list of origins with their regions.
- **`cloudflare-pp-cli radar get-origins-summary`** - Retrieves an aggregated summary of origin metrics grouped by the specified dimension.
- **`cloudflare-pp-cli radar get-origins-timeseries`** - Retrieves the time series of origin metrics for the specified origin.
- **`cloudflare-pp-cli radar get-origins-timeseries-group`** - Retrieves the distribution of origin metrics grouped by the specified dimension over time.
- **`cloudflare-pp-cli radar get-post-quantum-tls-support`** - Tests whether a hostname or IP address supports Post-Quantum (PQ) TLS key exchange. Returns information about the negotiated key exchange algorithm and whether it uses PQ cryptography.
- **`cloudflare-pp-cli radar get-quality-index-summary`** - Retrieves a summary (percentiles) of bandwidth, latency, or DNS response time from the Radar Internet Quality Index (IQI).
- **`cloudflare-pp-cli radar get-quality-index-timeseries-group`** - Retrieves a time series (percentiles) of bandwidth, latency, or DNS response time from the Radar Internet Quality Index (IQI).
- **`cloudflare-pp-cli radar get-quality-speed-histogram`** - Retrieves a histogram from the previous 90 days of Cloudflare Speed Test data, split into fixed bandwidth (Mbps), latency (ms), or jitter (ms) buckets.
- **`cloudflare-pp-cli radar get-quality-speed-summary`** - Retrieves a summary of bandwidth, latency, jitter, and packet loss, from the previous 90 days of Cloudflare Speed Test data.
- **`cloudflare-pp-cli radar get-ranking-domain-details`** - Retrieves domain rank details. Cloudflare provides an ordered rank for the top 100 domains, but for the remainder it only provides ranking buckets like top 200 thousand, top one million, etc.. These are available through Radar datasets endpoints.
- **`cloudflare-pp-cli radar get-ranking-domain-timeseries`** - Retrieves domains rank over time.
- **`cloudflare-pp-cli radar get-ranking-internet-services-categories`** - Retrieves the list of Internet services categories.
- **`cloudflare-pp-cli radar get-ranking-internet-services-timeseries`** - Retrieves Internet Services rank update changes over time.
- **`cloudflare-pp-cli radar get-ranking-top-domains`** - Retrieves the top or trending domains based on their rank. Popular domains are domains of broad appeal based on how people use the Internet. Trending domains are domains that are generating a surge in interest. For more information on top domains, see https://blog.cloudflare.com/radar-domain-rankings/.
- **`cloudflare-pp-cli radar get-ranking-top-internet-services`** - Retrieves top Internet services based on their rank.
- **`cloudflare-pp-cli radar get-reports-dataset-download`** - Retrieves the CSV content of a given dataset by alias or ID. When getting the content by alias the latest dataset is returned, optionally filtered by the latest available at a given date.
- **`cloudflare-pp-cli radar get-reports-datasets`** - Retrieves a list of datasets.
- **`cloudflare-pp-cli radar get-robots-txt-top-domain-categories-by-files-parsed`** - Retrieves the top domain categories by the number of robots.txt files parsed.
- **`cloudflare-pp-cli radar get-search-global`** - Searches for locations, autonomous systems, reports, bots, certificate logs, certificate authorities, industries and verticals. Location names can be localized by sending an `Accept-Language` HTTP header with a BCP 47 language tag (e.g., `Accept-Language: pt-PT`). The full quality-value chain is supported (e.g., `pt-PT,pt;q=0.9,en;q=0.8`).
- **`cloudflare-pp-cli radar get-tcp-resets-timeouts-summary`** - Retrieves the distribution of connection stage by TCP connections terminated within the first 10 packets by a reset or timeout.
- **`cloudflare-pp-cli radar get-tcp-resets-timeouts-timeseries-group`** - Retrieves the distribution of connection stage by TCP connections terminated within the first 10 packets by a reset or timeout over time.
- **`cloudflare-pp-cli radar get-tld-details`** - Retrieves the requested TLD information.
- **`cloudflare-pp-cli radar get-tlds`** - Retrieves a list of TLDs.
- **`cloudflare-pp-cli radar get-traffic-anomalies`** - Retrieves the latest Internet traffic anomalies, which are signals that might indicate an outage. These alerts are automatically detected by Radar and manually verified by our team.
- **`cloudflare-pp-cli radar get-traffic-anomalies-top`** - Retrieves the sum of Internet traffic anomalies, grouped by location. These anomalies are signals that might indicate an outage, automatically detected by Radar and manually verified by our team.
- **`cloudflare-pp-cli radar get-verified-bots-top-by-http-requests`** - Retrieves the top verified bots by HTTP requests, with owner and category.
- **`cloudflare-pp-cli radar get-verified-bots-top-categories-by-http-requests`** - Retrieves the top verified bot categories by HTTP requests, along with their corresponding percentage, over the total verified bot HTTP requests.
- **`cloudflare-pp-cli radar post-reports-dataset-download-url`** - Retrieves an URL to download a single dataset.

### ready

Manage ready

- **`cloudflare-pp-cli ready list`** - Return a success message after running readiness checks

### signed-url

Manage signed url

- **`cloudflare-pp-cli signed-url list`** - Internal route for testing signed URLs

### system

Manage system

- **`cloudflare-pp-cli system secrets-store-create`** - Creates a store in the account on behalf of the calling service.
The store will be marked as managed by the authenticated service.
Requires account_id in the request body.
- **`cloudflare-pp-cli system secrets-store-delete-bulk`** - Deletes one or more secrets from a store managed by the calling service.
Returns 404 if the store doesn't exist or is not managed by the authenticated service.
- **`cloudflare-pp-cli system secrets-store-delete-by-id`** - Deletes a store managed by the calling service.
Returns 404 if the store doesn't exist or is not managed by the authenticated service.
- **`cloudflare-pp-cli system secrets-store-duplicate-by-id`** - Duplicates a secret in a store managed by the calling service, keeping the value.
Returns 404 if the store doesn't exist or is not managed by the authenticated service.
- **`cloudflare-pp-cli system secrets-store-get-by-id`** - Returns details of a single secret from a store managed by the calling service.
Returns 404 if the store doesn't exist or is not managed by the authenticated service.
- **`cloudflare-pp-cli system secrets-store-get-store-by-id`** - Returns details of a single store managed by the calling service.
Returns 404 if the store doesn't exist or is not managed by the authenticated service.
- **`cloudflare-pp-cli system secrets-store-list`** - Lists all stores in an account that are managed by the calling service.
Only returns stores where managed_by matches the authenticated service.
- **`cloudflare-pp-cli system secrets-store-patch-by-id`** - Updates a single secret in a store managed by the calling service.
Returns 404 if the store doesn't exist or is not managed by the authenticated service.
- **`cloudflare-pp-cli system secrets-store-secret-create`** - Creates one or more secrets in a store managed by the calling service.
Returns 404 if the store doesn't exist or is not managed by the authenticated service.
- **`cloudflare-pp-cli system secrets-store-secret-delete-by-id`** - Deletes a single secret from a store managed by the calling service.
Returns 404 if the store doesn't exist or is not managed by the authenticated service.
- **`cloudflare-pp-cli system secrets-store-secrets-list`** - Lists all secrets in a store managed by the calling service.
Returns 404 if the store doesn't exist or is not managed by the authenticated service.

### tenants

Manage tenants

- **`cloudflare-pp-cli tenants retrieve`** - Retrieves a Tenant by Tenant ID.

### user

Manage user

- **`cloudflare-pp-cli user api-tokens-create-token`** - Create a new access token.
- **`cloudflare-pp-cli user api-tokens-delete-token`** - Destroy a token.
- **`cloudflare-pp-cli user api-tokens-list-tokens`** - List all access tokens you created.
- **`cloudflare-pp-cli user api-tokens-roll-token`** - Roll the token secret.
- **`cloudflare-pp-cli user api-tokens-token-details`** - Get information about a specific token.
- **`cloudflare-pp-cli user api-tokens-update-token`** - Update an existing token.
- **`cloudflare-pp-cli user api-tokens-verify-token`** - Test whether a token works.
- **`cloudflare-pp-cli user audit-logs-get-audit-logs`** - Gets a list of audit logs for a user account. Can be filtered by who made the change, on which zone, and the timeframe of the change.
- **`cloudflare-pp-cli user billing-history-deprecated-billing-history-details`** - Accesses your billing history object.
- **`cloudflare-pp-cli user billing-profile-deprecated-billing-profile-details`** - Accesses your billing profile object.
- **`cloudflare-pp-cli user details`** - User Details
- **`cloudflare-pp-cli user edit`** - Edit part of your user details.
- **`cloudflare-pp-cli user ip-access-rules-for-a-create-an-ip-access-rule`** - Creates a new IP Access rule for all zones owned by the current user.

Note: To create an IP Access rule that applies to a specific zone, refer to the [IP Access rules for a zone](#ip-access-rules-for-a-zone) endpoints.
- **`cloudflare-pp-cli user ip-access-rules-for-a-delete-an-ip-access-rule`** - Deletes an IP Access rule at the user level.

Note: Deleting a user-level rule will affect all zones owned by the user.
- **`cloudflare-pp-cli user ip-access-rules-for-a-list-ip-access-rules`** - Fetches IP Access rules of the user. You can filter the results using several optional parameters.
- **`cloudflare-pp-cli user ip-access-rules-for-a-update-an-ip-access-rule`** - Updates an IP Access rule defined at the user level. You can only update the rule action (`mode` parameter) and notes.
- **`cloudflare-pp-cli user load-balancer-healthcheck-events-list-healthcheck-events`** - List origin health changes.
- **`cloudflare-pp-cli user load-balancer-monitors-create-monitor`** - Create a configured monitor.
- **`cloudflare-pp-cli user load-balancer-monitors-delete-monitor`** - Delete a configured monitor.
- **`cloudflare-pp-cli user load-balancer-monitors-list-monitor-references`** - Get the list of resources that reference the provided monitor.
- **`cloudflare-pp-cli user load-balancer-monitors-list-monitors`** - List configured monitors for a user.
- **`cloudflare-pp-cli user load-balancer-monitors-monitor-details`** - List a single configured monitor for a user.
- **`cloudflare-pp-cli user load-balancer-monitors-patch-monitor`** - Apply changes to an existing monitor, overwriting the supplied properties.
- **`cloudflare-pp-cli user load-balancer-monitors-preview-monitor`** - Preview pools using the specified monitor with provided monitor details. The returned preview_id can be used in the preview endpoint to retrieve the results.
- **`cloudflare-pp-cli user load-balancer-monitors-preview-result`** - Get the result of a previous preview operation using the provided preview_id.
- **`cloudflare-pp-cli user load-balancer-monitors-update-monitor`** - Modify a configured monitor.
- **`cloudflare-pp-cli user load-balancer-pools-create-pool`** - Create a new pool.
- **`cloudflare-pp-cli user load-balancer-pools-delete-pool`** - Delete a configured pool.
- **`cloudflare-pp-cli user load-balancer-pools-list-pool-references`** - Get the list of resources that reference the provided pool.
- **`cloudflare-pp-cli user load-balancer-pools-list-pools`** - List configured pools.
- **`cloudflare-pp-cli user load-balancer-pools-patch-pool`** - Apply changes to an existing pool, overwriting the supplied properties.
- **`cloudflare-pp-cli user load-balancer-pools-patch-pools`** - Apply changes to a number of existing pools, overwriting the supplied properties. Pools are ordered by ascending `name`. Returns the list of affected pools. Supports the standard pagination query parameters, either `limit`/`offset` or `per_page`/`page`.
- **`cloudflare-pp-cli user load-balancer-pools-pool-details`** - Fetch a single configured pool.
- **`cloudflare-pp-cli user load-balancer-pools-pool-health-details`** - Fetch the latest pool health status for a single pool.
- **`cloudflare-pp-cli user load-balancer-pools-preview-pool`** - Preview pool health using provided monitor details. The returned preview_id can be used in the preview endpoint to retrieve the results.
- **`cloudflare-pp-cli user load-balancer-pools-update-pool`** - Modify a configured pool.
- **`cloudflare-pp-cli user permission-groups-list-permission-groups`** - Find all available permission groups for API Tokens.
- **`cloudflare-pp-cli user s-invites-invitation-details`** - Gets the details of an invitation.
- **`cloudflare-pp-cli user s-invites-list-invitations`** - Lists all invitations associated with my user.
- **`cloudflare-pp-cli user s-invites-respond-to-invitation`** - Responds to an invitation.
- **`cloudflare-pp-cli user s-organizations-leave-organization`** - Removes association to an organization.
- **`cloudflare-pp-cli user s-organizations-list-organizations`** - Lists organizations the user is associated with.
- **`cloudflare-pp-cli user s-organizations-organization-details`** - Gets a specific organization the user is associated with.
- **`cloudflare-pp-cli user subscription-delete-subscription`** - Deletes a user's subscription.
- **`cloudflare-pp-cli user subscription-get-subscriptions`** - Lists all of a user's subscriptions.
- **`cloudflare-pp-cli user subscription-update-subscription`** - Updates a user's subscriptions.

### users

Manage users

- **`cloudflare-pp-cli users list-tenants`** - Retrieves list of tenants the authenticated user / method has access to.

### workers

Manage workers

- **`cloudflare-pp-cli workers trigger-deploy-hook`** - Trigger a build using a deploy hook. This endpoint does not require authentication - the deploy_hook_uuid acts as a secret token.

### zones

Manage zones

- **`cloudflare-pp-cli zones 0-delete`** - Deletes an existing zone.
- **`cloudflare-pp-cli zones 0-get`** - Zone Details
- **`cloudflare-pp-cli zones 0-patch`** - Edits a zone. Only one zone property can be changed at a time.
- **`cloudflare-pp-cli zones get`** - Lists, searches, sorts, and filters your zones. Listing zones across more than 500 accounts
is currently not allowed.
- **`cloudflare-pp-cli zones post`** - Create Zone


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
cloudflare-pp-cli accounts list

# JSON for scripting and agents
cloudflare-pp-cli accounts list --json

# Filter to specific fields
cloudflare-pp-cli accounts list --json --select id,name,status

# Dry run — show the request without sending
cloudflare-pp-cli accounts list --dry-run

# Agent mode — JSON + compact + no prompts in one flag
cloudflare-pp-cli accounts list --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Explicit retries** - add `--idempotent` to create retries and `--ignore-missing` to delete retries when a no-op success is acceptable
- **Confirmable** - `--yes` for explicit confirmation of destructive actions
- **Piped input** - write commands can accept structured input when their help lists `--stdin`
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Use with Claude Code

Install the focused skill — it auto-installs the CLI on first invocation:

```bash
npx skills add mvanhorn/printing-press-library/cli-skills/pp-cloudflare -g
```

Then invoke `/pp-cloudflare <query>` in Claude Code. The skill is the most efficient path — Claude Code drives the CLI directly without an MCP server in the middle.

<details>
<summary>Use as an MCP server in Claude Code (advanced)</summary>

If you'd rather register this CLI as an MCP server in Claude Code, install the MCP binary first:


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Then register it:

```bash
claude mcp add cloudflare cloudflare-pp-mcp -e CLOUDFLARE_API_TOKEN=<your-token>
```

</details>

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/cloudflare-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `CLOUDFLARE_API_TOKEN` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "cloudflare": {
      "command": "cloudflare-pp-mcp",
      "env": {
        "CLOUDFLARE_API_TOKEN": "<your-token>"
      }
    }
  }
}
```

</details>

## Health Check

```bash
cloudflare-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/cloudflare-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `CLOUDFLARE_API_TOKEN` | per_call | Yes (preferred) | Bearer token; create at dash.cloudflare.com → My Profile → API Tokens. |
| `CLOUDFLARE_API_EMAIL` | per_call | No (legacy) | Account email for the legacy global-key auth pair. Use only when you can't use a scoped token. |
| `CLOUDFLARE_API_KEY` | per_call | No (legacy) | Global API key paired with `CLOUDFLARE_API_EMAIL`. Use only when you can't use a scoped token. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `cloudflare-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $CLOUDFLARE_API_TOKEN`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **401 / 403 from API** — run `cloudflare-pp-cli doctor`; if token is valid, the token's permissions don't cover the resource — Cloudflare API tokens are scoped
- **rate limit (429)** — the CLI auto-retries with the Retry-After header; if persistent, check `audit logs` for token-bound abuse alerts
- **search returns no results** — run `cloudflare-pp-cli sync --full` to refresh the local cache; cross-product joins read from local SQLite

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**wrangler**](https://github.com/cloudflare/workers-sdk) — TypeScript (4000 stars)
- [**flarectl**](https://github.com/cloudflare/cloudflare-go/tree/v0/cmd/flarectl) — Go (2000 stars)
- [**cloudflare-go**](https://github.com/cloudflare/cloudflare-go) — Go (2000 stars)
- [**terraform-provider-cloudflare**](https://github.com/cloudflare/terraform-provider-cloudflare) — Go (1300 stars)
- [**cf-terraforming**](https://github.com/cloudflare/cf-terraforming) — Go
- [**cloudflare-cli**](https://github.com/danielpigott/cloudflare-cli) — JavaScript
- [**mcp-server-cloudflare**](https://github.com/cloudflare/mcp-server-cloudflare) — TypeScript

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)

## Known Gaps

- **Scorecard `--spec` semantic validation:** Cloudflare's official OpenAPI at
  `cloudflare/api-schemas` references a `bearerAuth` security scheme that the
  printing-press scorecard's strict semantic validator can't resolve at the
  expected scope. The CLI itself scores **84/100 (Grade A)** when scorecard
  runs without `--spec`; the failure is a spec-validation tooling artifact,
  not a CLI quality issue. Run `printing-press scorecard --dir <dir>` (no
  `--spec`) for the real score.
