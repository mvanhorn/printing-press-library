---
name: pp-steam-web
description: "Steam player and game lookup via the Steam Web API. Look up player profiles, owned games, recent playtime, achievements, stats, badges, friend lists, VAC/game ban status, and game schemas. Use when the user asks about their Steam library, a friend's achievements, who's playing a game, compare two players' stats, a player's Steam level or badges, VAC status, or wants to resolve a vanity URL to a Steam ID."
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - steam-web-pp-cli
      env:
        - STEAM_WEB_API_KEY
    primaryEnv: STEAM_WEB_API_KEY
    install:
      - kind: go
        bins: [steam-web-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/media-and-entertainment/steam-web/cmd/steam-web-pp-cli
---

# Steam Web - Printing Press CLI

Look up Steam players, games, achievements, friends, and stats from the command line. The CLI wraps 170+ Steam Web API endpoints and pairs them with a local SQLite sync so repeat queries about the same library don't burn rate limit.

## When to Use This CLI

Reach for this when the user wants:

- look up a player's profile, level, or badges (`profile`, `level`, `badges`)
- list a player's owned games with playtime (`games`)
- show recently-played games (`recent`)
- pull a player's achievements or stats for a specific game (`achievements`, `stats`)
- check a player's VAC or game ban status (`bans`)
- list a player's friends (`friends`)
- see how many players are currently in a game (`players`)
- fetch a game's news feed (`news`)
- resolve a vanity URL (steamcommunity.com/id/foo) to a SteamID (`resolve`)
- dump a game's achievement + stat schema (`schema`)
- hit any of the 170+ Steam Web API endpoints by interface name (`api`)

Skip it when the user wants to buy games, manage Steam Workshop mods, or interact with the Steam store beyond what the Web API exposes.

## Argument Parsing

Parse `$ARGUMENTS`:

1. **Empty, `help`, or `--help`** -> show `steam-web-pp-cli --help`
2. **Starts with `install`** -> ends with `mcp` -> MCP installation; otherwise -> CLI installation
3. **Anything else** -> Direct Use (map to the best command and run it)

## CLI Installation

1. Check Go is installed: `go version` (requires Go 1.23+).
2. Install:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/steam-web/cmd/steam-web-pp-cli@latest
   ```

   If `@latest` installs a stale build (Go module proxy cache lag):
   ```bash
   GOPRIVATE='github.com/mvanhorn/*' GOFLAGS=-mod=mod \
     go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/steam-web/cmd/steam-web-pp-cli@main
   ```
3. Verify: `steam-web-pp-cli --version`.
4. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.
5. Auth setup:
   ```bash
   export STEAM_WEB_API_KEY="..."
   ```
   Request a key at https://steamcommunity.com/dev/apikey (free; requires a Steam account).
6. Verify: `steam-web-pp-cli doctor` reports key status.

## MCP Server Installation

The CLI ships an MCP server at `steam-web-pp-mcp`:

```bash
go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/steam-web/cmd/steam-web-pp-mcp@latest
claude mcp add -e STEAM_WEB_API_KEY=... steam-web-pp-mcp -- steam-web-pp-mcp
```

## Direct Use

1. Check installed: `which steam-web-pp-cli`. If missing, offer CLI installation.
2. A SteamID (17-digit) or vanity URL is needed for player commands. If the user gives a vanity URL, run `resolve <vanity>` first to get the SteamID.
3. Discover commands: `steam-web-pp-cli --help`; drill into `steam-web-pp-cli <cmd> --help`.
4. Execute with `--agent` for structured output:
   ```bash
   steam-web-pp-cli <command> [args] --agent
   ```

## Notable Commands

| Command | What it does |
|---------|--------------|
| `profile <steamid>` | Player profile summary (name, country, avatar, visibility) |
| `games <steamid>` | Owned games with total playtime |
| `recent <steamid>` | Recently played games (last 2 weeks) |
| `achievements <steamid> <appid>` | Player's achievements for a game |
| `stats <steamid> <appid>` | Player's stats for a game |
| `friends <steamid>` | Friend list with relationship timestamps |
| `level <steamid>` | Steam level |
| `badges <steamid>` | All badges earned |
| `bans <steamid>` | VAC and game-ban status |
| `players <appid>` | Current-player count for a game |
| `news <appid>` | News articles for a game |
| `schema <appid>` | Stat + achievement schema for a game |
| `resolve <vanity>` | Vanity URL -> SteamID |
| `api <interface> <method>` | Call any of 170+ Web API endpoints directly |

Run any command with `--help` for full flag documentation.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields, with dotted-path support (see below)
- **Previewable** — `--dry-run` shows the request without sending
- **Cacheable** — GET responses cached for 5 minutes, bypass with `--no-cache`
- **Non-interactive** — never prompts, every input is a flag


### Filtering output

`--select` accepts dotted paths to descend into nested responses; arrays traverse element-wise:

```bash
steam-web-pp-cli <command> --agent --select id,name
steam-web-pp-cli <command> --agent --select items.id,items.owner.name
```

Use this to narrow huge payloads to the fields you actually need — critical for deeply nested API responses.


### Response envelope

Data-layer commands wrap output in `{"meta": {...}, "results": <data>}`. Parse `.results` for data and `.meta.source` to know whether it's `live` or local. The `N results (live)` summary is printed to stderr only when stdout is a TTY; piped/agent consumers see pure JSON on stdout.


## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 2 | Usage error (wrong arguments) |
| 3 | Resource not found (player / game) |
| 4 | Authentication required (STEAM_WEB_API_KEY missing or invalid) |
| 5 | API error (Steam upstream; includes private-profile errors) |
| 7 | Rate limited |
