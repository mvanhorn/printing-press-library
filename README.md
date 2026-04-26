# Printing Press Library

Nothing is more valuable than time and money. In a world of AI agents, that's speed and token spend. A well-designed CLI is muscle memory for an agent: no hunting through docs, no wrong turns, no wasted tokens. The [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press) prints those CLIs. This repo is the catalog of CLIs already printed and ready to install.

24 CLIs across 17 categories. 17 ship a full MCP server. Browse them all at [printingpress.dev](https://printingpress.dev).

Three to try first:

- ESPN (sniffed, no official API). _"Tonight's NBA playoff games with live score, series state, each team's leading scorer's stat line, and any injury or lineup news from the last 24 hours."_ One call.
- flight-goat (Kayak nonstop search plus sniffed Google Flights). _"Non-stop flights over 8 hours from Seattle for 4 people, Dec 24 to Jan 1, cheapest first."_ Two sources, one query.
- linear-pp-cli (50ms against a local SQLite mirror). _"Every blocked issue whose blocker has been stuck for a week."_ Compound queries the Linear API can't answer.

## Start here

This repo is itself a Claude Code plugin marketplace. Add it, install the plugin, and you have every CLI in the catalog one slash-command away.

```text
/plugin marketplace add mvanhorn/printing-press-library
/plugin install printing-press-library@printing-press-library
```

After install, the main router skill is:

```text
/ppl
```

Want to print new CLIs from API specs? Install the Printing Press itself too:

```text
/plugin marketplace add mvanhorn/cli-printing-press
/plugin install cli-printing-press@cli-printing-press
```

## Two ways in

The repository and plugin are named `printing-press-library`. The mega-skill you actually use is `/ppl`. That naming split is intentional.

Use `/ppl` when you want discovery, routing, or installation. Examples:

```text
/ppl
/ppl sports scores
/ppl install espn cli
/ppl install espn mcp
/ppl espn lakers score
/ppl linear my open issues
```

Use a focused `/pp-<name>` skill when you already know the tool you want. Examples:

```text
/pp-espn lakers score
/pp-flightgoat sea to lax dec 24 to jan 1 nonstop
/pp-weather-goat phoenix forecast
```

`/ppl` is the catalog plus the librarian. Each `/pp-<name>` is a single shelf you reach directly when you don't need help finding it.

## Catalog

Tools grouped by category, sourced from [`registry.json`](registry.json). Each row links to the tool source and its focused plugin skill.

| Name | Skill | Auth | MCP | Slash install | What it does |
|------|-------|------|-----|---------------|--------------|
| [`agent-capture`](library/developer-tools/agent-capture/) | [`/pp-agent-capture`](skills/pp-agent-capture/SKILL.md) | local only | no | `/ppl install agent-capture cli` | Record, screenshot, and convert macOS windows and screens for agent evidence. |
| [`archive-is`](library/media-and-entertainment/archive-is/) | [`/pp-archive-is`](skills/pp-archive-is/SKILL.md) | none | full | `/ppl install archive-is cli` | Find and create Archive.today snapshots for URLs. |
| [`cal-com`](library/productivity/cal-com/) | [`/pp-cal-com`](skills/pp-cal-com/SKILL.md) | API key | full | `/ppl install cal-com cli` | Manage bookings, schedules, event types, and availability. |
| [`contact-goat`](library/sales-and-crm/contact-goat/) | [`/pp-contact-goat`](skills/pp-contact-goat/SKILL.md) | mixed | full | `/ppl install contact-goat cli` | Cross-source warm-intro graph across LinkedIn, Happenstance, and Deepline with a unified local store. |
| [`dominos-pp-cli`](library/commerce/dominos-pp-cli/) | [`/pp-dominos`](skills/pp-dominos/SKILL.md) | browser login | full | `/ppl install dominos cli` | Order Domino's, browse menus, and track deliveries. |
| [`dub`](library/marketing/dub/) | [`/pp-dub`](skills/pp-dub/SKILL.md) | API key | full | `/ppl install dub cli` | Create short links, track analytics, and manage domains. |
| [`espn`](library/media-and-entertainment/espn/) | [`/pp-espn`](skills/pp-espn/SKILL.md) | none | full | `/ppl install espn cli` | Live scores, standings, schedules, and sports news. |
| [`flightgoat`](library/travel/flightgoat/) | [`/pp-flightgoat`](skills/pp-flightgoat/SKILL.md) | API key optional | full | `/ppl install flightgoat cli` | Search flights, explore routes, and track flights. |
| [`hackernews`](library/media-and-entertainment/hackernews/) | [`/pp-hackernews`](skills/pp-hackernews/SKILL.md) | none | full | `/ppl install hackernews cli` | Browse stories, comments, jobs, and topic slices from Hacker News. |
| [`hubspot-pp-cli`](library/sales-and-crm/hubspot/) | [`/pp-hubspot`](skills/pp-hubspot/SKILL.md) | API key | full | `/ppl install hubspot cli` | Work with contacts, companies, deals, tickets, and pipelines. |
| [`instacart`](library/commerce/instacart/) | [`/pp-instacart`](skills/pp-instacart/SKILL.md) | browser session | no | `/ppl install instacart cli` | Search products, manage carts, and shop Instacart from the terminal. |
| [`kalshi`](library/payments/kalshi/) | [`/pp-kalshi`](skills/pp-kalshi/SKILL.md) | API key | full | `/ppl install kalshi cli` | Trade markets, inspect portfolios, and analyze odds. |
| [`linear`](library/project-management/linear/) | [`/pp-linear`](skills/pp-linear/SKILL.md) | API key | full | `/ppl install linear cli` | Manage issues, cycles, teams, and projects with local sync. |
| [`movie-goat`](library/media-and-entertainment/movie-goat/) | [`/pp-movie-goat`](skills/pp-movie-goat/SKILL.md) | bearer token | full | `/ppl install movie-goat cli` | Compare movie ratings, streaming availability, and recommendations. |
| [`pagliacci-pizza`](library/food-and-dining/pagliacci-pizza/) | [`/pp-pagliacci-pizza`](skills/pp-pagliacci-pizza/SKILL.md) | browser login | partial | `/ppl install pagliacci-pizza cli` | Order Pagliacci and browse public menu and store data without login. |
| [`pokeapi`](library/media-and-entertainment/pokeapi/) | [`/pp-pokeapi`](skills/pp-pokeapi/SKILL.md) | none | full | `/ppl install pokeapi cli` | PokeAPI as an agent-ready knowledge graph plus matchup and team-coverage workflows. |
| [`postman-explore`](library/developer-tools/postman-explore/) | [`/pp-postman-explore`](skills/pp-postman-explore/SKILL.md) | none | full | `/ppl install postman-explore cli` | Search and browse the Postman API Network. |
| [`producthunt`](library/marketing/producthunt/) | [`/pp-producthunt`](skills/pp-producthunt/SKILL.md) | none | full | `/ppl install producthunt cli` | Token-free Product Hunt CLI with local sync and views the website doesn't expose. |
| [`recipe-goat`](library/food-and-dining/recipe-goat/) | [`/pp-recipe-goat`](skills/pp-recipe-goat/SKILL.md) | API key | full | `/ppl install recipe-goat cli` | Find recipes across 37 trusted sites with trust-aware ranking and local cookbook. |
| [`slack`](library/productivity/slack/) | [`/pp-slack`](skills/pp-slack/SKILL.md) | API key | full | `/ppl install slack cli` | Send messages, search conversations, and monitor channels. |
| [`steam-web`](library/media-and-entertainment/steam-web/) | [`/pp-steam-web`](skills/pp-steam-web/SKILL.md) | API key | full | `/ppl install steam-web cli` | Look up Steam players, games, achievements, and stats. |
| [`trigger-dev`](library/developer-tools/trigger-dev/) | [`/pp-trigger-dev`](skills/pp-trigger-dev/SKILL.md) | API key | full | `/ppl install trigger-dev cli` | Monitor runs, trigger tasks, and inspect schedules and failures. |
| [`weather-goat`](library/other/weather-goat/) | [`/pp-weather-goat`](skills/pp-weather-goat/SKILL.md) | none | full | `/ppl install weather-goat cli` | Forecasts, alerts, air quality, and activity verdicts. |
| [`yahoo-finance`](library/commerce/yahoo-finance/) | [`/pp-yahoo-finance`](skills/pp-yahoo-finance/SKILL.md) | none | full | `/ppl install yahoo-finance cli` | Quotes, charts, fundamentals, options, and watchlists. |

## Direct install

You need [Go 1.23+](https://go.dev/dl/).

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

skills/
  ppl/
    SKILL.md
  pp-*/
    SKILL.md                 # generated mirror of library/<.>/SKILL.md

registry.json
```

Each published tool is self-contained: source code, a local README, a `.printing-press.json` provenance manifest, and the manuscripts from the printing run. `skills/pp-*` is a generated mirror of each library `SKILL.md`, produced by `tools/generate-skills/main.go`.

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
