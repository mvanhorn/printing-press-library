# ▲ / Is Agentic

Measure how ready a public website is for AI agents—directly from the terminal.
The CLI retrieves the latest completed [Is Agentic](https://is-agentic.com)
report and turns its audit evidence into a readable score and prioritized fixes.

No API key is required. Existing reports return immediately; when no completed
report exists, the CLI starts a scan and waits for the canonical result.

## Run it

No installation or API key required:

```sh
npx is-agentic vercel.com
```

```text
▲ / Is Agentic  vercel.com

  ████████████████████████████████░░░░    90 / 100
                                          Strong technical baseline
                                          4 failed · 7 partial

SCORE BREAKDOWN
  Essential       68.9 / 80    10 / 12 passed
  Recommended     15.7 / 20    16 / 25 passed
  Bonus                  +5    36 positive signals

FAILURES (4)

1. FAIL · ESSENTIAL  Agent-friendly 404s
   Evidence  Nonexistent paths return HTTP 200 with the app shell.
   Fix       Return a real HTTP 404 or 410 for nonexistent paths.
```

The full report separates failures from partial checks and wraps each finding
into labeled **Evidence** and **Fix** fields.

## JSON for agents and scripts

Add `--json` or `-j`:

```sh
npx is-agentic vercel.com --json
```

Successful output is the public API response unchanged:

```sh
npx is-agentic vercel.com --json | jq '.score'
```

API failures and local CLI errors are also emitted as structured JSON on
stdout. Errors use a nonzero exit status, so automation never needs to parse
human-facing text.

## Command reference

| Command | Output |
| --- | --- |
| `npx is-agentic <domain-or-url>` | Human-readable terminal report |
| `npx is-agentic <domain-or-url> --json` | Structured JSON |
| `npx is-agentic --help` | Usage and available options |

Set `NO_COLOR=1` to disable ANSI color. Node.js 18 or newer is required.

## How report lookup works

- The CLI requests the latest **completed** stored report first.
- If no report exists, it starts one scan and follows its progress to storage.
- It never forces a rescan when a completed report is already available.
- Scans only observe the target's public website; they do not mutate it.
- Success exits with `0`; usage, API, and network errors exit nonzero.

## Learn more

[Run a scan](https://is-agentic.com) ·
[Read the docs](https://is-agentic.com/docs) ·
[View the OpenAPI description](https://is-agentic.com/openapi.json)
