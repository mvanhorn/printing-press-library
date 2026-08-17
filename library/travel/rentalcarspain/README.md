# Rental Car Spain CLI

**Find your best rental car, fully insured, at any airport in Spain.** Compares zero-excess prices across DoYouSpain, Rentalcars and seven direct suppliers — mainland, Balearics and Canaries — for any car from a Fiat 500 to a BMW or Mercedes, ranked by real per-airport customer reviews, with price history and drop alerts.

Rental Car Spain searches DoYouSpain (the aggregator that surfaces the cheapest rental companies in an area) and quotes every price with the Full Insurance / Zero Excess tier by default. It cross-checks Delpaso's own site with 'compare', tracks prices over time with 'drift', and alerts on drops with 'watch' — the AutoSlash workflow, for Spain, that no other tool offers.

## Data sources & acceptable use — read this first

This tool is for **personal, non-commercial, educational use**: an aid to comparing prices you intend to book yourself. It is **not affiliated with** any of the rental companies or aggregators it queries; all trademarks belong to their owners.

To read prices, it contacts each supplier's own website and internal endpoints (Delpaso, Centauro, Drivalia, Clickrent, Goldcar, CICAR, Autoreisen, DoYouSpain, Rentalcars). **Those suppliers generally restrict or prohibit automated access** — in their terms of service and in `robots.txt`. Checked directly, the relevant `robots.txt` disallow the very paths this tool uses:

| Supplier | `robots.txt` disallows (paths this tool uses) |
| --- | --- |
| DoYouSpain | `/car-hire/*`, `/quote.asp*`, `/ajax_*`, `/do/*` |
| Rentalcars | `/search*`, `/SearchResults*`, `/json/*` |
| Centauro | `/ajax/*`, `/*?*` (any query-string URL) |
| Others | bot-protected sites; API paths are not published/public |

So **running this tool may breach those suppliers' terms.** That is a decision only you can make, and **you are solely responsible** for your own use and for compliance with any applicable terms and law. Where a project grows beyond personal use, the correct path is an **official affiliate/partner API** (e.g. Rentalcars/Booking.com offers one), not scraping.

How this project tries to be a good citizen — and where its limits are:

- **Honest identification.** Every request sends `User-Agent: rentalcarspain-pp-cli/…`. The tool does **not** impersonate a browser to slip past bot detection. If a supplier declines the honest client, that is a "no" — disable that supplier with `--disable-supplier` rather than trying to circumvent it.
- **Low impact.** Read-only (no bookings, accounts, or writes), low volume, and per-host rate-limited by default (`--rate-limit`).
- **Your control.** Skip any supplier you're not comfortable contacting: `--disable-supplier doyouspain,rentalcars,goldcar` (comma-separated, any command).
- **Not a booking.** Prices are best-effort and may be inaccurate or stale. **Always verify on the supplier's own site before booking.** Provided "AS IS", no warranty, no liability. See `NOTICE`.

## Install

The recommended path installs both the `rentalcarspain-pp-cli` binary and the `pp-rentalcarspain` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install rentalcarspain
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install rentalcarspain --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install rentalcarspain --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install rentalcarspain --agent claude-code
npx -y @mvanhorn/printing-press-library install rentalcarspain --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.6 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/travel/rentalcarspain/cmd/rentalcarspain-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/rentalcarspain-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install rentalcarspain --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-rentalcarspain --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-rentalcarspain --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install rentalcarspain --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/rentalcarspain-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/travel/rentalcarspain/cmd/rentalcarspain-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "rentalcarspain": {
      "command": "rentalcarspain-pp-mcp"
    }
  }
}
```

</details>

## Authentication

No account or API key needed — the suppliers' sites are public. Rental Car Spain sends an **honest, non-browser** User-Agent (`rentalcarspain-pp-cli/…`) that identifies the tool rather than impersonating a browser; 'doctor' verifies connectivity. See "Data sources & acceptable use" above for the terms-of-service considerations before you run it.

## Quick Start

```bash
# Confirm the sources are reachable (sends the honest tool User-Agent).
rentalcarspain-pp-cli doctor --dry-run

# Resolve a place name to a DoYouSpain code (Malaga Airport = MAL02).
rentalcarspain-pp-cli locations Malaga

# Search Malaga Airport for a week in August, cheapest full-insurance offers first.
rentalcarspain-pp-cli search 20/08/2026 27/08/2026 --sort cheapest

# See the cheapest offer per supplier, including Delpaso, Record Go and Wiber.
rentalcarspain-pp-cli suppliers 20/08/2026 27/08/2026

# Cross-check the aggregator's Delpaso price against Delpaso's own site.
rentalcarspain-pp-cli compare 20/08/2026 27/08/2026

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Per-airport recommendation with reviews
- **`recommend`** — The review-aware recommendation for an airport. Attaches customer ratings (Rentalcars primary, DoYouSpain fallback) and shows three blocks: (1) the **top companies at this airport** rated ≥ `--min-rating` and ranked by trust, each priced best-available (our direct client where we have one, else the company's cheapest fully-insured price from the aggregators); (2) **recommended cars** — a small and a bigger fully-insured (zero-excess) pick, then the cheapest base-rate cars as base + standalone cover; (3) a **full trust table** of every company at the airport with caveats.

  _Reach for this when the airport isn't Málaga and you want the most trusted company, not just the cheapest._

  ```bash
  rentalcarspain-pp-cli recommend BIO 21/07/2026 28/07/2026
  rentalcarspain-pp-cli recommend ALC 20/07/2026 27/07/2026 --min-rating 8 --agent
  ```

### Cheapest fully-insured, across every source
- **`best`** — Rank the cheapest zero-excess (fully-insured) options across all sources: direct companies (Delpaso, Centauro, Drivalia, Clickrent, Goldcar, plus CICAR and Autoreisen in the Canaries) at their own no-excess price, plus aggregator offers (DoYouSpain, Rentalcars) priced as base rate + a standalone third-party excess-insurance estimate — the AutoSlash-style money-saver that's often cheaper than the rental firm's own full cover.

  _Reach for this to answer "what's the cheapest way to rent fully insured?" in one call._

  The standalone-cover estimate is **modeled on real iCarHire single-trip quotes** — a fixed policy base **plus** a per-day rate (not a flat per-day, which under-quotes short rentals). Rows that lean on it are tagged `~est`; tune it with `--excess-cover-base` / `--excess-cover-per-day`, or use **`--real-only`** to rank on fully-quoted zero-excess prices alone, and `--direct-only` to compare the companies' own full cover.

  ```bash
  rentalcarspain-pp-cli best 20/07/2026 03/08/2026 --limit 8 --agent
  rentalcarspain-pp-cli best AGP 15/09/2026 22/09/2026 --real-only   # only fully-quoted prices
  ```
- **`direct`** — Full-insurance (zero-excess) quotes straight from the companies' own booking sites — the true all-inclusive price, not an aggregator teaser. Queries seven direct clients: Delpaso (Málaga), Centauro, Drivalia, Clickrent and Goldcar (mainland + islands), plus CICAR and Autoreisen in the Canary Islands. Supports Málaga and other Spanish airports (IATA code or name); companies that don't serve the chosen airport are marked unavailable.

  _Reach for this when you always want full coverage and want the real price you'll book on._

  ```bash
  rentalcarspain-pp-cli direct 20/07/2026 03/08/2026 --options 4
  ```

### Driver age handled honestly
Pass **`--driver-age`** and the tool reflects each supplier's *real* young/senior policy — verified against their live booking flows, not guessed:

- **Priced-in surcharges.** Where a supplier charges an obligatory young/senior fee in its online total, the quote includes it: **Centauro** (Conductor joven +€13/day under 25; Conductor senior +€7/day at 74+), **Drivalia** (young-driver fee, live from its offer API). **Delpaso** doesn't price age online, so its published "Conductor joven" rule (€12/day, min €36, max €100) is computed in; **Clickrent** and **Goldcar** add no online surcharge, so their prices are unchanged.
- **Eligibility flags.** Cars a young driver can't actually rent are shown but tagged, never silently quoted: `Goldcar Kia Picanto [min age 21]`, `Clickrent Audi A1 [min age 21]` — Clickrent's real per-car minimums (18/21/30) and Goldcar's under-21 decline.
- **A clear caveat** under the results explains, per supplier, whether the age surcharge is in the quote, counter-collected, or an eligibility limit.

```bash
rentalcarspain-pp-cli direct AGP 15/09/2026 22/09/2026 --driver-age 20 --options 4
rentalcarspain-pp-cli best AGP 15/09/2026 22/09/2026 --driver-age 72
```

### Skip a supplier you'd rather not contact
**`--disable-supplier`** (any command) omits named suppliers from every layer — direct clients *and* aggregators — so anyone uneasy about a supplier's terms can leave it out. See "Data sources & acceptable use" above.

```bash
rentalcarspain-pp-cli best AGP 15/09/2026 22/09/2026 --disable-supplier goldcar,doyouspain
rentalcarspain-pp-cli direct AGP 15/09/2026 22/09/2026 --disable-supplier rentalcars
```

### Multi-source & multi-airport search
- **`search --source all`** merges DoYouSpain and Rentalcars; every offer shows its excess ('none' = fully insured, a number = your deductible, '?' = not stated). Location accepts an IATA code (AGP, ALC, BCN…), a DoYouSpain code, or an airport name (accent-optional — `malaga`, `almeria`, `coruna` all work); run `locations` to list supported airports.

  **Full Spain coverage: all 32 commercial airports** — every AENA field with scheduled passenger service. Mainland (AGP, ALC, BCN, MAD, VLC, SVQ, BIO, GRO, RMU, REU, LEI, GRX, XRY, ZAZ, SCQ, OVD, VGO, LCG, SDR, EAS, VLL, PNA), Balearics (PMI, IBZ, MAH), Canaries (TFS, TFN, LPA, ACE, FUE, SPC) and Melilla (MLN).

  Two of the smallest — **Valladolid (VLL)** and **Melilla (MLN)** — currently return no offers: neither aggregator nor any direct company sells rentals there. That's a genuine "no supply" answer, not a failure; they stay listed so the codes resolve and start working if supply appears.

  ```bash
  rentalcarspain-pp-cli search ALC 20/07/2026 27/07/2026 --source all
  ```

### Indicative prices in your currency
- **`--currency USD|GBP|…`** adds an indicative conversion to any table: `≈ 187.95 USD (164.80 EUR)`. Spanish rentals are **billed in EUR** — the conversion is for planning only (your card does its own FX at booking). Rates come from the European Central Bank's free daily feed and are cached locally (see `cache`). JSON/`--agent` output stays EUR-canonical.

  ```bash
  rentalcarspain-pp-cli best AGP 15/08/2026 22/08/2026 --currency USD
  rentalcarspain-pp-cli suppliers BIO 15/08/2026 22/08/2026 --currency GBP
  ```

### The renter's ritual
- **`compare`** — Compare DoYouSpain's cheapest offer for a supplier against that supplier's own booking site, side by side.

  _Reach for this to confirm the aggregator price is real before booking, the way a careful renter double-checks the company directly._

  ```bash
  rentalcarspain-pp-cli compare 20/08/2026 27/08/2026 --agent
  ```
- **`suppliers`** — One line per supplier — Delpaso, Record Go, Wiber, Sixt and the rest — each with its cheapest full-insurance offer, ranked.

  _Reach for this to see which of your usual companies is cheapest today without scrolling every offer._

  ```bash
  rentalcarspain-pp-cli suppliers 20/08/2026 27/08/2026 --agent
  ```

### Local price memory
- **`drift`** — Show how a saved Malaga search's cheapest full-insurance price has moved across your local snapshots.

  _Reach for this to decide whether to book now or wait, based on the actual trend not a guess._

  ```bash
  rentalcarspain-pp-cli drift agp-august --agent
  ```
- **`watch`** — Re-quote a saved search and exit 0 when the cheapest full-insurance price is at or below your target, 10 otherwise.

  _Reach for this in a cron job to get alerted the moment a tracked Malaga rental drops to your price._

  ```bash
  rentalcarspain-pp-cli watch agp-august --target-price 250 --agent
  ```
- **`dates`** — Fan out searches across a date window for a fixed rental length and rank pickup dates by cheapest total.

  _Reach for this when your dates are flexible and you want the cheapest day to start the rental._

  ```bash
  rentalcarspain-pp-cli dates --from 15/08/2026 --to 25/08/2026 --nights 7 --agent
  ```
- **`saved`** — Name and store a recurring Malaga search so watch and drift can re-run it.

  _Reach for this to set up the searches that feed watch and drift._

  ```bash
  rentalcarspain-pp-cli saved add agp-august MAL02 20/08/2026 27/08/2026
  ```

## Recipes

### Cheapest week in August at Malaga Airport

```bash
rentalcarspain-pp-cli dates --from 10/08/2026 --to 24/08/2026 --nights 7 --sort cheapest --agent
```

Sweeps pickup dates across two weeks and ranks them by cheapest full-insurance total.

### Only my usual companies

```bash
rentalcarspain-pp-cli search 20/08/2026 27/08/2026 --supplier delpaso,recordgo,wiber --agent
```

Filters the aggregator down to the three suppliers you book with.

### Track a rental and alert on a drop

```bash
rentalcarspain-pp-cli watch agp-august --target-price 250 --agent
```

Re-quotes a saved search; exits 0 when at or below 250 EUR so cron can notify you.

### Narrow the JSON for an agent

```bash
rentalcarspain-pp-cli search 20/08/2026 27/08/2026 --agent --select offers.supplier,offers.car,offers.total,offers.full_insurance_total
```

Returns only the fields an agent needs from a large result set.

### Confirm the aggregator price directly

```bash
rentalcarspain-pp-cli compare 20/08/2026 27/08/2026 --agent
```

Puts DoYouSpain's cheapest Delpaso offer next to Delpaso's own quote with the delta.

### Quote a young driver honestly

```bash
rentalcarspain-pp-cli direct AGP 15/09/2026 22/09/2026 --driver-age 20 --options 4
```

Adds each supplier's real young-driver surcharge (Centauro/Drivalia/Delpaso) and flags cars a 20-year-old can't rent (`[min age 21]`), with a per-supplier caveat under the table.

### Trust only fully-quoted prices

```bash
rentalcarspain-pp-cli best AGP 15/09/2026 22/09/2026 --real-only
```

Ranks on genuine zero-excess totals only, hiding rows whose total includes the standalone-cover estimate (`~est`).

### Leave out a supplier

```bash
rentalcarspain-pp-cli best AGP 15/09/2026 22/09/2026 --disable-supplier goldcar,doyouspain
```

Skips those suppliers entirely across direct and aggregator layers.

### Filter by brand or car type

```bash
rentalcarspain-pp-cli best AGP 15/09/2026 22/09/2026 --class bmw
rentalcarspain-pp-cli direct AGP 15/09/2026 22/09/2026 --class cabrio --options 3
rentalcarspain-pp-cli best AGP 15/09/2026 22/09/2026 --class "suv,estate" --real-only
```

`--class` (on `best`, `direct`, `search`, `suppliers`) matches **brands** (`bmw`, `mercedes`, `"alfa romeo"`), **body types** (`cabrio`, `suv`, `estate`, `van`, `coupe`, `pickup`), and **sizes** (`mini`, `economy`, `compact`, `premium`, `luxury`) — comma-separated terms are OR. It matches the model name and decodes ACRISS codes, so a car listed only as `IFAR` still matches `suv`. On `best`/`direct` the results stay zero-excess; combine with `--real-only` for fully-quoted prices only.

## Usage

Run `rentalcarspain-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data such as `data.db` |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `RENTALCARSPAIN_CONFIG_DIR`, `RENTALCARSPAIN_DATA_DIR`, `RENTALCARSPAIN_STATE_DIR`, or `RENTALCARSPAIN_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `RENTALCARSPAIN_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export RENTALCARSPAIN_HOME=/srv/rentalcarspain
rentalcarspain-pp-cli doctor
```

Under `RENTALCARSPAIN_HOME=/srv/rentalcarspain`, the four dirs resolve to `/srv/rentalcarspain/config`, `/srv/rentalcarspain/data`, `/srv/rentalcarspain/state`, and `/srv/rentalcarspain/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "rentalcarspain": {
      "command": "rentalcarspain-pp-mcp",
      "env": {
        "RENTALCARSPAIN_HOME": "/srv/rentalcarspain"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `RENTALCARSPAIN_DATA_DIR` overrides an explicit `--home` for that kind. Use `RENTALCARSPAIN_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `RENTALCARSPAIN_HOME` does not move files back to platform defaults, and `doctor` cannot find files left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. Run `rentalcarspain-pp-cli doctor --fail-on warn` to check path warnings in automation.

## Commands

### offers

Raw DoYouSpain offer listing for a location and date range (low-level; prefer the top-level 'search' command)

- **`rentalcarspain-pp-cli offers`** - Fetch the raw DoYouSpain results HTML for an already-resolved search session


### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`rentalcarspain-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`rentalcarspain-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`rentalcarspain-pp-cli learnings list`** - Inspect taught rows
- **`rentalcarspain-pp-cli learnings forget <query>`** - Undo a teach
- **`rentalcarspain-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`rentalcarspain-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`rentalcarspain-pp-cli teach-pattern`** - Install a query/resource template up front
- **`rentalcarspain-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `RENTALCARSPAIN_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `rentalcarspain-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
rentalcarspain-pp-cli offers --s example-value --b example-value

# JSON for scripting and agents
rentalcarspain-pp-cli offers --s example-value --b example-value --json

# Filter to specific fields
rentalcarspain-pp-cli offers --s example-value --b example-value --json --select id,name,status

# Dry run — show the request without sending
rentalcarspain-pp-cli offers --s example-value --b example-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
rentalcarspain-pp-cli offers --s example-value --b example-value --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Read-only by default** - this CLI does not create, update, delete, publish, send, or mutate remote resources
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
rentalcarspain-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Run `rentalcarspain-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/rentalcarspain/config.toml`; `--home`, `RENTALCARSPAIN_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **search returns no offers** — Run 'rentalcarspain-pp-cli doctor'; if DoYouSpain returns HTTP 406 the User-Agent was spoofed as a browser — Rental Car Spain must send a plain UA.
- **dates use the wrong format** — DoYouSpain dates are dd/mm/yyyy, e.g. 20/08/2026, not ISO.
- **Record Go or Wiber missing from results** — They appear as suppliers within 'search'/'suppliers'; there is no standalone recordgo/wiber command (Record Go geo-blocks and Wiber requires a browser).

## Known Gaps
- **Record Go and Wiber have no standalone live client.** Record Go geo-blocks non-EU traffic and Wiber sits behind a browser JS challenge, so neither can be quoted directly from this tool. Both appear as suppliers within `search` and `suppliers` via the DoYouSpain aggregator; use `--supplier recordgo` / `--supplier wiber` to isolate them.
- **DoYouSpain per-offer deposit/excess and the explicit full-insurance upsell price are not separately isolated** in the HTML parser yet; the quoted price is the offer's default (which DoYouSpain presents with Full Insurance). A future parser refinement can split the base and full-insurance tiers per offer.
