---
name: pp-coffee-goat
description: "The third-wave coffee terminal — every elite roaster, every YouTube creator review, your brews, and the God cup. Trigger phrases: `did hoffmann review this bag`, `any new ethiopian naturals`, `the god cup`, `find a bean like the one I loved`, `show me my flavor wheel`, `what flavors do I actually like`, `recommend a bag for my friend`, `use coffee-goat`, `run coffee-goat`."
author: "Justin Fu"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - coffee-goat-pp-cli
    install:
      - kind: go
        bins: [coffee-goat-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/food-and-dining/coffee-goat/cmd/coffee-goat-pp-cli
---
<!-- GENERATED FILE — DO NOT EDIT.
     This file is a verbatim mirror of library/food-and-dining/coffee-goat/SKILL.md,
     regenerated post-merge by tools/generate-skills/. Hand-edits here are
     silently overwritten on the next regen. Edit the library/ source instead.
     See AGENTS.md "Generated artifacts: registry.json, cli-skills/". -->

# Coffee GOAT — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `coffee-goat-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press-library install coffee-goat --cli-only
   ```
2. Verify: `coffee-goat-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/food-and-dining/coffee-goat/cmd/coffee-goat-pp-cli@latest
```

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

## When to Use This CLI

Use coffee-goat when an agent needs to compare or recommend specialty coffee across many roasters at once, when answering 'what should I drink next' or 'what should I buy next' from a user's logged cellar, when finding YouTube creator coverage of a specific bag in seconds instead of rewatching a 20-minute video, when matching champion recipes to currently-buyable beans, when mapping a user's palate onto the SCA flavor wheel, when picking a gift bag for a friend whose palate profile is imported, when finding cafes near a location, or when diagnosing whether a user's recent low ratings are bean quality or equipment drift. Do NOT use it for ordering — coffee-goat is read-only across upstream sources. Do NOT use it to find which beans a cafe serves — no public no-auth source links cafes to beans.

## When Not to Use This CLI

Do not activate this CLI for requests that require creating, updating, deleting, publishing, commenting, upvoting, inviting, ordering, sending messages, booking, purchasing, or changing remote state. This printed CLI exposes read-only commands for inspection, export, sync, and analysis.

## Unique Capabilities

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

## Command Reference

**products** — Unified roaster product corpus synced from 24 specialty coffee roasters

- `coffee-goat-pp-cli products` — List synced roaster products with structured filters


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
coffee-goat-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Hand-written Extensions

These commands are declared by the spec author and require separate hand-written wiring; the generator does not emit Cobra registration for them. They are listed here for discoverability and are intentionally outside `## Command Reference` so the verify-skill unknown-command check does not treat them as generator-owned paths.

- `coffee-goat-pp-cli search [query] [--in-stock] [--origin <s>] [--process <s>] [--price-lt <cents>] [--limit <N>]` — Cross-roaster FTS5 search over the synced roaster_products corpus
- `coffee-goat-pp-cli watch save <name> <query>` — Save a cross-roaster query that emits only new matches on subsequent runs
- `coffee-goat-pp-cli watch list` — List saved watch queries with last-sync-anchor
- `coffee-goat-pp-cli watch run [<name>]` — Run a saved (or all) watch queries; emits only newly-arrived matching products
- `coffee-goat-pp-cli twin <roaster-or-product-slug> [--top <N>]` — Find the closest match to a bean across all roasters via attribute + descriptor similarity
- `coffee-goat-pp-cli creator-review <bean-or-roaster-slug>` — Lookup Hoffmann/Hedrick YouTube clips mentioning a bean or roaster (with transcript excerpts)
- `coffee-goat-pp-cli flavor-wheel` — Map your rated brews onto the official SCA Coffee Tasters' Flavor Wheel hierarchy
- `coffee-goat-pp-cli friend-pick palate-export <name> --out <file>` — Export a portable JSON palate profile derived from your brews + ratings + descriptors
- `coffee-goat-pp-cli friend-pick palate-import <file>` — Import a friend's palate profile into the local store
- `coffee-goat-pp-cli friend-pick pick <name> [--from market|shelf] [--top <N>]` — Recommend bags ranked against an imported friend's palate signature (not yours)
- `coffee-goat-pp-cli god-cup [--method espresso|v60|aeropress|...]` — Headline meta-recommendation: one brew pick from your shelf + one buy pick across the market, integrating shelf...
- `coffee-goat-pp-cli brews log [--bean <id>] [--method <s>] [--dose-g <f>] [--yield-g <f>] [--time-s <i>] [--rating <0..10>] [--descriptors <csv>]` — Record a brew. Required input for every personal-history command.
- `coffee-goat-pp-cli brews list [--bean <id>] [--method <s>] [--limit <N>] [--since <rfc3339>]` — List recent brews ordered by brewed_at descending
- `coffee-goat-pp-cli brews show <id>` / `coffee-goat-pp-cli brews delete <id>` — Per-brew detail and removal
- `coffee-goat-pp-cli brews cellar add --roaster <s> --product <s> [--roast-date <date>] [--mass-g <g>]` — Add a bag to the local cellar so brews can link to it
- `coffee-goat-pp-cli brews cellar list/show/update/remove` — Cellar inventory CRUD
- `coffee-goat-pp-cli compare <bean> [<bean>...]` — Side-by-side compare of two or more beans on origin/process/varietal/altitude/price + Coffee Review + your own brew rating
- `coffee-goat-pp-cli producer [name] [--min-roasters <N>] [--in-stock] [--limit <N>]` — Cross-roaster producer index over roaster_products.producer
- `coffee-goat-pp-cli transcript-search <query> [--creator hoffmann|hedrick] [--limit <N>]` — FTS5 over Hoffmann + Hedrick transcripts with approximate-timestamp video URLs
- `coffee-goat-pp-cli shelf [--method <s>]` — Per-method peak-freshness status (resting / peak / past-peak / stale) for every bag in the cellar
- `coffee-goat-pp-cli dial-in <bean-id-or-roaster/product> --method <s>` — Bayesian dial-in suggestion blending user brews with origin+process cluster averages
- `coffee-goat-pp-cli whats-next --method <s> [--limit <N>]` — Score every bag in the cellar; return top picks by freshness + dial-in confidence + palate fit
- `coffee-goat-pp-cli drift [--method <s>]` — OLS slope of rating vs days-since-roast per method
- `coffee-goat-pp-cli bag-life <bean-id-or-roaster/product>` — Per-bag rating curve over days-since-roast
- `coffee-goat-pp-cli refill-plan [--lookback-days <N>] [--horizon-days <N>] [--twins-per-bag <N>]` — Project depletion across the cellar and suggest twin replacements
- `coffee-goat-pp-cli blind-cup` — Spearman rank correlation of user brew ratings vs Coffee Review scores
- `coffee-goat-pp-cli palate-map [--top <N>]` — Top loved / avoided descriptor tokens + per-origin mean rating
- `coffee-goat-pp-cli fx <bean> [--target USD|EUR|GBP|...] [--to us-domestic|us-intl|eu|uk]` — Landed-cost quote: convert listed price + curated shipping to target currency

## Recipes


### The God cup — one brew, one buy

```bash
coffee-goat god-cup --method v60 --agent --select brew_pick.bean,brew_pick.score,brew_pick.rationale,buy_pick.product,buy_pick.roaster,buy_pick.rationale
```

The eponymous command. Integrates shelf freshness, brew history, Coffee Review scores, and creator coverage into one brew-now pick and one buy-next pick. The --select narrows to the agent-relevant fields when the response is deep.

### Did Hoffmann or Hedrick ever review this bag

```bash
coffee-goat creator-review sey-banko-gotiti --agent --select results.creator,results.video_title,results.timestamp,results.transcript_excerpt
```

Returns every clip that mentions the bag, with timestamp and a snippet — answers in 30 seconds what would otherwise be a 20-minute rewatch.

### Restock alert across roasters

```bash
coffee-goat watch save kenyas "kenya" --agent
```

Save a cross-roaster query; on subsequent runs `coffee-goat watch run kenyas` emits only new matches since last sync.

### Map your palate onto the SCA Flavor Wheel

```bash
coffee-goat flavor-wheel --agent --select preferred,avoided,sparse_coverage
```

Aggregates your rated brews against the official SCA Coffee Tasters Flavor Wheel hierarchy — shows which flavor families you actually like.

### Recommend a bag for a friend whose palate you have

```bash
coffee-goat friend-pick pick anne --from market --top 3 --agent
```

Import a friend palate via `coffee-goat friend-pick palate-import friend.json`; pick recommends bags matching their descriptor signature, not yours.

### Log a brew and ask what to brew next

```bash
coffee-goat brews cellar add --roaster sey --product banko-gotiti --roast-date 2026-05-10 --mass-g 250
coffee-goat brews log --bean 1 --method v60 --dose-g 18 --yield-g 300 --time-s 180 --rating 8 --descriptors "blackberry,plum"
coffee-goat whats-next --method v60 --agent
```

Captures one brew, then ranks every bag in the cellar by combined freshness + dial-in confidence + palate fit. Run `whats-next` whenever the user asks "what should I brew next."

### Plan reorders for the week

```bash
coffee-goat refill-plan --horizon-days 7 --twins-per-bag 3 --agent
```

Projects how many days of coffee remain for each bag from recent brew-log dose use; flags bags running out inside the horizon and suggests cross-roaster twin replacements ranked by similarity.

### Compare two bags head-to-head

```bash
coffee-goat compare sey/banko-gotiti onyx/geisha-honey --agent
```

Joins both bags' attributes + Coffee Review score + your own brew rating into one row each. Use this when deciding between two specific bags rather than browsing.

### Find the Hedrick puck-prep clip

```bash
coffee-goat transcript-search "puck prep" --creator hedrick --agent
```

Returns matching excerpts with an approximate `&t=<seconds>` jump URL — works when the user remembers a technique but not the bag.

## Auth Setup

No authentication required.

Run `coffee-goat-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  coffee-goat-pp-cli products --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag
- **Read-only** — do not use this CLI for create, update, delete, publish, comment, upvote, invite, order, send, or other mutating requests

### Response envelope

Commands that read from the local store or the API wrap output in a provenance envelope:

```json
{
  "meta": {"source": "live" | "local", "synced_at": "...", "reason": "..."},
  "results": <data>
}
```

Parse `.results` for data and `.meta.source` to know whether it's live or local. A human-readable `N results (live)` summary is printed to stderr only when stdout is a terminal AND no machine-format flag (`--json`, `--csv`, `--compact`, `--quiet`, `--plain`, `--select`) is set — piped/agent consumers and explicit-format runs get pure JSON on stdout.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
coffee-goat-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
coffee-goat-pp-cli feedback --stdin < notes.txt
coffee-goat-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.coffee-goat-pp-cli/feedback.jsonl`. They are never POSTed unless `COFFEE_GOAT_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `COFFEE_GOAT_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
coffee-goat-pp-cli profile save briefing --json
coffee-goat-pp-cli --profile briefing products
coffee-goat-pp-cli profile list --json
coffee-goat-pp-cli profile show briefing
coffee-goat-pp-cli profile delete briefing --yes
```

Explicit flags always win over profile values; profile values win over defaults. `agent-context` lists all available profiles under `available_profiles` so introspecting agents discover them at runtime.

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 2 | Usage error (wrong arguments) |
| 3 | Resource not found |
| 5 | API error (upstream issue) |
| 7 | Rate limited (wait and retry) |
| 10 | Config error |

## Argument Parsing

Parse `$ARGUMENTS`:

1. **Empty, `help`, or `--help`** → show `coffee-goat-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/food-and-dining/coffee-goat/cmd/coffee-goat-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add coffee-goat-pp-mcp -- coffee-goat-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which coffee-goat-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   coffee-goat-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `coffee-goat-pp-cli <command> --help`.
