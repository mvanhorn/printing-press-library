# Costco Sameday CLI

Costco Same-Day (sameday.costco.com) GraphQL CLI. Cookie auth after Costco Azure B2C SSO. Place-order mutation is FinalizeCheckout — exposed only via gated `order place` (--yes + --confirm-charge; --dry-run never charges). Tip updates via UpdateCheckout tipsUpdate. Cancel mutation not captured.

Learn more at [Costco Sameday](https://sameday.costco.com).

Created by [@DashLabsDev](https://github.com/DashLabsDev) (Thomas McCormick).

## Install

The recommended path installs both the `costco-sameday-pp-cli` binary and the `pp-costco-sameday` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install costco-sameday
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install costco-sameday --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install costco-sameday --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install costco-sameday --agent claude-code
npx -y @mvanhorn/printing-press-library install costco-sameday --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.6 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/commerce/costco-sameday/cmd/costco-sameday-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/costco-sameday-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install costco-sameday --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-costco-sameday --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-costco-sameday --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install costco-sameday --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

The bundle reuses your local browser session — set it up first if you haven't:

```bash
costco-sameday-pp-cli auth login --chrome
```

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/costco-sameday-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/commerce/costco-sameday/cmd/costco-sameday-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "costco-sameday": {
      "command": "costco-sameday-pp-mcp"
    }
  }
}
```

</details>

## Quick Start

### 1. Install

See [Install](#install) above.

### 2. Authenticate

This CLI uses your browser session for authentication. Log in to sameday.costco.com in Chrome, then:

```bash
costco-sameday-pp-cli auth login --chrome
```

Or import an existing browser capture:

```bash
costco-sameday-pp-cli auth login --cookies-file storage-state.json
```

`--cookies-file` accepts Playwright storage-state JSON or a raw `Cookie:` header text file. Use `--cookies-file -` to read the header from stdin (for example `pbpaste | costco-sameday-pp-cli auth login --cookies-file -`). Stdin also accepts DevTools "Copy request headers" or "Copy as cURL (bash)" output from a sameday.costco.com request. Only the `__Host-instacart_sid` session cookie is required; it is HttpOnly, so copy it from DevTools Network rather than `document.cookie`. Other Same-Day cookies are kept when present. Add `--diagnose` to check a paste without saving anything: it prints only the byte count, detected format, parse stage, cookie names, and whether `__Host-instacart_sid` is present, never values. The Chrome path requires a cookie extraction tool. Install one:

```bash
pip install pycookiecheat          # Python (recommended)
brew install barnardb/cookies/cookies  # Homebrew
```

When your session expires, run `auth login --chrome` again.

### 3. Verify Setup

```bash
costco-sameday-pp-cli doctor
```

This checks your configuration and credentials.

### 4. Try Your First Command

```bash
costco-sameday-pp-cli account gethouseholdbyuser --operation-name GetHouseholdByUser
```

## Usage

Run `costco-sameday-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data: `credentials.toml`, `data.db`, cookies, browser-session proof files, and other auth sidecars |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `COSTCO_SAMEDAY_CONFIG_DIR`, `COSTCO_SAMEDAY_DATA_DIR`, `COSTCO_SAMEDAY_STATE_DIR`, or `COSTCO_SAMEDAY_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `COSTCO_SAMEDAY_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export COSTCO_SAMEDAY_HOME=/srv/costco-sameday
costco-sameday-pp-cli doctor
```

Under `COSTCO_SAMEDAY_HOME=/srv/costco-sameday`, the four dirs resolve to `/srv/costco-sameday/config`, `/srv/costco-sameday/data`, `/srv/costco-sameday/state`, and `/srv/costco-sameday/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "costco-sameday": {
      "command": "costco-sameday-pp-mcp",
      "env": {
        "COSTCO_SAMEDAY_HOME": "/srv/costco-sameday"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `COSTCO_SAMEDAY_DATA_DIR` overrides an explicit `--home` for that kind. Use `COSTCO_SAMEDAY_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `COSTCO_SAMEDAY_HOME` does not move files back to platform defaults, and `doctor` cannot find credentials left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. On the first auth write, stored secrets leave `config.toml` and are consolidated into `credentials.toml` under the data directory. Run `costco-sameday-pp-cli doctor --fail-on warn` to check path and credential-location warnings in automation.

## Session context and ZIP

Cart, slots, retailer, product and search commands fill `shopId`, `cartId`,
`zoneId`, `postalCode` and `userLocation` automatically when you omit the
matching flag. Explicit flags (`--shop-id`, `--cart-id`, ...) always win.

- The Costco shop and zone are looked up from your delivery ZIP. ZIP
  precedence: a `postalCode` or `userLocation.postalCode` on this command,
  then `--zip`, then `COSTCO_SAMEDAY_ZIP`, then the last explicit `--zip`
  (remembered in the state dir), then the account/IP geolocation. The
  account/IP zone is used only when its ZIP matches; a different ZIP uses
  RetailersZone. A postal code on the command selects the shop, zone and cart
  for that call and is not remembered as the default. An explicit `--shop-id`
  resolves the cart without a ZIP.
- The active cart comes from your signed-in session (`ActiveCartId`, falling
  back to `PersonalActiveCarts`) and is cached for 10 minutes; shop and zone
  are cached for 24 hours. Use `session context --refresh` to re-resolve.
- A GraphQL response with `errors` and no data exits non-zero and names the
  missing variable (for example `missing cartId`) with the flag to pass.

```bash
costco-sameday-pp-cli session context --zip 98027
costco-sameday-pp-cli slots availableservices
costco-sameday-pp-cli search kirkland --data-source live
```

## Commands

### account

Costco Same-Day account GraphQL operations

- **`costco-sameday-pp-cli account gethouseholdbyuser`** - GraphQL query GetHouseholdByUser (persistedQuery)
- **`costco-sameday-pp-cli account householdbyuser`** - GraphQL query HouseholdByUser (persistedQuery)

### cart

Costco Same-Day cart GraphQL operations

- **`costco-sameday-pp-cli cart activecartid`** - GraphQL query ActiveCartId (persistedQuery)
- **`costco-sameday-pp-cli cart cartbottombannerquery`** - GraphQL query CartBottomBannerQuery (persistedQuery)
- **`costco-sameday-pp-cli cart cartcheckoutvalidation`** - GraphQL query CartCheckoutValidation (persistedQuery)
- **`costco-sameday-pp-cli cart cartcouponremovalpopupquery`** - GraphQL query CartCouponRemovalPopupQuery (persistedQuery)
- **`costco-sameday-pp-cli cart cartonloadplacementquery`** - GraphQL query CartOnloadPlacementQuery (persistedQuery)
- **`costco-sameday-pp-cli cart cartproductsrecomendation`** - GraphQL query CartProductsRecomendation (persistedQuery)
- **`costco-sameday-pp-cli cart cartrecommendationsplacements`** - GraphQL query CartRecommendationsPlacements (persistedQuery)
- **`costco-sameday-pp-cli cart cartsignaledeta`** - GraphQL query CartSignaledEta (persistedQuery)
- **`costco-sameday-pp-cli cart cartswitchervariant`** - GraphQL query CartSwitcherVariant (persistedQuery)
- **`costco-sameday-pp-cli cart carttotals`** - GraphQL query CartTotals (persistedQuery)
- **`costco-sameday-pp-cli cart cartviewlayout`** - GraphQL query CartViewLayout (persistedQuery)
- **`costco-sameday-pp-cli cart familyexpeditedonboardingcart`** - GraphQL query FamilyExpeditedOnboardingCart (persistedQuery)
- **`costco-sameday-pp-cli cart finishmycartviewlayout`** - GraphQL query FinishMyCartViewLayout (persistedQuery)
- **`costco-sameday-pp-cli cart finishmycartviewuilayout`** - GraphQL query FinishMyCartViewUILayout (persistedQuery)
- **`costco-sameday-pp-cli cart fixcartbasketforcheckout`** - GraphQL query FixCartBasketForCheckout (persistedQuery)
- **`costco-sameday-pp-cli cart floatingcartmessages`** - GraphQL query FloatingCartMessages (persistedQuery)
- **`costco-sameday-pp-cli cart getexpresscartplacementsquery`** - GraphQL query GetExpressCartPlacementsQuery (persistedQuery)
- **`costco-sameday-pp-cli cart inlinecarteppvariant`** - GraphQL query InlineCartEppVariant (persistedQuery)
- **`costco-sameday-pp-cli cart othersingleretailercarts`** - GraphQL query OtherSingleRetailerCarts (persistedQuery)
- **`costco-sameday-pp-cli cart personalactivecarts`** - GraphQL query PersonalActiveCarts (persistedQuery)
- **`costco-sameday-pp-cli cart treatmentcartmessages`** - GraphQL query TreatmentCartMessages (persistedQuery)
- **`costco-sameday-pp-cli cart updatecartitemsmutation`** - GraphQL mutation UpdateCartItemsMutation (persistedQuery)
- **`costco-sameday-pp-cli cart usercart`** - GraphQL query UserCart (persistedQuery)
- **`costco-sameday-pp-cli cart usercartenriched`** - GraphQL query UserCartEnriched (persistedQuery)

### checkout

Costco Same-Day checkout GraphQL operations

- **`costco-sameday-pp-cli checkout checkoutaisleitemmin`** - GraphQL query CheckoutAisleItemMin (persistedQuery)
- **`costco-sameday-pp-cli checkout checkoutcmd`** - GraphQL query CheckoutCmd (persistedQuery)
- **`costco-sameday-pp-cli checkout checkoutdraftorderinvoicev2`** - GraphQL query CheckoutDraftOrderInvoiceV2 (persistedQuery)
- **`costco-sameday-pp-cli checkout checkoutheaderview`** - GraphQL query CheckoutHeaderView (persistedQuery)
- **`costco-sameday-pp-cli checkout checkoutpagemeta`** - GraphQL query CheckoutPageMeta (persistedQuery)
- **`costco-sameday-pp-cli checkout checkoutv4disclaimeronetrusttoggle`** - GraphQL query CheckoutV4DisclaimerOneTrustToggle (persistedQuery)
- **`costco-sameday-pp-cli checkout getexpresscheckouttoggleplacements`** - GraphQL query GetExpressCheckoutTogglePlacements (persistedQuery)
- **`costco-sameday-pp-cli checkout giftingexpandedcheckoutdetails`** - GraphQL query GiftingExpandedCheckoutDetails (persistedQuery)
- **`costco-sameday-pp-cli checkout initializecheckout`** - GraphQL query InitializeCheckout (persistedQuery)
- **`costco-sameday-pp-cli checkout updatebuyflowpaymentinstructionsv2`** - GraphQL mutation UpdateBuyflowPaymentInstructionsV2 — attach/select payment instrument on checkout session (not FinalizeCheckout).
- **`costco-sameday-pp-cli checkout updatecheckout`** - GraphQL mutation UpdateCheckout (persistedQuery). Tip changes use checkoutUpdates.tipsUpdate.tippingFields.tipInputToken. Not the charge mutation.

### orders

Costco Same-Day orders GraphQL operations

- **`costco-sameday-pp-cli orders customercancelselections`** - GraphQL query CustomerCancelSelections — cancel-reason options UI. Cancel MUTATION not in place-order HAR (cancel may have happened after capture).
- **`costco-sameday-pp-cli orders orderuptimer`** - GraphQL query OrderUpTimer (persistedQuery)

### products

Costco Same-Day products GraphQL operations

- **`costco-sameday-pp-cli products autosuggestions`** - GraphQL query Autosuggestions (persistedQuery)
- **`costco-sameday-pp-cli products itemcardsizesearchvariant`** - GraphQL query ItemCardSizeSearchVariant (persistedQuery)
- **`costco-sameday-pp-cli products itemdetaildata`** - GraphQL query ItemDetailData (persistedQuery)
- **`costco-sameday-pp-cli products itemdetailfeaturedproductlist`** - GraphQL query ItemDetailFeaturedProductList (persistedQuery)
- **`costco-sameday-pp-cli products itemdetailsrecommendationsplacements`** - GraphQL query ItemDetailsRecommendationsPlacements (persistedQuery)
- **`costco-sameday-pp-cli products itemdetailsretailerproduct`** - GraphQL query ItemDetailsRetailerProduct (persistedQuery)
- **`costco-sameday-pp-cli products itemdetailsupplementalfields`** - GraphQL query ItemDetailSupplementalFields (persistedQuery)
- **`costco-sameday-pp-cli products itemdetailsv2flags`** - GraphQL query ItemDetailsV2Flags (persistedQuery)
- **`costco-sameday-pp-cli products itemdetailsv4`** - GraphQL query ItemDetailsV4 (persistedQuery)
- **`costco-sameday-pp-cli products itemdetailviewlayout`** - GraphQL query ItemDetailViewLayout (persistedQuery)
- **`costco-sameday-pp-cli products items`** - GraphQL query Items (persistedQuery)
- **`costco-sameday-pp-cli products klarnaosmitemdetails`** - GraphQL query KlarnaOSMItemDetails (persistedQuery)
- **`costco-sameday-pp-cli products postaddtocartsearchlayoutquery`** - GraphQL query PostAddToCartSearchLayoutQuery (persistedQuery)
- **`costco-sameday-pp-cli products searchfacets`** - GraphQL query SearchFacets (persistedQuery)
- **`costco-sameday-pp-cli products searchresultsplacements`** - GraphQL query SearchResultsPlacements (persistedQuery)
- **`costco-sameday-pp-cli products viewlayoutsearchresults`** - GraphQL query ViewLayoutSearchResults (persistedQuery)

### retailer

Costco Same-Day retailer GraphQL operations

- **`costco-sameday-pp-cli retailer currentretailer`** - GraphQL query CurrentRetailer (persistedQuery)
- **`costco-sameday-pp-cli retailer landingretailermetas`** - GraphQL query LandingRetailerMetas (persistedQuery)

### session

Costco Same-Day session GraphQL operations

- **`costco-sameday-pp-cli session context`** - Resolve the delivery ZIP, Costco shopId, zoneId and active cartId used to auto-fill other commands
- **`costco-sameday-pp-cli session complementaryproductitems`** - GraphQL query ComplementaryProductItems (persistedQuery)
- **`costco-sameday-pp-cli session getcobrandcreditcardoffermutation`** - GraphQL mutation GetCobrandCreditCardOfferMutation (persistedQuery)

### slots

Costco Same-Day slots GraphQL operations

- **`costco-sameday-pp-cli slots availableservices`** - GraphQL query AvailableServices (persistedQuery)
- **`costco-sameday-pp-cli slots slotcampaignplacement`** - GraphQL query SlotCampaignPlacement (persistedQuery)


### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`costco-sameday-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`costco-sameday-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`costco-sameday-pp-cli learnings list`** - Inspect taught rows
- **`costco-sameday-pp-cli learnings forget <query>`** - Undo a teach
- **`costco-sameday-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`costco-sameday-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`costco-sameday-pp-cli teach-pattern`** - Install a query/resource template up front
- **`costco-sameday-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `COSTCO_SAMEDAY_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `costco-sameday-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
costco-sameday-pp-cli account gethouseholdbyuser --operation-name GetHouseholdByUser

# JSON for scripting and agents
costco-sameday-pp-cli account gethouseholdbyuser --operation-name GetHouseholdByUser --json
# Filter to specific fields by name
costco-sameday-pp-cli account gethouseholdbyuser --operation-name GetHouseholdByUser --json --select <field>[,<field>...]

# Dry run — show the request without sending
costco-sameday-pp-cli account gethouseholdbyuser --operation-name GetHouseholdByUser --dry-run

# Agent mode — JSON + compact + no prompts in one flag
costco-sameday-pp-cli account gethouseholdbyuser --operation-name GetHouseholdByUser --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select <field>[,<field>...]` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Explicit retries** - add `--idempotent` to create retries when a no-op success is acceptable
- **Explicit confirmation** - `--agent` does not imply `--yes`; pass `--yes` separately only after the target, arguments, and side effects are clear
- **Piped input** - write commands can accept structured input when their help lists `--stdin`
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
costco-sameday-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Run `costco-sameday-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/costco-sameday-pp-cli/config.toml`; `--home`, `COSTCO_SAMEDAY_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `costco-sameday-pp-cli doctor` to check credentials
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

## HTTP Transport

This CLI uses standard HTTP transport with HTTP/2 disabled for browser-facing endpoints. It does not require a resident browser process for normal API calls.

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
