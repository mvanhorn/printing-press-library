# Fox News RSS CLI — brief

## Goal

Agent-native read-only CLI for Fox News headlines via public Google Publisher RSS feeds.

## Data source

Base: `https://moxie.foxnews.com/google-publisher/`

| Section | Path |
|---------|------|
| latest (default) | latest.xml |
| world | world.xml |
| politics | politics.xml |
| science | science.xml |
| health | health.xml |
| sports | sports.xml |
| travel | travel.xml |
| tech | tech.xml |
| opinion | opinion.xml |
| video | videos.xml |

## Scope

- `headlines --section <id> --limit N --json`
- `sections` to list ids
- `doctor` probes latest feed
- No auth, no SQLite, no MCP in v1
