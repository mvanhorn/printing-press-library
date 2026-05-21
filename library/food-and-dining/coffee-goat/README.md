# Coffee GOAT CLI

**The third-wave coffee terminal — every elite roaster, every YouTube creator review, your brews, and the God cup.**

coffee-goat aggregates the global specialty-coffee shelf via Shopify-backed roaster storefronts, Coffee Review scores, and James Hoffmann + Lance Hedrick YouTube transcripts (via the youtube-pp-cli sibling) into one no-auth local SQLite corpus — then joins it with your personal brew log via `brews log`, your local cellar via `beans add`, SCA cupping sessions via `cupping`, and a curated V60 recipe library via `recipe-replay`.

The headline is the `god-cup` recommender, which integrates shelf freshness + brew history + Coffee Review + creator coverage into one brew-now and one buy-next pick. Cross-roaster surface: `search`, restock `watch`, closest-`twin` replacement, multi-bean `compare`, cross-roaster `producer` index. Personal analytics: per-method `shelf` freshness, Bayesian-flavored `dial-in`, K-nearest-twin `predict-rating`, `whats-next` shelf-priority picks, OLS rating `drift`, per-bag `bag-life` curves, `refill-plan` projected depletion + twin suggestions, monthly spend + cost-per-cup via `budget`, origin-coverage tracking via `coffee-map`, `blind-cup` Spearman calibration vs Coffee Review, `palate-map` descriptor signature, SCA `flavor-wheel` mapping, `friend-pick` palate sharing. Editorial + economics: `creator-review` and `transcript-search` over Hoffmann + Hedrick, landed-cost `fx` quotes with curated shipping, `champion-replay` matching of championship-style recipes against in-stock lots. Operational: corpus aggregations via `analytics`, recently-synced rows via `tail`, per-source sync history via `jobs`, multi-source orchestration via `sync` and `workflow`.

A standalone cafe locator and a few editorial-overlay commands remain Phase 3 follow-up — they require new external sync sources that ship in a later print.

Learn more at [Coffee GOAT](https://github.com/mvanhorn/printing-press-library).

Printed by [@justinwfu](https://github.com/justinwfu) (Justin Fu).

## Install

The recommended path installs both the `coffee-goat-pp-cli` binary and the `pp-coffee-goat` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press install coffee-goat
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install coffee-goat --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press install coffee-goat --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press install coffee-goat --agent claude-code
npx -y @mvanhorn/printing-press install coffee-goat --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/food-and-dining/coffee-goat/cmd/coffee-goat-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/coffee-goat-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-coffee-goat --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-coffee-goat --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-coffee-goat skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-coffee-goat. The skill defines how its required CLI can be installed.
```

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/coffee-goat-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/food-and-dining/coffee-goat/cmd/coffee-goat-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "coffee-goat": {
      "command": "coffee-goat-pp-mcp"
    }
  }
}
```

</details>

## Authentication

No API key required. All shipped sources are no-auth: roaster storefronts via public /products.json, Coffee Review WP REST, and YouTube transcripts via the youtube-pp-cli sibling (timedtext endpoint). Optional: `OPENROUTER_API_KEY` enables bean-bag photo scanning if/when the scan command is wired up. youtube-pp-cli must be on PATH for creator features; run `coffee-goat doctor` to verify.

## Quick Start

```bash
# Confirm youtube-pp-cli is on PATH and the representative roaster (Onyx) is reachable
coffee-goat doctor


# Pull catalogs from Shopify roasters, Coffee Review, and the Hoffmann + Hedrick YouTube channels into the local store
coffee-goat sync


# Cross-roaster origin/process search — every Ethiopian natural under $25 ($25.00 = 2500 cents) across the global shelf
coffee-goat search "ethiopia natural" --in-stock --price-lt 2500 --agent


# Find the closest current match across roasters for a bean you loved (use any synced roaster_products handle)
coffee-goat twin sey-banko-gotiti --top 5 --agent


# Show every Hoffmann/Hedrick clip mentioning Onyx with transcript excerpts
coffee-goat creator-review onyx --agent


# Integrates shelf + brews + Coffee Review + creator transcripts into one brew-now and one buy-next pick (works against fixture data; cellar/brew CLI is Phase 3 follow-up)
coffee-goat god-cup --method espresso --agent

```

## Recipes

Short scripts that combine multiple commands to answer a question.

**Find a single-origin gesha under $40 that pairs with a champion-replay routine.**

```bash
coffee-goat search "gesha" --in-stock --price-lt 4000 --json --compact
coffee-goat champion-replay list --json | jq '.recipes[] | select(.varietal=="gesha")'
coffee-goat champion-replay shop wbrc-2023-wolfl-bermudez --in-stock
```

**Rebuild your palate signature and check whether your shelf still fits it.**

```bash
coffee-goat palate-map --json
coffee-goat shelf --by signature-match --json
coffee-goat whats-next --json
```

**Weekly review: what's depleting, what to refill, how much you spent.**

```bash
coffee-goat shelf --by freshness
coffee-goat refill-plan --json
coffee-goat budget --by month --json
```

**Map your palate against a championship-style recipe and find lots that match.**

```bash
coffee-goat champion-replay list --year 2023
coffee-goat champion-replay shop wbrc-2023-style-bermudez-gesha --in-stock
coffee-goat palate-map --json
```

## Unique Features

These capabilities aren't available in any other tool for this API.

### The God cup
- **`god-cup`** — Integrates your shelf, brew history, Coffee Review scores, Hoffmann/Hedrick coverage, and champion recipes to produce one brew-now pick and one buy-next pick.

  _Pick this as the headline 'what should I do next' question; integrates every signal source into one answer._

  ```bash
  coffee-goat god-cup --method espresso --agent
  ```

### Cross-source corpus
- **`search`** — Search every elite specialty roaster's catalog at once — Ethiopian naturals across the curated shelf in one query.

  _Pick this when an agent needs a cross-source, structured product query rather than visiting many storefronts._

  ```bash
  coffee-goat search "ethiopia natural" --in-stock --price-lt 2500 --agent
  ```
- **`watch`** — Saved search emits only new matches on each sync — silent when nothing changed, cron-safe.

  _Pick this when a user wants to be alerted the moment a specific producer or origin appears anywhere._

  ```bash
  coffee-goat watch save bermudez "diego bermudez gesha" --agent
  ```
- **`twin`** — Given a bean you loved, find the closest current match across the curated shelf via origin/varietal/process/altitude/descriptor similarity.

  _Pick this to replace a sold-out favorite without manually scanning every other roaster._

  ```bash
  coffee-goat twin sey-banko-gotiti --top 5 --agent
  ```
- **`compare`** — Side-by-side bean table with origin/process/altitude/$/oz/score deltas; works across roasters.

  _Pick this when choosing between candidates from different roasters with structured deltas._

  ```bash
  coffee-goat compare sey-banko-gotiti glitch-yirgacheffe --agent
  ```
- **`producer`** — Track a producer or farm (Diego Bermudez, Wush Wush, El Paraiso) across every roaster sync — multi-year lineage view.

  _Pick this to compare different roasters' interpretations of one producer or to track lot evolution._

  ```bash
  coffee-goat producer "Diego Bermudez" --min-roasters 3 --agent
  ```
- **`fx`** — Live ECB FX rates normalize $/oz across continents; curated shipping table adds ship-to landed cost.

  _Pick this before ordering from JP/DK/UK/DE roasters to compare true landed cost._

  ```bash
  coffee-goat fx tim-wendelboe-finca-el-paraiso --target USD --to us-domestic --agent
  ```

### Personal-history analytics
- **`dial-in`** — Suggest grind/dose/yield/time for a new bean using your brew history of similar beans (origin/process/varietal/altitude clusters).

  _Pick this to start with a confidence-banded recipe instead of burning beans on cold-start dial-ins._

  ```bash
  coffee-goat dial-in sey-banko-gotiti --method espresso --agent
  ```
- **`shelf`** — What's on your shelf today, sorted by freshness with per-method peak windows (espresso 8–21d, filter 5–28d).

  _Pick this to answer 'what should I open next' with hard freshness rules._

  ```bash
  coffee-goat shelf --method v60 --agent
  ```
- **`whats-next`** — Joins shelf freshness × dial-in confidence × user palate signature to pick one bag from your shelf for tomorrow morning.

  _Pick this when the household agent needs one deterministic 'brew this' answer at dawn._

  ```bash
  coffee-goat whats-next --method v60 --agent
  ```
- **`drift`** — Detect grinder/water/technique drift via fixed-effects OLS on rating-vs-day across multiple beans; partitions variance by method.

  _Pick this when ratings slide across unrelated beans — the CLI tells you it's your hardware, not the coffee._

  ```bash
  coffee-goat drift --method espresso --agent
  ```
- **`refill-plan`** — Combines consumption rate from your brews + shelf depletion + cross-roaster twin similarity to recommend 3 replacement picks.

  _Pick this to never run out of coffee and always restock toward your taste profile._

  ```bash
  coffee-goat refill-plan --agent
  ```
- **`blind-cup`** — Blind cupping session with hashed cup IDs; reveal at end and report Spearman correlation against Coffee Review's scores.

  _Pick this to quantify how your palate maps to published scores over time._

  ```bash
  coffee-goat blind-cup --agent
  ```
- **`palate-map`** — Aggregate your 8+ rated brews into an origin/process/varietal weight signature, then rank current shelf and restock candidates by signature match.

  _Pick this to see what flavor profile you actually prefer (vs what you claim), and rank everything by it._

  ```bash
  coffee-goat palate-map --agent
  ```
- **`bag-life`** — For one bag, plot daily ratings vs days-since-roast and call peak / decline / dead.

  _Pick this to decide whether a struggling shot is the bag dying or your technique slipping._

  ```bash
  coffee-goat bag-life onyx-geometry --agent
  ```
- **`friend-pick`** — Recommend a bag for a friend whose palate profile you've imported — pick from your shelf or current market, ranked against their descriptor signature instead of yours.

  _Pick this when an agent needs to recommend a gift bag or birthday-coffee pick that fits someone else's documented palate._

  ```bash
  coffee-goat friend-pick pick maya --from market --top 3 --agent
  ```
- **`flavor-wheel`** — Maps your brew ratings onto the official SCA Coffee Tasters' Flavor Wheel hierarchy (fruity → berry → blackberry, floral → tea-like → black tea, etc.) and shows which sections you actually prefer.

  _Pick this when the user wants to get clearer on what flavor families they actually like vs claim to like, against the canonical specialty-coffee taxonomy._

  ```bash
  coffee-goat flavor-wheel --agent --select preferred,avoided,sparse_coverage
  ```
- **`brews`** — Brew log (log/list/show/delete) and local cellar (cellar add/list/show/update/remove) — the input feed for every personal-history command.

  _Pick this to start building the personal-history signal that powers dial-in, drift, palate-map, predict-rating, and whats-next._

  ```bash
  coffee-goat brews log --bean 3 --method v60 --dose-g 18 --yield-g 300 --time-s 180 --rating 8
  ```
- **`budget`** — Dose-grams attribution (dose_g / bag_size_g × bag_price). YTD + projection range (linear + trailing-3-mo × 12). Pivot --by month|roaster|bag|method. --include-shipping uses fx's curated table.

  _Pick this to answer 'am I overspending vs last quarter?' or 'which bag was actually expensive per-cup?' — money axis nothing else touches._

  ```bash
  coffee-goat budget --by bag --agent
  ```
- **`coffee-map`** — Country-primary coverage with tiered depth labels (unexplored/tasted/explored/deep). Synced corpus default; --world adds curated specialty origins. --fill suggests bags inline; `coffee-map fill` ranks across all gaps by CR score + process tiebreak.

  _Pick this to discover gaps in your origin coverage ('you've never had a Burundi') and get a concrete bag to close each gap._

  ```bash
  coffee-goat coffee-map --fill --agent
  ```
- **`predict-rating`** — Predicts YOUR rating for a (bean, method) pair via K=5 nearest twins from your brew log. Subcommand split: `predict-rating [bean]` and `predict-rating cellar` write to predicted_ratings; `predict-rating calibration` is mcp:read-only and reports MAE+bias sliced by method/origin/process.

  _Pick this before buying a new bag to estimate how you'll rate it, and run calibration periodically to see if the model over-predicts naturals or under-predicts darks._

  ```bash
  coffee-goat predict-rating sey/banko-gotiti --method v60 --agent
  ```
- **`cupping`** — Full 10-attribute SCA form, multi-cupper sessions (auto-creates palate_profiles), open default + --blind slot labels. 7 verbs: start/score/finalize/abandon/show/list/log. Finalize bridges cupper=self scores to the brews table as method='cupping'.

  _Pick this to run a structured tasting (solo or multi-cupper, open or blind) and have the scores flow into drift/palate-map/predict-rating automatically._

  ```bash
  coffee-goat cupping start --name "Sat AM" --bean sey/banko-gotiti --bean april/ethiopia-natural
  ```

### Editorial signal
- **`creator-review`** — Find every Hoffmann or Hedrick clip that mentions a bean or roaster, with transcript excerpt and timestamp.

  _Pick this when the user remembers Hoffmann or Hedrick talking about a bag and wants the exact clip in 30 seconds, not a 20-minute rewatch._

  ```bash
  coffee-goat creator-review sey-banko-gotiti --agent
  ```
- **`transcript-search`** — Local FTS5 search across all synced Hoffmann + Hedrick transcripts with timestamp + video link.

  _Pick this when an agent needs the exact clip where a creator said X._

  ```bash
  coffee-goat transcript-search "flow profiling" --agent --select results.video_title,results.timestamp,results.excerpt
  ```
- **`recipe-replay`** — Hardcoded library of 10 canonical V60 recipes (Hoffmann 2018/2023, Tetsu 4:6 light/dark, Lance Pulse, WBrC 2020-2024 champions) with structured 5-tuple + technique markdown.

  _Pick this to bridge from theoretical champion routines to your specific bean — match scores by bean features, apply scales dose+water, --log pre-fills a brews row._

  ```bash
  coffee-goat recipe-replay apply hoffmann-ultimate-2023 sey/banko-gotiti --dose 18 --log
  ```

## Usage

Run `coffee-goat-pp-cli --help` for the full command reference and flag list.

## Commands

### products

Unified roaster product corpus synced from 24 specialty coffee roasters

- **`coffee-goat-pp-cli products`** - List synced roaster products with structured filters


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
coffee-goat-pp-cli products

# JSON for scripting and agents
coffee-goat-pp-cli products --json

# Filter to specific fields
coffee-goat-pp-cli products --json --select id,name,status

# Dry run — show the request without sending
coffee-goat-pp-cli products --dry-run

# Agent mode — JSON + compact + no prompts in one flag
coffee-goat-pp-cli products --agent
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
coffee-goat-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Config file: `~/.config/coffee-goat-pp-cli/config.toml` (override with `COFFEE_GOAT_CONFIG`)

Static request headers can be configured under `headers`; per-command header overrides take precedence.

### Environment variables

- `COFFEE_GOAT_CONFIG` — override the config file path
- `COFFEE_GOAT_BASE_URL` — override the upstream base URL (used by `printing-press verify` to point at a mock server)
- `COFFEE_GOAT_CLI_PATH` — absolute path to the CLI binary used when the MCP server spawns CLI subprocesses
- `COFFEE_GOAT_FEEDBACK_ENDPOINT` — opt-in HTTPS endpoint that receives `coffee-goat feedback` submissions
- `COFFEE_GOAT_FEEDBACK_AUTO_SEND` — set to `1`/`true` to send feedback immediately without the local review prompt
- `NO_COLOR` — disable colored output (also honored by `--agent`)

## Troubleshooting

**Not found errors (exit code 3)**
- Check the bean handle or roaster slug is correct
- Run `coffee-goat products` (filtered with `--roaster <slug>` or `--query <text>`) to see available beans
- Run `coffee-goat sync` if the local store is empty

### API-specific

- **doctor says "youtube-pp-cli not found on PATH"** — Install youtube-pp-cli from the printing-press public library, then rerun `coffee-goat doctor`. Creator features (creator-review, god-cup) are degraded but not broken without it — sync just skips the YouTube source with a warning.
- **sync returns 0 products for a roaster** — Run `coffee-goat sync --source shopify --roaster <slug>` to scope to one roaster; the registry handles known corrections for Leaves and Friedhats (their canonical hostnames).
- **search returns nothing after sync** — Confirm `coffee-goat sync` exited 0 and reported items for at least one source. Then try a broader query like `coffee-goat search ethiopia` to rule out FTS5 tokenizer edge cases.
- **twin returns very low similarity scores** — Synced descriptor coverage varies by roaster. Tag and origin fields drive most of the similarity; sparse descriptor coverage on a roaster will reduce twin precision for its bags.
- **creator-review returns no clips for a bag you remember being mentioned** — Bean-mention extraction is regex match against the registered roaster slug/name set. If the creator referred to the roaster by a non-canonical name, the regex misses it. Re-run `coffee-goat sync --source youtube` after updating registry names.
- **flavor-wheel shows mostly empty wheel sections** — flavor-wheel needs at least 8 rated brews to populate the SCA wheel meaningfully. Personal brew-log CLI (`brews log`) is Phase 3 follow-up; for now seed via SQL: `INSERT INTO brews(bean_id, rating, descriptors_json) VALUES (...)`.
- **friend-pick says "no palate profile found"** — Run `coffee-goat friend-pick palate-export <your-name> --out you.json` on your machine and have your friend run the same; exchange JSONs and import via `coffee-goat friend-pick palate-import friend.json`.

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**Beanconqueror**](https://github.com/graphefruit/Beanconqueror) — TypeScript
- [**Brewlog**](https://github.com/jnsgruk/brewlog) — Rust
- [**Artisan**](https://github.com/artisan-roaster-scope/artisan) — Python

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
