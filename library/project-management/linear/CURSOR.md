# Using linear-pp-cli with Cursor

This guide covers what we have seen work well when replacing (or complementing) the hosted **Linear MCP** server with the **pp-linear** skill and `linear-pp-cli`.

## 1. Get a personal API key (not “Integrations”)

Linear’s GraphQL API uses a **personal API key**:

1. Open Linear → **Settings** (workspace or profile, depending on your layout).
2. Go to **Account → Security & access** (direct link while logged in: [Security & access](https://linear.app/settings/account/security)).
3. Under **Personal API keys**, click **New API key**, name it (for example `Cursor` or `linear-pp-cli`), choose scopes, then copy the value once.

Keys are **not** created from **Integrations → Connected accounts** (Slack, GitHub, etc.); that page is for OAuth links between products.

Official reference: [API and Webhooks](https://linear.app/docs/api-and-webhooks).

## 2. Configure the CLI

**Option A — environment variable (good for CI or disposable shells):**

```bash
export LINEAR_API_KEY="lin_api_..."
```

**Option B — config file (persists on disk):**

```bash
linear-pp-cli auth set-api-key "lin_api_..."
```

Default path: `~/.config/linear-pp-cli/config.toml` (`api_key` field). The separate `auth set-token` command stores an OAuth-style value in `access_token` and is not for personal API keys.

Verify:

```bash
linear-pp-cli doctor
linear-pp-cli me --agent
```

## 3. Cursor: skill vs MCP

- **pp-linear** documents how agents should run `linear-pp-cli` (sync, `issues`, `today`, etc.). Install the skill via Printing Press (`pp-linear`) or copy `SKILL.md` into your project’s agent skills path.
- If you also use Cursor’s **Linear MCP** plugin, you are loading both MCP tool definitions and the CLI skill. Teams sometimes **disable** the Linear MCP entry in the workspace `.cursor/mcp.json` and rely on the CLI only to reduce standing context size. That is optional.

## 4. Security

- Never commit API keys or paste them into chat logs.
- Revoke and rotate a key if it was exposed.
- Prefer **scoped** keys and the minimum access your workflow needs.

## 5. Offline usage

After a successful **`linear-pp-cli sync`**, many read-heavy commands use the local SQLite store. See the main [README](./README.md) for command references.
