# Using gorgias-pp-cli with Cursor

Cursor can drive `gorgias-pp-cli` through its MCP support. The recommended
setup gives Cursor agents read access to Gorgias plus the local SQLite
mirror, with a code-orchestration gateway that keeps context cost flat
regardless of API surface size.

## One-time setup

1. **Get a Gorgias API key.** In Gorgias: Settings → Account → REST API →
   Create API Key. Your username is your account email; your domain is
   what's in your Gorgias URL (`<domain>.gorgias.com`).

2. **Install the CLI.** Install through Printing Press:

   ```bash
   npx -y @mvanhorn/printing-press-library install gorgias --cli-only
   ```

3. **Provide credentials.** Set three environment variables however your
   workflow prefers (shell profile, a secrets manager, a CI secret store —
   the CLI doesn't care):

   ```bash
   export GORGIAS_BASE_URL=https://<tenant>.gorgias.com/api
   export GORGIAS_USERNAME=account-email-placeholder
   export GORGIAS_API_KEY=<your-api-key>
   ```

4. **Verify before wiring into Cursor.**

   ```bash
   gorgias-pp-cli doctor --json
   ```

   Expect `"credentials": "valid"` and `"env_vars": "OK 2/2 available"`.

## Wire into Cursor

Add the MCP server to `.cursor/mcp.json` at your repo root:

```json
{
  "mcpServers": {
    "gorgias": {
      "command": "gorgias-pp-mcp",
      "env": {
        "GORGIAS_BASE_URL": "https://<tenant>.gorgias.com/api",
        "GORGIAS_USERNAME": "account-email-placeholder",
        "GORGIAS_API_KEY": "(your key)"
      }
    }
  }
}
```

If you'd rather not put the key in `mcp.json`, set the three env vars in
your shell before launching Cursor and drop the `env` block.

Restart Cursor. The Gorgias MCP server now exposes a small fixed set of tools (live count reported by the `context` tool's `tool_count` field — no hardcoded inventory).

## What you get inside Cursor

The Cursor agent can call these tools without you typing commands:

- **`gorgias_search`** — natural-language search for the right endpoint.
- **`gorgias_execute`** — call any endpoint by ID (`tickets.list`,
  `customers.get`, etc.). Discover IDs via `gorgias_search` first.
- **`workflow_archive`** / **`workflow_status`** — bulk sync all
  resources, inspect mirror state.
- **`sync`** / **`search`** / **`sql`** / **`analytics`** /
  **`orphans`** / **`stale`** / **`load`** — local SQLite mirror queries.
- **`tail`** — stream live ticket changes.
- **`export`** / **`import`** — JSONL bulk movement.
- **`context`** — full CLI descriptor (auth, novel features, schema).

Example Cursor prompts:

> "How many tickets did we get yesterday from Shopify customers?"
>
> Cursor → `sync(resources: "tickets", since: "1d")` →
> `sql("SELECT count(*) FROM resources WHERE resource_type = 'tickets' AND json_extract(data, '$.customer.meta.shopify_id') IS NOT NULL")`.

## Token budget

The 15-tool MCP surface measures ~1K tokens of pure description text
and ~7K tokens of JSON schemas (~9K total in a live `tools/list`
response), regardless of the underlying 108-endpoint API size. The
gateway pattern (`gorgias_search` → `gorgias_execute`) means the full
Gorgias surface is reachable without each endpoint costing tokens for
its own typed tool — relevant if you're running Cursor with multiple
MCP servers active.

## Offline / disconnected usage

After a `sync`, every read except `tail` works offline against
`~/.local/share/gorgias-pp-cli/data.db`:

```bash
gorgias-pp-cli sync --resources tickets,customers,tags,macros --since 30d
# Now disconnect.
gorgias-pp-cli tickets list --data-source local
gorgias-pp-cli search refund --data-source local
```

Set `GORGIAS_AUTO_REFRESH_TTL=15m` and every read will sync first if the
mirror is older than the TTL — useful for "I trust local but want
freshness" workflows.

## Rotating the API key

Generate a new key in Gorgias (Settings → Account → REST API), update the
`GORGIAS_API_KEY` env var wherever you set it, and run
`gorgias-pp-cli doctor` to confirm. No restart needed for the CLI side.
Cursor's MCP server picks up the new value on the next tool call when the
env var is sourced at invocation time.

## See also

- [README.md](README.md) — full feature list, troubleshooting
- [SKILL.md](SKILL.md) — Claude Code skill descriptor (parallel to this
  file but for Claude Code)
