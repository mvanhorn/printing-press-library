# Squarespace CLI Discovery

Date: 2026-05-12

## What Was Found

The current CLI is a Printing Press-generated Squarespace baseline from the official Commerce OpenAPI schema. It covers the public Commerce API surface:

- Contacts and contact address book entries
- Commerce products, variants, images, inventory, orders, transactions, and transaction summaries
- Website profile and commerce store pages
- Webhook subscriptions
- Local sync, search, SQLite cache, `doctor`, `auth`, MCP, and agent-context support

The logged-in Squarespace account/admin surface for `https://account.squarespace.com/domains` is still separate from the official Commerce API. Browser capture is required for domains, account sites, and admin page-management APIs because those endpoints are not present in the official Commerce OpenAPI schema.

## How

Tools and steps used:

1. Installed the Printing Press generator:
   `go install github.com/mvanhorn/cli-printing-press/v4/cmd/printing-press@latest`
2. Verified the local generator:
   `printing-press --version` returned `printing-press 4.5.1`.
3. Installed Printing Press companion skills:
   `npx skills add mvanhorn/cli-printing-press/skills -g -y`
4. Cloned the public library:
   `/Users/zaydk/Desktop/printing-press-research/printing-press-library`
5. Downloaded the official Squarespace Commerce schema:
   `https://developers.squarespace.com/commerce-apis/latest/schema-processor-version-version-latest.json`
6. Generated the CLI:
   `printing-press generate --spec .../commerce-openapi.json --name squarespace --output /Users/zaydk/printing-press/library/squarespace --spec-source official`
7. Hardened generated output by hand:
   - Made `address_book.contacts_id` nullable to match real sync payloads that may not include parent contact IDs.
   - Added `site`, `website`, `store-pages`, and `pages` aliases over the generated `1-0` commands.
   - Added a hand-written `account` namespace for the authenticated dashboard API discovered from the Squarespace account bundle.
8. Ran Twitter/X research with `bird`:
   - `bird user-tweets mvanhorn --json`
   - `bird search '"printingpress.dev" OR "@ppressdev" OR "printing-press-library"' --json`
   - `bird search 'Squarespace API pages CLI domains developer API' --json`
9. Attempted Chrome CDP capture with `browser-harness-js` against the user's Google Chrome profile.

## Why It Matters

The official schema is enough for a useful Commerce CLI but not enough for the user's target: account domains, site list, and broad logged-in admin interactions. The final library-quality Squarespace CLI should merge two surfaces:

- Official Commerce API commands with stable bearer-token auth.
- Browser-sniffed account/admin commands using authenticated Squarespace web session mechanics, with guarded writes and explicit confirmation for destructive actions.

The generated CLI now has a stable baseline that can be extended once authenticated CDP or HAR capture is available.

## Dashboard Account Surface

The generated project now includes read-only commands for the account dashboard API that was absent from the official schema:

```text
squarespace-pp-cli account context
squarespace-pp-cli account sites
squarespace-pp-cli account briefs
squarespace-pp-cli account domains
squarespace-pp-cli account domain-summaries
squarespace-pp-cli account domain get --name example.com
squarespace-pp-cli account domain whois --name example.com
squarespace-pp-cli account domain registrar-info --name example.com
squarespace-pp-cli account domain certificates --name example.com
squarespace-pp-cli account domain is-unparked --name example.com
squarespace-pp-cli account domain custom-records --name example.com
squarespace-pp-cli account domain email-forwarding --name example.com
squarespace-pp-cli account domain email-mx-conflicts --name example.com
squarespace-pp-cli account domain billing-eligibility --name example.com
squarespace-pp-cli account domain billing-valid-terms --name example.com
squarespace-pp-cli account domain google-workspace-pricing --country-code US
squarespace-pp-cli account website get --id <website-id>
squarespace-pp-cli account website domains --website-id <website-id>
squarespace-pp-cli account website contributors --website-id <website-id>
```

Auth for these commands is intentionally runtime-only:

```bash
SQUARESPACE_ACCOUNT_COOKIE='name=value; ...' squarespace-pp-cli account context --json
SQUARESPACE_ACCOUNT_COOKIE_FILE=/path/to/cookie-header.txt squarespace-pp-cli account sites --json
```

The CLI redacts token/session/cookie/secret-shaped JSON fields before printing. Account commands require a currently authenticated browser session cookie.

After attaching to the user's logged-in main Chrome profile over CDP on port `55521`, the account dashboard live-smoke passed against a managed domain page.

Captured dashboard API calls included:

```text
GET /api/account/1/domain-summaries?sortDirection=ASCENDING&sortField=NAME&query=&page=0&pageSize=50
GET /api/account/1/website-summaries?page=1
GET /api/account/1/profile
GET /api/account/1/user/is-suspicious
GET /api/account/1/domains/byName/<domain-name>
GET /api/account/1/clone-websites/all-jobs/status?websiteIds=<website-id>
GET /api/account/1/websites/<website-id>/website-domains
GET /api/account/1/domains/<domain-name>/category
GET /api/account/1/website-summaries/<website-id>
GET /api/account/1/domains/<domain-id>/user-permissions
GET /api/account/1/domains/<domain-id>/is-unparked
GET /api/account/1/domains/<domain-id>/forwarding-presets
GET /api/account/1/domains/<domain-id>/registrar-info
GET /api/account/1/domains/<domain-id>/custom-record-set
GET /api/account/1/domains/<domain-id>/presets
GET /api/account/1/domains/<domain-id>/email-forwarding
GET /api/account/1/domains/<domain-id>/email-forwarding/has-conflicting-mx-records
GET /api/account/1/plans/available-plans/google-apps/applicable/starting-prices?countryCode=US
GET /api/account/1/billing/websites/<website-id>/contracts/<contract-id>/eligibility
GET /api/account/1/billing/websites/<website-id>/contracts/<contract-id>/validTerms?domainName=<domain-name>
GET /api/account/1/user/domains
GET /api/account/1/manifests/business-merchandising
GET /api/account/1/manifests/email-product-frontend
```

Important discovery: many domain-detail endpoints use an internal domain id rather than the public domain name, and billing endpoints use both `websiteId` and `subscriptionId` from the domain lookup payload. The CLI resolves `--name <domain-name>` through `/api/account/1/domains/byName/<domain-name>` before calling id-based or contract-based endpoints. No domain-specific ids are baked into the CLI.

Live CLI smoke results using a runtime-only Cookie header from the logged-in Chrome profile:

```text
squarespace-pp-cli account domain get --name <domain-name> --json --compact
PASS: returned object with id/name fields

squarespace-pp-cli account domain registrar-info --name <domain-name> --json --compact
PASS: returned pendingRegistrantChangeInfo, tldInfo, transferLock, whoisContacts

squarespace-pp-cli account domain is-unparked --name <domain-name> --json --compact
PASS: returned true

squarespace-pp-cli account domain custom-records --name <domain-name> --json --compact
PASS: returned DNS records

squarespace-pp-cli account domain email-forwarding --name <domain-name> --json --compact
PASS: returned customTxtRecordData, domainName, forwardingRules, status

squarespace-pp-cli account domain email-mx-conflicts --name <domain-name> --json --compact
PASS: returned MX conflict status

squarespace-pp-cli account domain billing-eligibility --name <domain-name> --json --compact
PASS: resolved website/contract ids and returned billing eligibility

squarespace-pp-cli account domain billing-valid-terms --name <domain-name> --json --compact
PASS: resolved website/contract ids and returned valid renewal terms

squarespace-pp-cli account domain google-workspace-pricing --country-code US --json --compact
PASS: returned Google Workspace applicable starting prices

squarespace-pp-cli account website domains --website-id <website-id> --json --compact
PASS: returned domainTransfers, domains, hasDomainsPermissions, registeringDomains

squarespace-pp-cli account domain-summaries --page-size 50 --json --compact
PASS: returned hasNextPage, summaries, totalCount
```

## Raw Evidence

Official schema summary:

```json
{
  "openapi": "3.1.1",
  "title": "Commerce API",
  "version": "2",
  "pathCount": 33,
  "tags": [
    "Contacts",
    "Analytics",
    "Products",
    "Websites",
    "Inventory",
    "Orders",
    "Transactions",
    "Profiles",
    "WebhookSubscriptions"
  ]
}
```

Printing Press validation:

```text
Dogfood Report: squarespace
Path Validity: 9/9 valid (PASS)
Auth Protocol: MATCH
Dead Flags: 0 dead (PASS)
Dead Functions: 0 dead (PASS)
Examples: 10/10 commands have examples (PASS)
MCP Surface: PASS
Verdict: PASS
```

Scorecard:

```text
Total: 84/100 - Grade A
Gap: insight scored 4/10 - needs improvement
```

Runtime verification:

```text
Mode: mock
Data Pipeline: PASS: sync completed
Pass Rate: 100% (17/17 passed, 0 critical)
Verdict: PASS
```

Local tests:

```text
go test ./... PASS
go build ./... PASS
```

Dry-run examples:

```text
squarespace-pp-cli site profile --dry-run --json
GET https://api.squarespace.com/1.0/authorization/website

squarespace-pp-cli pages --dry-run --json
GET https://api.squarespace.com/1.0/commerce/store_pages

squarespace-pp-cli account context --dry-run --json
GET https://account.squarespace.com/api/account/1/context/project-picker

squarespace-pp-cli account website domains --website-id demo --dry-run --json
GET https://account.squarespace.com/api/account/1/websites/demo/website-domains
```

Twitter/X Printing Press usage examples observed:

- Matt Van Horn launched Printing Press as a CLI factory and library for agent-native CLIs.
- Community examples mentioned Whoop, YouTube, Substack, Peloton, Table Reservation Goat, Jira, Ticketmaster, Fathom, OpenTable, Tock, Resy, arXiv, OpenAlex, Jimmy John's, ClickUp, Beehiiv, and SEC Edgar.
- Current community pattern is "print a CLI, then manually validate and clean up", which matches this Squarespace flow.

CDP blocker evidence:

```text
browser-harness-js found Google Chrome at:
ws://127.0.0.1:55521/devtools/browser/...

Connection attempts timed out until Chrome's remote-debugging permission prompt is accepted.
Raw /json/version on the detected port returned HTTP 404, so the DevTools HTTP surface is also gated.
```

No API keys, cookies, session tokens, request headers, or HAR payloads were written to this file.

## Reproducibility

From `/Users/zaydk/printing-press/library/squarespace`:

```bash
go test ./...
go build ./...
printing-press dogfood --dir /Users/zaydk/printing-press/library/squarespace --spec /Users/zaydk/Desktop/printing-press-research/squarespace/schema/commerce-openapi.json
printing-press scorecard --dir /Users/zaydk/printing-press/library/squarespace --spec /Users/zaydk/Desktop/printing-press-research/squarespace/schema/commerce-openapi.json
printing-press verify --dir /Users/zaydk/printing-press/library/squarespace --spec /Users/zaydk/Desktop/printing-press-research/squarespace/schema/commerce-openapi.json --cleanup
```

For authenticated admin/domains discovery, accept Chrome's remote-debugging prompt when `browser-harness-js` connects, then capture:

```bash
browser-harness-js 'await session.connect({profileDir:"/Users/zaydk/Library/Application Support/Google/Chrome", timeoutMs:60000}); return await listPageTargets();'
```

Navigate to `https://account.squarespace.com/domains`, interact with domains/sites/pages, and export a redacted request manifest or HAR before running:

```bash
printing-press browser-sniff --har <redacted-capture.har> --name squarespace-admin --output <admin-spec.yaml>
```
