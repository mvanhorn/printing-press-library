# Fox News (`foxnews-pp-cli`)

Read Fox News headlines from the public **Google Publisher RSS** feeds on `moxie.foxnews.com`. No API key, no scraping — one HTTP GET per request.

## Install

```bash
npx -y @mvanhorn/printing-press install foxnews --cli-only
```

Or from this directory:

```bash
go install ./cmd/foxnews-pp-cli
```

Verify: `foxnews-pp-cli --version`

## Quick start

```bash
# Latest headlines (all sections) — default
foxnews-pp-cli headlines --limit 10 --json

# Politics only
foxnews-pp-cli headlines --section politics --limit 5 --json

# List section ids
foxnews-pp-cli sections --json

# Health check
foxnews-pp-cli doctor --json
```

## Sections

| `--section` | Feed |
|-------------|------|
| `latest` (default) | `latest.xml` |
| `world` | `world.xml` |
| `politics` | `politics.xml` |
| `science` | `science.xml` |
| `health` | `health.xml` |
| `sports` | `sports.xml` |
| `travel` | `travel.xml` |
| `tech` | `tech.xml` |
| `opinion` | `opinion.xml` |
| `video` | `videos.xml` |

Alias: `videos` maps to `video`.

## Configuration

| Variable | Purpose |
|----------|---------|
| `FOX_NEWS_FEED_BASE` | Optional RSS base URL (default `https://moxie.foxnews.com/google-publisher`) |

## Output

- **Piped** (scripts/agents): JSON by default, wrapped as `{"meta":{...},"results":[...]}`
- **Terminal**: table by default; add `--json` or `--agent` for JSON

`--agent` sets `--json --compact --no-input --no-color --yes`. Compact keeps `title`, `link`, `published`, `section` on headlines.

```bash
foxnews-pp-cli headlines --section sports --limit 15 --agent
foxnews-pp-cli agent-context --pretty
```

This CLI does **not** ship Drudge-style `sync`, `splash`, `breaking`, or `which` — use `sections`, `headlines`, and `agent-context` instead.

## License

Apache-2.0. See [LICENSE](LICENSE) and [NOTICE](NOTICE).
