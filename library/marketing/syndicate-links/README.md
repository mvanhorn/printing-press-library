# Syndicate Links CLI

> Every API has a secret identity. Syndicate Links isn't an affiliate platform.
> It's an **agentic commerce rail** — the wire that connects merchant agents to
> publisher agents and pays the publisher for the conversion. Every commission
> row is a recorded handshake between two autonomous systems.

Syndicate Links operates the attribution rail underneath agent commerce: a
merchant publishes a program and products, a publisher (human, agent, or
hybrid) creates a tracking link, and when a conversion lands, the rail settles
in either fiat (Stripe — stubbed) or sats (Lightning — live). This CLI is
muscle memory for agents who live on that rail.

> Looking for an MCP server instead? See [Why no MCP here?](#why-no-mcp-here) — Syndicate
> Links ships a hand-tuned MCP server at `syndicate-links-mcp` on npm. This
> repo is the CLI rail.

## Install

The standard Printing Press install handles both the Go binary and the focused
Claude Code skill:

```bash
npx -y @mvanhorn/printing-press install syndicate-links
```

CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install syndicate-links --cli-only
```

Direct Go install (requires Go 1.26.3+):

```bash
go install github.com/mvanhorn/printing-press-library/library/marketing/syndicate-links/cmd/syndicate-links-pp-cli@latest
```

## Authenticate

The CLI honors both `SL_API_KEY` (canonical Syndicate Links name, matches the
hand-tuned MCP server and SDKs) and `SYNDICATE_LINKS_BEARER_AUTH` (printing-press
default). Either works:

```bash
# Merchant key
export SL_API_KEY="mk_live_..."

# Publisher (human) key
export SL_API_KEY="ak_live_..."

# Publisher (agent) key — agent plan required
export SL_API_KEY="aff_agent_..."
```

The key prefix routes the request server-side. `mk_*` keys can only hit
`/merchant/*`; `ak_*` / `aff_agent_*` keys can only hit `/affiliate/*`. The
CLI mirrors that boundary: merchant commands take merchant keys; publisher
commands take publisher keys.

Verify:

```bash
syndicate-links-pp-cli doctor
```

## Three things you can do that the dashboard can't

### 1. Ship a program + products in one round-trip

The dashboard walks you through it screen-by-screen. The CLI ships it in two
calls, idempotent and scriptable.

```bash
syndicate-links-pp-cli merchant create-program \
  --name "Agent Tools" --default-commission-pct 15 --auto-approve \
  --agent | jq -r '.data.id' > /tmp/program-id

syndicate-links-pp-cli merchant bulk-create-products --stdin --agent <<JSON
{
  "programId": "$(cat /tmp/program-id)",
  "products": [
    {"name": "Claude Pro", "url": "https://claude.ai/upgrade", "price": 20.00},
    {"name": "GitHub Copilot", "url": "https://github.com/features/copilot", "price": 10.00}
  ]
}
JSON
```

### 2. Sync the program catalog locally, then run queries the API doesn't expose

```bash
# One-time sync of programs + products + partnerships into a local SQLite db
syndicate-links-pp-cli sync

# Find every product under $20 across every program you have an approved
# partnership in. The /affiliate/products/search endpoint can't do this —
# it doesn't know your partnership status.
syndicate-links-pp-cli analytics --type products --group-by programId --json
```

`--data-source local` reads from the SQLite mirror; `--data-source live` skips
the cache; `--data-source auto` (default) falls back to live on cache miss.

### 3. Bind an agent's checkout to a publisher with a signed attribution token

Three-call flow: register the agent key, create the tracking link, issue the
token. The token is HMAC-signed server-side; the agent presents it at checkout
inside `referrerContext` and the merchant's `POST /merchant/conversions` call
attaches the conversion to the publisher with `attributionMethod=agent_token`.

```bash
# Once, as the publisher: generate an agent key
syndicate-links-pp-cli affiliate create-agent-key --type agent --agent | jq -r .data.key

# Per conversion: mint a short-lived signed token
syndicate-links-pp-cli affiliate attribution-token \
  --program-id "$PROGRAM_ID" --tracking-code "$CODE" --agent
```

## Output

Three formats, switched by context:

```bash
# Terminal: pretty table by default
syndicate-links-pp-cli merchant list-programs

# Pipe: auto-JSON (no --json flag needed)
syndicate-links-pp-cli merchant list-programs | jq .

# CI / agent mode: --agent sets --json --compact --no-input --no-color --yes
syndicate-links-pp-cli merchant list-programs --agent
```

Exit codes: `0` success · `2` usage · `3` not found · `4` auth · `5` API · `7`
rate-limited · `10` config.

## Use with Claude Code

```bash
npx skills add mvanhorn/printing-press-library/cli-skills/pp-syndicate-links -g
```

Then in any Claude Code session:

```
/pp-syndicate-links list my merchant's programs
/pp-syndicate-links create a tracking link for program X destination URL Y
/pp-syndicate-links show pending publisher partnerships for my biggest program
```

The skill drives the CLI directly. No MCP server in the middle — see below for
why.

## Why no MCP here?

Most Printing Press CLIs ship two binaries: `<api>-pp-cli` and `<api>-pp-mcp`.
This one ships only the CLI on purpose. Syndicate Links has a **hand-tuned MCP
server** at [`syndicate-links-mcp`](https://www.npmjs.com/package/syndicate-links-mcp)
maintained alongside the API. It has 7 tools shaped around the agent commerce
use case (program discovery, attribution token issuance, conversion lookup,
balance + payout). The press would have generated 52 thin endpoint mirrors;
two MCP servers for the same API confuses agents and forks maintenance.

If you want the MCP rail:

```bash
npx -y syndicate-links-mcp
```

Or in `~/.config/claude-desktop/config.json`:

```json
{
  "mcpServers": {
    "syndicate-links": {
      "command": "npx",
      "args": ["-y", "syndicate-links-mcp"],
      "env": { "SL_API_KEY": "..." }
    }
  }
}
```

If you want the CLI rail (this repo), you're already in the right place.

## Health check

```bash
syndicate-links-pp-cli doctor
```

Verifies the config file, the auth env var, and that `https://api.syndicatelinks.co`
is reachable. JSON envelope under `--agent` for scripting.

## Configuration

Config: `~/.config/syndicate-links-pp-cli/config.toml`

| Env var | Required | Notes |
| --- | --- | --- |
| `SL_API_KEY` | One of these | Canonical Syndicate Links env var. Prefix routes the request: `mk_live_*` → merchant, `ak_live_*` → human publisher, `aff_agent_*` → agent publisher. |
| `SYNDICATE_LINKS_BEARER_AUTH` | One of these | Press-default env var. Same slot; either name works. |
| `SYNDICATE_LINKS_BASE_URL` | No | Override the base URL (default `https://api.syndicatelinks.co`). Useful for local dev against `http://localhost:3000`. |

## Troubleshooting

**`Auth: not configured`** — Set `SL_API_KEY` and re-run `doctor`. The prefix
must match the command namespace: a `mk_live_*` key on `affiliate ...` returns
`403`.

**`403` on merchant write commands** — Most merchant write paths count against
plan limits (`maxPrograms`, `maxAffiliatesPerProgram`). Check
`syndicate-links-pp-cli merchant billing --agent`.

**`POST /affiliate/payouts/claim` rejects with `400 method must be lightning_invoice`** —
Lightning is the only live payout rail today. Stripe + USDC are planned. Use
a BOLT11 invoice with sat amount within ±2% of the requested USD amount.

**`409 Duplicate click within 60s`** — The publisher agent click endpoint
(`affiliate agent-click`) has a 60-second IP-hash dedup window. Wait or rotate
the `ipHash`.

## Links

- API reference: <https://syndicatelinks.co/docs/api-reference>
- Hand-tuned MCP server: <https://www.npmjs.com/package/syndicate-links-mcp>
- Dashboard: <https://app.syndicatelinks.co>
- Publisher portal: <https://affiliate.syndicatelinks.co>

---

Printed by the [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press).
The Printing Press emits CLI + skill + MCP; for Syndicate Links we kept the
CLI + skill and use our existing hand-tuned MCP server. Distribution is the
library PR — installable via `npx -y @mvanhorn/printing-press install syndicate-links`.
