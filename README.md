# Printing Press Library

Nothing is more valuable than time and money. In a world of AI agents, that's speed and token spend. A well-designed CLI is muscle memory for an agent: no hunting through docs, no wrong turns, no wasted tokens. The [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press) prints those CLIs. This repo is the catalog of CLIs already printed and ready to install.

24 CLIs across 17 categories. 17 ship a full MCP server. Browse them all at [printingpress.dev](https://printingpress.dev).

Three to try first:

- ESPN (sniffed, no official API). _"Tonight's NBA playoff games with live score, series state, each team's leading scorer's stat line, and any injury or lineup news from the last 24 hours."_ One call.
- flight-goat (Kayak nonstop search plus sniffed Google Flights). _"Non-stop flights over 8 hours from Seattle for 4 people, Dec 24 to Jan 1, cheapest first."_ Two sources, one query.
- linear-pp-cli (50ms against a local SQLite mirror). _"Every blocked issue whose blocker has been stuck for a week."_ Compound queries the Linear API can't answer.

## Release status

The v0.1.0 npm installer and `cli-skills/` direct-install namespace are prepared in this repo, but public use is waiting on three release steps:

1. Merge this work to `main`, so `cli-skills/` exists on the default branch.
2. Publish `@mvanhorn/printing-press` to npm.
3. Make the repo public, or document the private-repo token setup for early users.

While the repo is private, live installer use requires `GITHUB_TOKEN` or `GH_TOKEN` for catalog and skill fetches, plus private Go module access for `go install`.

## Install a CLI

After v0.1.0 is published, the primary install path is:

```bash
npx -y @mvanhorn/printing-press install espn
```

That command installs the Go binary and the focused `pp-espn` skill. The npm package is intentionally thin: it reads the live catalog in `registry.json`, resolves the CLI's Go module path, runs `go install`, and installs the matching skill from `cli-skills/pp-<name>`.

Useful commands:

```bash
npx -y @mvanhorn/printing-press search sports
npx -y @mvanhorn/printing-press list
npx -y @mvanhorn/printing-press update espn
npx -y @mvanhorn/printing-press uninstall espn --yes
```

## Use the plugin router

This repo is also a Claude Code plugin marketplace. Add it and install the plugin:

```text
/plugin marketplace add mvanhorn/printing-press-library
/plugin install printing-press-library@printing-press-library
```

The plugin exposes the catalog router skill:

```text
/ppl
```

/ppl handles discovery, routing, and install guidance:

```text
/ppl
/ppl sports scores
/ppl install espn
/ppl espn lakers score
/ppl linear my open issues
```

The plugin intentionally does not install every focused `/pp-*` skill by default. Focused skills live under `cli-skills/` for direct installation, so users can install only the tools they want.

Want to print new CLIs from API specs? Install the Printing Press itself too:

```text
/plugin marketplace add mvanhorn/cli-printing-press
/plugin install cli-printing-press@cli-printing-press
```

## Focused skills

When you already know the tool you want, install just that skill:

```bash
npx skills add mvanhorn/printing-press-library/cli-skills/pp-espn -g
```

Then use the focused slash skill directly:

```text
/pp-espn lakers score
/pp-flightgoat sea to lax dec 24 to jan 1 nonstop
/pp-weather-goat phoenix forecast
```

`/ppl` is the catalog router. Each `/pp-<name>` skill is a focused interface for one CLI.

## Catalog

Tools grouped by category, sourced from [`registry.json`](registry.json). Each row links to the tool source and its focused direct-install skill.

| Name | Skill | Auth | MCP | Slash install | What it does |
|------|-------|------|-----|---------------|--------------|
| [`agent-capture`](library/developer-tools/agent-capture/) | [`/pp-agent-capture`](cli-skills/pp-agent-capture/SKILL.md) | local only | no | `/ppl install agent-capture cli` | Record, screenshot, and convert macOS windows and screens for agent evidence. |
| [`archive-is`](library/media-and-entertainment/archive-is/) | [`/pp-archive-is`](cli-skills/pp-archive-is/SKILL.md) | none | full | `/ppl install archive-is cli` | Find and create Archive.today snapshots for URLs. |
| [`cal-com`](library/productivity/cal-com/) | [`/pp-cal-com`](cli-skills/pp-cal-com/SKILL.md) | API key | full | `/ppl install cal-com cli` | Manage bookings, schedules, event types, and availability. |
| [`contact-goat`](library/sales-and-crm/contact-goat/) | [`/pp-contact-goat`](cli-skills/pp-contact-goat/SKILL.md) | mixed | full | `/ppl install contact-goat cli` | Cross-source warm-intro graph across LinkedIn, Happenstance, and Deepline with a unified local store. |
| [`dominos-pp-cli`](library/commerce/dominos-pp-cli/) | [`/pp-dominos`](cli-skills/pp-dominos/SKILL.md) | browser login | full | `/ppl install dominos cli` | Order Domino's, browse menus, and track deliveries. |
| [`dub`](library/marketing/dub/) | [`/pp-dub`](cli-skills/pp-dub/SKILL.md) | API key | full | `/ppl install dub cli` | Create short links, track analytics, and manage domains. |
| [`espn`](library/media-and-entertainment/espn/) | [`/pp-espn`](cli-skills/pp-espn/SKILL.md) | none | full | `/ppl install espn cli` | Live scores, standings, schedules, and sports news. |
| [`flightgoat`](library/travel/flightgoat/) | [`/pp-flightgoat`](cli-skills/pp-flightgoat/SKILL.md) | API key optional | full | `/ppl install flightgoat cli` | Search flights, explore routes, and track flights. |
| [`hackernews`](library/media-and-entertainment/hackernews/) | [`/pp-hackernews`](cli-skills/pp-hackernews/SKILL.md) | none | full | `/ppl install hackernews cli` | Browse stories, comments, jobs, and topic slices from Hacker News. |
| [`hubspot-pp-cli`](library/sales-and-crm/hubspot/) | [`/pp-hubspot`](cli-skills/pp-hubspot/SKILL.md) | API key | full | `/ppl install hubspot cli` | Work with contacts, companies, deals, tickets, and pipelines. |
| [`instacart`](library/commerce/instacart/) | [`/pp-instacart`](cli-skills/pp-instacart/SKILL.md) | browser session | no | `/ppl install instacart cli` | Search products, manage carts, and shop Instacart from the terminal. |
| [`kalshi`](library/payments/kalshi/) | [`/pp-kalshi`](cli-skills/pp-kalshi/SKILL.md) | API key | full | `/ppl install kalshi cli` | Trade markets, inspect portfolios, and analyze odds. |
| [`linear`](library/project-management/linear/) | [`/pp-linear`](cli-skills/pp-linear/SKILL.md) | API key | full | `/ppl install linear cli` | Manage issues, cycles, teams, and projects with local sync. |
| [`movie-goat`](library/media-and-entertainment/movie-goat/) | [`/pp-movie-goat`](cli-skills/pp-movie-goat/SKILL.md) | bearer token | full | `/ppl install movie-goat cli` | Compare movie ratings, streaming availability, and recommendations. |
| [`pagliacci-pizza`](library/food-and-dining/pagliacci-pizza/) | [`/pp-pagliacci-pizza`](cli-skills/pp-pagliacci-pizza/SKILL.md) | browser login | partial | `/ppl install pagliacci-pizza cli` | Order Pagliacci and browse public menu and store data without login. |
| [`pokeapi`](library/media-and-entertainment/pokeapi/) | [`/pp-pokeapi`](cli-skills/pp-pokeapi/SKILL.md) | none | full | `/ppl install pokeapi cli` | PokeAPI as an agent-ready knowledge graph plus matchup and team-coverage workflows. |
| [`postman-explore`](library/developer-tools/postman-explore/) | [`/pp-postman-explore`](cli-skills/pp-postman-explore/SKILL.md) | none | full | `/ppl install postman-explore cli` | Search and browse the Postman API Network. |
| [`producthunt`](library/marketing/producthunt/) | [`/pp-producthunt`](cli-skills/pp-producthunt/SKILL.md) | none | full | `/ppl install producthunt cli` | Token-free Product Hunt CLI with local sync and views the website doesn't expose. |
| [`recipe-goat`](library/food-and-dining/recipe-goat/) | [`/pp-recipe-goat`](cli-skills/pp-recipe-goat/SKILL.md) | API key | full | `/ppl install recipe-goat cli` | Find recipes across 37 trusted sites with trust-aware ranking and local cookbook. |
| [`sentry`](library/monitoring/sentry/) | [`/pp-sentry`](cli-skills/pp-sentry/SKILL.md) | bearer token | full | `/ppl install sentry cli` | Error tracking and performance monitoring for projects, issues, events, and releases. |
| [`slack`](library/productivity/slack/) | [`/pp-slack`](cli-skills/pp-slack/SKILL.md) | API key | full | `/ppl install slack cli` | Send messages, search conversations, and monitor channels. |
| [`steam-web`](library/media-and-entertainment/steam-web/) | [`/pp-steam-web`](cli-skills/pp-steam-web/SKILL.md) | API key | full | `/ppl install steam-web cli` | Look up Steam players, games, achievements, and stats. |
| [`trigger-dev`](library/developer-tools/trigger-dev/) | [`/pp-trigger-dev`](cli-skills/pp-trigger-dev/SKILL.md) | API key | full | `/ppl install trigger-dev cli` | Monitor runs, trigger tasks, and inspect schedules and failures. |
| [`weather-goat`](library/other/weather-goat/) | [`/pp-weather-goat`](cli-skills/pp-weather-goat/SKILL.md) | none | full | `/ppl install weather-goat cli` | Forecasts, alerts, air quality, and activity verdicts. |
| [`yahoo-finance`](library/commerce/yahoo-finance/) | [`/pp-yahoo-finance`](cli-skills/pp-yahoo-finance/SKILL.md) | none | full | `/ppl install yahoo-finance cli` | Quotes, charts, fundamentals, options, and watchlists. |

## Binary-only install

If you only want the binary and not the companion skill, install directly with [Go 1.23+](https://go.dev/dl/):

```bash
go install github.com/mvanhorn/printing-press-library/<path>/cmd/<binary>@latest
```

A few worked examples:

```bash
go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/espn/cmd/espn-pp-cli@latest
go install github.com/mvanhorn/printing-press-library/library/project-management/linear/cmd/linear-pp-cli@latest
go install github.com/mvanhorn/printing-press-library/library/travel/flightgoat/cmd/flightgoat-pp-cli@latest
```

For the MCP server companion:

```bash
go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/espn/cmd/espn-pp-mcp@latest
claude mcp add espn-pp-mcp -- espn-pp-mcp
```

If a CLI needs credentials, the focused skill and the per-CLI README document the required environment variables.

## Repo structure

```text
library/
  <category>/
    <tool>/
      cmd/
        <cli-binary>/
        <mcp-binary>/        # when available
      internal/
      README.md
      go.mod
      .printing-press.json
      .manuscripts/

.claude-plugin/
  marketplace.json
  plugin.json

cli-skills/
  pp-*/
    SKILL.md                 # generated direct-install mirror of library/<.>/SKILL.md

npm/
  package.json
  src/
  bin/

skills/
  ppl/
    SKILL.md

registry.json
```

Each published tool is self-contained: source code, a local README, a `.printing-press.json` provenance manifest, and the manuscripts from the printing run. `cli-skills/pp-*` is a generated mirror of each library `SKILL.md`, produced by `tools/generate-skills/main.go`. `skills/ppl` is the plugin-facing router skill.

## What endorsed means

Every published tool in this repo has passed:

1. Generation from an API spec or captured interface through the [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
2. Validation checks: build, vet, help, version, plus the structural dogfood and runtime verify gates
3. Provenance capture through `.printing-press.json` and `.manuscripts/`

Some tools are refined after generation. The generated artifacts remain in the tool directory so the provenance stays inspectable.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md). For deeper architecture, see [AGENTS.md](AGENTS.md).

## License

MIT
