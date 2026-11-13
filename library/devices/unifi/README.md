# UniFi CLI

**Operate UniFi consoles through the cloud connector or your LAN, with a local history, change previews and a change journal.**

One binary for the official UniFi APIs, over the cloud connector or a direct connection to your gateway. A local mirror keeps a snapshot from each sync, and 'impact' estimates what a change would affect before you make it. Reads have run against one test console over the cloud connector; most writes have not.

Created by [@phoenix-server](https://github.com/phoenix-server) (Ricardo Cabral).

Contributors: [@amansk](https://github.com/amansk) (Amandeep Khurana), [@cathrynlavery](https://github.com/cathrynlavery) (Cathryn Lavery), [@officialasishkumar](https://github.com/officialasishkumar) (Asish Kumar).

## Install

The recommended path installs both the `unifi-pp-cli` binary and the `pp-unifi` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install unifi
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install unifi --cli-only
```

For skill only: installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install unifi --skill-only
```

To constrain the skill install to one or more specific agents (repeatable: agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install unifi --agent claude-code
npx -y @mvanhorn/printing-press-library install unifi --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.11 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/devices/unifi/cmd/unifi-pp-cli@latest
```

This installs the CLI only: no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/unifi-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install unifi --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-unifi --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-unifi --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install unifi --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle: Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/unifi-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `UNIFI_API_KEY` when Claude Desktop prompts you. The bundle asks only for the cloud key. For local gateway access, use the manual config below.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/devices/unifi/cmd/unifi-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json` on macOS, `%APPDATA%\Claude\claude_desktop_config.json` on Windows):

```json
{
  "mcpServers": {
    "unifi": {
      "command": "unifi-pp-mcp",
      "env": {
        "UNIFI_API_KEY": "<your-key>"
      }
    }
  }
}
```

For the local gateway, add `UNIFI_GATEWAY` and `UNIFI_LOCAL_API_KEY` to `env`. To allow reads only, add `"UNIFI_READ_ONLY": "1"`.

</details>

### Upgrading from an earlier release

API commands now sit under their product. Old names keep working and print a note on stderr saying where they moved, for example `sites clients get-connected-overview-page` runs `network list-clients`. Arguments and flags are unchanged.

`port-audit` and `guest report` were removed, and now fail with a message naming the commands that replace them: `network list-devices` and `network get-device`, and `network list-hotspot-vouchers` and `network list-clients`.

Existing LAN settings still work: `UNIFI_GATEWAY_HOST`, and a saved `base_url` ending in `/proxy/network`.

### Remote MCP over HTTP

`unifi-pp-mcp --transport` chooses `stdio` or `http`. It is not the CLI's `--transport`: the MCP server takes its route from `UNIFI_GATEWAY`, `UNIFI_CONSOLE_ID` and `UNIFI_TRANSPORT` in its environment.

```bash
export UNIFI_MCP_HTTP_TOKEN="$(openssl rand -hex 32)"
unifi-pp-mcp --transport http
```

This serves streamable HTTP at `http://127.0.0.1:7777/mcp`. `PP_MCP_TRANSPORT=http` does the same as the flag.

**What the server enforces:**

- It refuses to start unless `UNIFI_MCP_HTTP_TOKEN` has at least 32 characters. The command above prints 64.
- Every request must send `Authorization: Bearer <token>`, or it gets a 401.
- Plain HTTP is allowed only on a loopback `--addr`. Any other bind, including `:7777` (every interface), refuses to start without `--tls-cert` and `--tls-key`.
- A loopback `Origin` is accepted. Any other `Origin` is refused unless listed in `UNIFI_MCP_HTTP_ALLOWED_ORIGINS`, a comma-separated list of `scheme://host[:port]` values. Requests with no `Origin` header are accepted, so this stops browsers, not other clients. The list is not a CORS policy; the server emits no CORS headers.
- Request bodies are capped at 4 MiB.

**What it does not do:**

- It has one shared token: no per-caller identity, scopes or expiry. Changing the token means restarting the server. The audit log cannot say which caller did what.
- It has no rate limit or lockout on failed attempts.
- It is not read-only by default. A token holder gets every tool with the authority of the server's keys, including writes. `confirm: true` on `unifi_execute`, `raw` and `change apply` is an argument the caller sets. It separates a preview from a send. It is not a human approval.
- **MCP `sql` and `search` read the local mirror, which holds no secret values.** Passphrases, voucher codes and stream URLs are stored as salted digests such as `[secret:v1:...]`. `unifi_read` and `raw` still return whatever the live API sends, including those values if a console returns them.
- MCP callers cannot choose the route. `gateway` and `transport` are not offered and are refused, and they stay operator settings.

**Risk on a non-loopback address:** anyone who can reach the port can try tokens without limit, and anyone who gets the token can read network and client data and Protect streams and snapshots, and can write wherever the keys allow. TLS protects the token in transit and nothing else.

**If you expose it:**

- Prefer loopback with an SSH tunnel or VPN.
- Otherwise restrict the port to known addresses with a firewall.
- Set `UNIFI_MCP_READ_ONLY=1` (any value except empty, `0`, `false`, `no` or `off` turns it on), which drops tools that are not marked read-only and refuses state-changing calls, and `UNIFI_READ_ONLY=1`. Commands marked local-write, which write only this CLI's own learn store, stay available.
- Set `UNIFI_CONSOLE_ID`. MCP callers then cannot target another console unless it is listed in `UNIFI_CONSOLE_ALLOWLIST`. Site Manager reads and writes stay account-wide. Over MCP, `reach --probe-writes` also needs `confirm` set to boolean true.
- Do not set `UNIFI_LOCAL_API_KEY` on a server other people can call.

## Authentication

Cloud: create a key at unifi.ui.com under Settings > API Keys and set UNIFI_API_KEY. Local: create a key in the Network app under Settings > Control Plane > Integrations, set UNIFI_LOCAL_API_KEY, and pass --gateway or set UNIFI_GATEWAY. The keys are separate, and a cloud key is never sent to a gateway. Run 'reach' to see what answers.

## Cloud or local: choosing a transport

Network, Protect and InnerSpace commands run on a console, so they need a route. There are two:

| | Cloud connector | Local gateway |
| --- | --- | --- |
| Route | `api.ui.com/v1/connector/consoles/<id>/proxy/...` | `https://<gateway>/proxy/...` |
| Key | `UNIFI_API_KEY` | `UNIFI_LOCAL_API_KEY` |
| Where the key comes from | unifi.ui.com, Settings > API Keys | Network app, Settings > Control Plane > Integrations |
| Select it | `--console`, `UNIFI_CONSOLE_ID` or `console_id` in config | `--gateway`, `UNIFI_GATEWAY` or `gateway` in config |
| Needs | Console firmware 5.0.3 or newer (per the spec) | This machine can reach the gateway (LAN or VPN) |
| Documented limits | 100 requests per minute per console, 25 seconds per request, 10 MB per response | None documented |
| Writes | Docs say cloud keys are read-only today, but one voucher create and delete worked on one test console; other cloud writes are unproven | Should work with a local key; untested live |

**How to choose:**

- Off-site, several consoles, or read-only work: use the cloud connector.
- You need to write and can reach the gateway: use the local gateway. Do not plan on cloud writes until `reach` or a verified `change apply` shows they work.
- You have both: set both and use `--transport` to choose per command.

**How the route is resolved:**

1. Flags beat env, and env beats `config.toml`.
2. A gateway is used whenever one is set and `--transport` is not `cloud`, even if a console is also set. `auto` behaves the same as leaving it unset.
3. `--transport local` with no gateway fails with exit 10. `--transport cloud` ignores any gateway.
4. With no gateway and no console, the CLI lists `/v1/hosts`. One console is used automatically. Several are an error (exit 10) that lists the ids, because it will not guess.

**Some commands always use the cloud.** `hosts`, `devices`, `sites`, `isp-metrics`, `sd-wan-configs`, `carrier`, `mobility` and `fleet` call `api.ui.com` with `UNIFI_API_KEY`, even with `--gateway` set. They have no local route. With only a local key they cannot succeed, and `sync` reports those resources as failed and carries on unless you pass `--strict`.

**Keys stay separate.** A cloud key is never sent to a gateway: without `UNIFI_LOCAL_API_KEY`, a local request fails with exit 4. Non-organization cloud keys reach only the key owner's consoles; organization keys reach every console in the org.

**Gateway address.** A bare host, `host:port` or URL, with no path (the CLI adds `/proxy` itself). It must be `https`; only loopback may use plain `http`. `gateway` and `local_api_key` in a `config.toml` that other users can edit are ignored.

**Certificates.** Gateways usually serve a self-signed certificate, so verification is skipped by default for literal private, loopback and link-local addresses such as `192.168.1.1`. DNS names and public addresses are verified. `UNIFI_INSECURE_SKIP_VERIFY` overrides both ways: `1`, `true`, `yes` or `on` skips verification (with a warning for non-local hosts); `0`, `false`, `no` or `off` always verifies. For a verified local connection, use `--ca-cert` and `--tls-server-name`.

**Connector path shape.** The published spec contradicts itself on whether connector paths include `/proxy`. The CLI uses the `/proxy` form. `reach` tries both and reports which one answers.

## What is not proven or not covered

- **Cloud writes:** docs say cloud keys are read-only today, while the connector documents write verbs. One hotspot voucher create and delete worked with a cloud key on one test console. Other cloud writes are unproven.
- **Local writes:** designed to work with a local key, untested live.
- **`raw` controller paths:** unofficial and unverified. Ubiquiti does not document them, and whether a key can reach them depends on the console.
- **`change apply` over MCP:** callers must pass `--change-json`, because file paths are refused.
- **`change apply --trial` expiry:** it relies on the gateway stopping a `ONE_TIME_ONLY` schedule. That has not been verified, so check the policy yourself after the window. Trials cannot cross midnight, and times must be console-local (`--trial-timezone`).
- **`rule-predict`:** a prediction with no API confirmation. It returns `indeterminate` for domain, application, region and scheduled filters, and legacy firewall rules are outside the model.
- **`impact`:** reasons from the last sync and never reports safe. It cannot see every cause of path loss.
- **UniFi Access and Talk:** not covered. No official spec and no commands.
- **Blocking a client:** no official endpoint. The API's client actions are guest authorize and unauthorize. Blocking needs an unverified `raw` path or a firewall or ACL rule.
- **Protect WebSocket subscribe endpoints:** no commands. `tail` polls instead.

## Safety controls

- **Most writes send immediately.** `network`, `protect`, `carrier` and `mobility` write commands and `import` have no `--confirm` step or prompt. Use `--dry-run` first. Only `raw` and `change apply`/`change undo` print a preview and wait for `--confirm`. `--yes` applies only to local commands such as `local-data purge` and `profile delete`.
- **`UNIFI_READ_ONLY`** (any value except empty, `0`, `false`, `no` or `off`) refuses every mutating API request from the CLI and from the MCP server, with exit 10. `--dry-run` still works.
- **`UNIFI_CONSOLE_ALLOWLIST`** is a comma-separated list of console ids. Connector requests to any other console are refused with exit 10. It does not limit Site Manager reads such as `hosts` and `devices`, which still list every console the key can see, and it does not limit the local gateway route. `fleet` shows a console outside the list as `connector=failed`. A named console is also checked on the local route, and local-only commands refuse a `--console` outside the list (exit 10). A configured gateway with no console named stays outside the allowlist, and so do Site Manager commands and rows already in the mirror. `fleet --console` with a console outside the list exits 10; an unfiltered `fleet` still shows such consoles as `connector=failed`.
- **`unifi-pp-cli audit`** reads the local log of attempted writes, including refused and ambiguous ones. Secret-named fields are scrubbed. Reads, previews and dry runs are not logged, and entries do not record who called.
- **Ambiguous writes are not replayed.** After a timeout, a 408 or a 5xx the write may have applied, so the CLI reports it and does not send it again.

## Quick Start

```bash
# Confirm the binary runs. Sends no requests.
unifi-pp-cli doctor --dry-run

# See which consoles, products and transports answer. Needs a key.
unifi-pp-cli reach --agent

# Build the local mirror and take a snapshot. History features need it.
unifi-pp-cli sync

# Read the timeline. Changes show up once there is more than one sync.
unifi-pp-cli digest --since 7d --agent

```

## Unique Features

### Safe remote operation
- **`reach`** — See which transports and products answer on each console. Write status is yes, no or unproven, and a probe alone never reports yes.

  _Run this first in a session to learn what answers before planning a change. Spends one read per surface per console._

  ```bash
  unifi-pp-cli reach --agent
  ```
- **`impact`** — Estimate from the last sync what a proposed action would take down and whether it risks your way back in. It never reports safe.

  _Use before any change that could cut the path needed to undo it. Verdicts are blocks-path, at-risk, no-known-risk or cannot-determine, and it cannot see every cause of path loss._

  ```bash
  unifi-pp-cli impact device restart gateway --agent
  ```
- **`change apply`** — Send one change to an official Network or Protect path with a journaled before-image, a read-back check and undo. Nothing is sent without --confirm.

  _Use instead of a plain write when the change must be checked and revertible. Docs say cloud keys are read-only today, other cloud writes are unproven beyond one voucher create and delete, and --trial expiry is unverified._

  ```bash
  unifi-pp-cli change apply firewall-policy.json --trial --site 11111111-2222-3333-4444-555555555555 --agent
  ```

### Local history
- **`drift`** — See field-level config changes between two synced snapshots, marked attributed (in the change journal) or out-of-band.

  _Use to answer what changed in config and whether it went through change apply. Needs at least two syncs._

  ```bash
  unifi-pp-cli drift --agent
  ```
- **`digest`** — One timeline, built from local syncs, of state changes, ISP dips, drift, newcomers and journaled changes.

  _Use for a catch-up summary instead of querying each product. A flap between two syncs is missed._

  ```bash
  unifi-pp-cli digest --since 7d --agent
  ```
- **`newcomer`** — List clients, devices and cameras first seen at a sync in a window.

  _Use to spot unfamiliar hardware. Randomized MACs can inflate the list, and a visit between syncs is missed._

  ```bash
  unifi-pp-cli newcomer --since 24h --agent
  ```

### Understand your network
- **`rule-predict`** — Predict which firewall policy would match a hypothetical packet. This is a local prediction with no API confirmation.

  _Use before editing rules. Treat indeterminate as unknown; legacy firewall rules are outside the model._

  ```bash
  unifi-pp-cli rule-predict --src-zone LAN --dst-zone External --dst-port 443 --protocol tcp --agent
  ```
- **`topology`** — See gateway, switches and APs as a tree per console, from the last sync.

  _Use to orient on the physical layout. Uplinks come from per-device detail requests, so sync --no-hydrate leaves the tree flat._

  ```bash
  unifi-pp-cli topology --agent
  ```

## Cookbook

### Check before changing

```bash
unifi-pp-cli impact device restart gateway --agent
```

Reports affected clients and a control-path verdict from the last sync. Run `sync` first, or the verdict is `cannot-determine`.

### Restart an access point

```bash
unifi-pp-cli network list-sites --agent
unifi-pp-cli network list-devices 11111111-2222-3333-4444-555555555555 --agent --select data.id,data.name,data.model,data.state
unifi-pp-cli network run-device-action 11111111-2222-3333-4444-555555555555 22222222-3333-4444-5555-666666666666 --action RESTART --dry-run --agent
```

The last command previews. Without `--dry-run` it sends immediately, and a restart cannot be undone. It needs a key that can write, so it will probably need the local route.

### Narrow device output

```bash
unifi-pp-cli network list-devices 11111111-2222-3333-4444-555555555555 --agent --select data.name,data.model,data.state
```

Keeps large device payloads small for agents. Get the site id from `network list-sites`.

### Attribute changes

```bash
unifi-pp-cli drift --agent
```

Marks each change `attributed` when the change journal holds it, otherwise `out-of-band`. Writes made outside `change apply` count as `out-of-band`. Needs at least two syncs.

### Test a local route

```bash
unifi-pp-cli reach --gateway 192.168.1.1 --json
```

Probes the gateway with `UNIFI_LOCAL_API_KEY`. Works without a cloud key.

### Find out whether writes are refused

```bash
unifi-pp-cli reach --console EXAMPLE-CONSOLE-ID --probe-writes --json
```

Sends a `DELETE` for an all-zero id, which matches nothing. `write_status` is `no` on a 401, 403 or 405, and `unproven` on anything else. It becomes `yes` only after a verified `change apply` on that console and transport.

### Preview, apply and undo a journaled change

```bash
unifi-pp-cli change apply --change-json '{"method":"PATCH","path":"/network/integration/v1/sites/SITE/firewall/policies/POLICY","body":{"enabled":false}}' --json
unifi-pp-cli change apply disable-guest-wifi.json --confirm --json --gateway 192.168.1.1
unifi-pp-cli change list --since 7d --json
unifi-pp-cli change undo chg_0123456789abcdef --json
```

The first and last commands print a plan and send nothing. Add `--confirm` to `change undo` to send it.

### Call an unofficial controller path

```bash
unifi-pp-cli raw network/api/s/default/stat/health --gateway 192.168.1.1 --json
```

Unofficial and unverified. Write methods need `--confirm`, and a `raw` write has no journal or undo.

### Check every console

```bash
unifi-pp-cli fleet health --json
unifi-pp-cli fleet firmware --json --select firmware.console_id,firmware.update_available
```

Exits 0 even when some consoles fail, so read `fetch_failures`.

### Review what was tried

```bash
unifi-pp-cli audit --outcome ambiguous --agent
```

An `ambiguous` write may or may not have applied. Read the object before trying again.

### Sync with a limited connector budget

```bash
unifi-pp-cli sync --console EXAMPLE-CONSOLE-ID
```

Each device detail is one request against the connector's documented 100 per minute. Add `--max-hydrate` with a lower number (default 100) to cap them, or `--no-hydrate` to skip them, which leaves `topology` flat and limits what `impact` can say.

## Recipes

### Check before changing

```bash
unifi-pp-cli impact device restart gateway --agent
```

Reports affected clients and a control-path verdict from the last sync. Run sync first, or the verdict is cannot-determine.

### Narrow device output

```bash
unifi-pp-cli network list-devices 11111111-2222-3333-4444-555555555555 --agent --select data.name,data.model,data.state
```

Keeps large device payloads small for agents. Get the site id from network list-sites.

### Attribute changes

```bash
unifi-pp-cli drift --agent
```

Marks each change attributed when the change journal holds it, otherwise out-of-band. Writes made outside change apply count as out-of-band.

### Test a local route

```bash
unifi-pp-cli reach --gateway 192.168.1.1 --json
```

Probes the gateway with UNIFI_LOCAL_API_KEY. Works without a cloud key.

### Preview a journaled change

```bash
unifi-pp-cli change apply --change-json '{"method":"PATCH","path":"/network/integration/v1/sites/SITE/firewall/policies/POLICY","body":{"enabled":false}}' --json
```

Prints the plan and sends nothing. Add --confirm to send it.

## Usage

Run `unifi-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data: `credentials.toml`, `data.db`, cookies, browser-session proof files, and other auth sidecars |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `UNIFI_CONFIG_DIR`, `UNIFI_DATA_DIR`, `UNIFI_STATE_DIR`, or `UNIFI_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `UNIFI_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export UNIFI_HOME=/srv/unifi
unifi-pp-cli doctor
```

Under `UNIFI_HOME=/srv/unifi`, the four dirs resolve to `/srv/unifi/config`, `/srv/unifi/data`, `/srv/unifi/state`, and `/srv/unifi/cache`.

On Windows, set the same variable in PowerShell:

```powershell
$env:UNIFI_HOME = "C:\srv\unifi"
unifi-pp-cli doctor
```

Under `$env:UNIFI_HOME = "C:\srv\unifi"`, the four dirs resolve to `C:\srv\unifi\config`, `C:\srv\unifi\data`, `C:\srv\unifi\state`, and `C:\srv\unifi\cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "unifi": {
      "command": "unifi-pp-mcp",
      "env": {
        "UNIFI_HOME": "/srv/unifi"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `UNIFI_DATA_DIR` overrides an explicit `--home` for that kind. Use `UNIFI_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `UNIFI_HOME` does not move files back to platform defaults, and `doctor` cannot find credentials left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. On the first auth write, stored secrets leave `config.toml` and are consolidated into `credentials.toml` under the data directory. Run `unifi-pp-cli doctor --fail-on warn` to check path and credential-location warnings in automation.

### Local data

Reads are saved locally by default: in `--data-source auto` mode every successful read is also written to the SQLite store (`data.db`, or `data-<hash>.db` per credential) in the data directory. GET responses are cached for five minutes under `http/` in the cache directory. Credentials live in `credentials.toml` in the data directory. The learn journal lives under `learn/` in the state directory. `unifi-pp-cli doctor` prints the resolved paths, every store file, and the response-cache size.

None of it is encrypted. New files are created owner-only; beyond that the data relies on OS account permissions and full-disk encryption (FileVault, BitLocker, LUKS).

- **Erase it:** `unifi-pp-cli local-data purge` lists the store files, response cache, and learn journal it would delete; add `--yes` to delete them, or `--only store|cache|journal` to narrow it. Config and credentials are kept.
- **Run without a local copy:** set `UNIFI_NO_LOCAL_DATA=1`. Reads go to the API (`--data-source live`; `--data-source local` still reads an existing store), the response cache is off (`--no-cache`), nothing new is written to the store, and `sync` and `workflow archive` refuse to run. Set it in an MCP host's `env` block and every command the server runs inherits it.
- **Choose the read source without flags:** `UNIFI_DATA_SOURCE=auto|live|local` sets the `--data-source` default; an explicit flag wins.

## Commands

### carrier

Manage carrier

- **`unifi-pp-cli carrier assign-subscriber-plan`** - Assigns the subscriber's service plan. Assigning an archived plan returns `400 service_plan_archived`; an unknown plan returns `404 service_plan_not_found`. Requires scope `assign:plans`.
- **`unifi-pp-cli carrier attach-subscriber-host`** - Attaches (or re-links) the subscriber's gateway host. A subscriber has at most one host; supplying a DIFFERENT host replaces the current one in place (RMA re-link).

**Not idempotent.** Re-sending the host this subscriber is already linked to is rejected with `409 gateway_already_attached`: nothing is written. A host linked to a *different* subscriber in the organization is `409 gateway_already_linked`. Read `GET /subscribers/{id}` (field `hostId`) if you need to know the current link before calling; `prevHostId` on a successful response is the host that was replaced (`null` on a fresh attach).

Because a repeat is an error rather than a no-op, do NOT blindly retry a request that timed out: re-read the subscriber first, or treat `409 gateway_already_attached` as "already applied".

Requires scope `update:subscribers`.
- **`unifi-pp-cli carrier create-subscriber`** - Creates a subscriber (identity, optional service address, and optionally a plan; attach a gateway host separately). Always returns `201`: there is no external-reference deduplication. `subscriberNumber` is the only required field. Requires scope `create:subscribers`.
- **`unifi-pp-cli carrier detach-subscriber-host`** - Detaches the subscriber's gateway host.

**Not idempotent.** A subscriber with no linked host is rejected with `409 no_attached_host`: nothing is written. On success `prevHostId` is the host that was removed.

Because a repeat is an error rather than a no-op, do NOT blindly retry a request that timed out: re-read the subscriber first (`GET /subscribers/{id}`, field `hostId`), or treat `409 no_attached_host` as "already applied".

Requires scope `update:subscribers`.
- **`unifi-pp-cli carrier get-service-plan`** - Requires scope `read:plans`. Returns 400 if `id` is not a valid UUID.
- **`unifi-pp-cli carrier get-subscriber`** - Requires scope `read:subscribers`. Returns 404 for an unknown or out-of-scope subscriber, or 400 if `id` is not a valid UUID.
- **`unifi-pp-cli carrier list-service-plans`** - Lists the org's service plans (read-only). Requires scope `read:plans`.
- **`unifi-pp-cli carrier list-subscribers`** - Lists subscribers visible to the key (in-scope subscribers only). Cursor-paginated. Requires scope `read:subscribers`.
- **`unifi-pp-cli carrier resume-subscriber`** - Resumes the subscriber's service. Idempotent: resuming a not-suspended subscriber is a 200 no-op. Requires scope `resume:service`. Returns 400 if `id` is not a valid UUID.
- **`unifi-pp-cli carrier suspend-subscriber`** - Suspends the subscriber's service (subscriber-level). Idempotent: suspending an already-suspended subscriber is a 200 no-op. Requires scope `suspend:service`.
- **`unifi-pp-cli carrier update-subscriber`** - Updates name / email / notes / service address / plan / subscriber number / metadata. An absent field is left unchanged; a JSON `null` clears `email`, `notes`, or `planId` (the nullable-clearable fields). Requires scope `update:subscribers`.

### devices

Manage devices

- **`unifi-pp-cli devices`** - Retrieves a list of UniFi devices managed by hosts where the UI account making the API call is the owner or a super admin. <br/><br/>**Note**: The structure of the `devices.uidb` field may vary depending on the UniFi OS or Network Server version. The example provided is based on UniFi OS 4.1.13.

### hosts

Manage hosts

- **`unifi-pp-cli hosts get-by-id`** - Retrieves detailed information about a specific host by ID. <br/><br/>**Note**: The structure of the `userData` and `reportedState` fields may vary depending on the UniFi OS or Network Server version. The example provided is based on UniFi OS 4.1.13.
- **`unifi-pp-cli hosts list`** - Retrieves a list of all hosts associated with the UI account making the API call. <br/><br/>**Note**: The structure of `userData` and `reportedState` fields may vary depending on the UniFi OS or Network Server version. The example provided is based on UniFi OS 4.1.13.

### innerspace

Manage innerspace

- **`unifi-pp-cli innerspace get-asset`** - Serves a floor plan asset file for the given plan ID and filename. Requires the same session authentication as other integration endpoints.
- **`unifi-pp-cli innerspace list-access-points`** - Returns placed UniFi access points with position, orientation, mounting details, and status. Optionally filter by `siteId`.
- **`unifi-pp-cli innerspace list-floor-plans`** - Returns floor plans for RF planning import including image URLs, scale (pixels per meter), and coordinate origin. Optionally filter by `siteId`.
- **`unifi-pp-cli innerspace list-inventory`** - Returns managed access points and switches that are not placed on any floor plan. Optionally filter by `siteId`.
- **`unifi-pp-cli innerspace list-project`** - Returns the project state for external integrations including shapes, plans, products, wall types, and attenuation object types. Use the mode query parameter for 2D or 3D representations.
- **`unifi-pp-cli innerspace list-switches`** - Returns placed UniFi switches with position and status. Optionally filter by `siteId`.

### isp-metrics

Manage isp metrics

- **`unifi-pp-cli isp-metrics <type>`** - Retrieves ISP metrics data for all sites linked to the UI account's API key. 5-minute interval metrics are available for at least 24 hours, and 1-hour interval metrics are available for at least 30 days.

### mobility

Manage mobility

- **`unifi-pp-cli mobility get-device`** - Returns full detail for a single device, including WAN, cellular,
WiFi, VPN, firewall rules, subscription, and GPS location.

**Scope**: `read:mobility`
- **`unifi-pp-cli mobility list-device-clients`** - Returns a paginated list of all clients associated with a device,
including ONLINE, OFFLINE, and BLOCKED clients.

`wifi_experience` is omitted for wired clients (`type = WIRED`).

**Scope**: `read:mobility`
- **`unifi-pp-cli mobility list-devices`** - Returns a paginated list of devices in the workspace.
Each item contains identity and status fields only.
Use `getDevice` for full detail.

**Scope**: `read:mobility`
- **`unifi-pp-cli mobility list-workspace-admins`** - Returns all admins for a workspace with their permission levels.

The caller must have active membership. Only mobility permissions are exposed.

Admins are sorted: owner first, then active > pending > inactive > declined.

**Scope**: `read:mobility`
- **`unifi-pp-cli mobility list-workspaces`** - Returns all workspaces visible to the authenticated user.
Only workspaces with active membership are returned.

**Scope**: `read:mobility`
- **`unifi-pp-cli mobility update-device-name`** - Updates the user-assigned display name of a device.

**Scope**: `write:mobility`

**Permission**: workspace **Admin** (Viewers receive 403)
- **`unifi-pp-cli mobility update-device-network`** - Partial update: only provided fields are modified.
WAN, IPv6, and InternetSource are not configurable.

**Scope**: `write:mobility`

**Permission**: workspace **Admin**
- **`unifi-pp-cli mobility update-device-wireless`** - Replaces the WiFi SSID and password. Both fields are required.
Channel, TX power, and security protocol are not configurable.

**Scope**: `write:mobility`

**Permission**: workspace **Admin**

### network

Endpoints for creating, updating, deleting, and inspecting network configurations
including VLANs, DHCP, NAT, and IPv4/IPv6 settings.

- **`unifi-pp-cli network adopt-device`** - Adopt a device to a site.
- **`unifi-pp-cli network create`** - Create a new network on a site.
- **`unifi-pp-cli network create-acl-rule`** - Create a new user defined ACL rule on a site.
- **`unifi-pp-cli network create-dns-policy`** - Create a new DNS policy on a site.
- **`unifi-pp-cli network create-firewall-policy`** - Create a new firewall policy on a site.
- **`unifi-pp-cli network create-firewall-zone`** - Create a new custom firewall zone on a site.
- **`unifi-pp-cli network create-hotspot-voucher`** - Create one or more Hotspot vouchers.
- **`unifi-pp-cli network create-traffic-matching-list`** - Create a new traffic matching list on a site.
- **`unifi-pp-cli network create-wifi-broadcast`** - Create a new Wifi Broadcast on the specified site.
- **`unifi-pp-cli network delete`** - Delete an existing network on a site.
- **`unifi-pp-cli network delete-acl-rule`** - Delete an existing user defined ACL rule on a site.
- **`unifi-pp-cli network delete-dns-policy`** - Delete an existing DNS policy on a site.
- **`unifi-pp-cli network delete-firewall-policy`** - Delete an existing firewall policy on a site.
- **`unifi-pp-cli network delete-firewall-zone`** - Delete a custom firewall zone from a site.
- **`unifi-pp-cli network delete-hotspot-vouchers`** - Remove Hotspot vouchers based on the specified filter criteria.

<details>
<summary>Filterable properties (click to expand)</summary>

|Name|Type|Allowed functions|
|-|-|-|
|`id`|`UUID`|`eq` `ne` `in` `notIn`|
|`createdAt`|`TIMESTAMP`|`eq` `ne` `gt` `ge` `lt` `le`|
|`name`|`STRING`|`eq` `ne` `in` `notIn` `like`|
|`code`|`STRING`|`eq` `ne` `in` `notIn`|
|`authorizedGuestLimit`|`INTEGER`|`isNull` `isNotNull` `eq` `ne` `gt` `ge` `lt` `le`|
|`authorizedGuestCount`|`INTEGER`|`eq` `ne` `gt` `ge` `lt` `le`|
|`activatedAt`|`TIMESTAMP`|`eq` `ne` `gt` `ge` `lt` `le`|
|`expiresAt`|`TIMESTAMP`|`eq` `ne` `gt` `ge` `lt` `le`|
|`expired`|`BOOLEAN`|`eq` `ne`|
|`timeLimitMinutes`|`INTEGER`|`eq` `ne` `gt` `ge` `lt` `le`|
|`dataUsageLimitMBytes`|`INTEGER`|`isNull` `isNotNull` `eq` `ne` `gt` `ge` `lt` `le`|
|`rxRateLimitKbps`|`INTEGER`|`isNull` `isNotNull` `eq` `ne` `gt` `ge` `lt` `le`|
|`txRateLimitKbps`|`INTEGER`|`isNull` `isNotNull` `eq` `ne` `gt` `ge` `lt` `le`|
</details>
- **`unifi-pp-cli network delete-hotspot-voucher`** - Remove a specific Hotspot voucher.
- **`unifi-pp-cli network delete-traffic-matching-list`** - Delete an exist traffic matching list on a site.
- **`unifi-pp-cli network delete-wifi-broadcast`** - Delete an existing Wifi Broadcast from the specified site.
- **`unifi-pp-cli network get`** - Retrieve detailed information about a specific network.
- **`unifi-pp-cli network get-acl-rule`** - Get ACL Rule
- **`unifi-pp-cli network get-acl-rule-ordering`** - Retrieve user-defined ACL rule ordering on a site.
- **`unifi-pp-cli network get-client`** - Retrieve detailed information about a specific connected client, including name, IP address, MAC address, connection type and access information.
- **`unifi-pp-cli network get-device`** - Retrieve detailed information about a specific adopted device, including firmware versioning, uplink state, details about device features and interfaces (ports, radios) and other key attributes.
- **`unifi-pp-cli network get-device-statistics`** - Retrieve the latest real-time statistics of a specific adopted device, such as uptime, data transmission rates, CPU and memory utilization.
- **`unifi-pp-cli network get-dns-policy`** - Retrieve specific DNS policy.
- **`unifi-pp-cli network get-firewall-policy`** - Retrieve specific firewall policy.
- **`unifi-pp-cli network get-firewall-policy-ordering`** - Retrieve user-defined firewall policy ordering for a specific source/destination zone pair.
- **`unifi-pp-cli network get-firewall-zone`** - Get a firewall zone on a site.
- **`unifi-pp-cli network get-hotspot-voucher`** - Retrieve details of a specific Hotspot voucher.
- **`unifi-pp-cli network get-references`** - Retrieve references to a specific network.
- **`unifi-pp-cli network get-switching-lag`** - Retrieve LAG details.
- **`unifi-pp-cli network get-switching-mc-lag-domain`** - Retrieve MC-LAG Domain details.
- **`unifi-pp-cli network get-switching-switch-stack`** - Retrieve Switch Stack details.
- **`unifi-pp-cli network get-traffic-matching-list`** - Get an exist traffic matching list on a site.
- **`unifi-pp-cli network get-wifi-broadcast`** - Retrieve detailed information about a specific Wifi.
- **`unifi-pp-cli network get-info`** - Retrieve general information about the UniFi Network application.
- **`unifi-pp-cli network list`** - Retrieve a paginated list of all Networks on a site.

<details>
<summary>Filterable properties (click to expand)</summary>

|Name|Type|Allowed functions|
|-|-|-|
|`management`|`STRING`|`eq` `ne` `in` `notIn`|
|`id`|`UUID`|`eq` `ne` `in` `notIn`|
|`name`|`STRING`|`eq` `ne` `in` `notIn` `like`|
|`enabled`|`BOOLEAN`|`eq` `ne`|
|`vlanId`|`INTEGER`|`eq` `ne` `gt` `ge` `lt` `le` `in` `notIn`|
|`deviceId`|`UUID`|`eq` `ne` `in` `notIn` `isNull` `isNotNull`|
|`metadata.origin`|`STRING`|`eq` `ne` `in` `notIn`|
</details>
- **`unifi-pp-cli network list-acl-rules`** - Retrieve a paginated list of all ACL rules on a site.

<details>
<summary>Filterable properties (click to expand)</summary>

|Name|Type|Allowed functions|
|-|-|-|
|`type`|`STRING`|`eq` `ne` `in` `notIn`|
|`id`|`UUID`|`eq` `ne` `in` `notIn`|
|`enabled`|`BOOLEAN`|`eq` `ne`|
|`name`|`STRING`|`eq` `ne` `in` `notIn` `like`|
|`description`|`STRING`|`isNull` `isNotNull` `eq` `ne` `in` `notIn` `like`|
|`action`|`STRING`|`eq` `ne` `in` `notIn`|
|`index`|`INTEGER`|`eq` `ne` `gt` `ge` `lt` `le` `in` `notIn`|
|`protocolFilter`|`SET(STRING)`|`isNull` `isNotNull` `contains` `containsAny` `containsAll` `containsExactly`|
|`networkId`|`UUID`|`isNull` `isNotNull` `eq` `ne` `in` `notIn`|
|`enforcingDeviceFilter.deviceIds`|`SET(UUID)`|`isNull` `isNotNull` `contains` `containsAny` `containsAll` `containsExactly`|
|`metadata.origin`|`STRING`|`eq` `ne` `in` `notIn`|
|`sourceFilter.type`|`STRING`|`isNull` `isNotNull` `eq` `ne` `in` `notIn`|
|`sourceFilter.ipAddressesOrSubnets`|`SET(STRING)`|`contains` `containsAny` `containsAll` `containsExactly`|
|`sourceFilter.portFilter`|`SET(INTEGER)`|`isNull` `isNotNull` `contains` `containsAny` `containsAll` `containsExactly`|
|`sourceFilter.networkIds`|`SET(UUID)`|`contains` `containsAny` `containsAll` `containsExactly`|
|`sourceFilter.macAddresses`|`SET(STRING)`|`contains` `containsAny` `containsAll` `containsExactly`|
|`sourceFilter.prefixLength`|`INTEGER`|`isNull` `isNotNull` `eq` `ne` `gt` `ge` `lt` `le` `in` `notIn`|
|`destinationFilter.type`|`STRING`|`isNull` `isNotNull` `eq` `ne` `in` `notIn`|
|`destinationFilter.ipAddressesOrSubnets`|`SET(STRING)`|`contains` `containsAny` `containsAll` `containsExactly`|
|`destinationFilter.portFilter`|`SET(INTEGER)`|`isNull` `isNotNull` `contains` `containsAny` `containsAll` `containsExactly`|
|`destinationFilter.networkIds`|`SET(UUID)`|`contains` `containsAny` `containsAll` `containsExactly`|
|`destinationFilter.macAddresses`|`SET(STRING)`|`contains` `containsAny` `containsAll` `containsExactly`|
|`destinationFilter.prefixLength`|`INTEGER`|`isNull` `isNotNull` `eq` `ne` `gt` `ge` `lt` `le` `in` `notIn`|
</details>
- **`unifi-pp-cli network list-clients`** - Retrieve a paginated list of all connected clients on a site, including physical devices (computers, smartphones) and active VPN connections.

<details>
<summary>Filterable properties (click to expand)</summary>

|Name|Type|Allowed functions|
|-|-|-|
|`id`|`UUID`|`eq` `ne` `in` `notIn`|
|`type`|`STRING`|`eq` `ne` `in` `notIn`|
|`macAddress`|`STRING`|`isNull` `isNotNull` `eq` `ne` `in` `notIn`|
|`ipAddress`|`STRING`|`isNull` `isNotNull` `eq` `ne` `in` `notIn`|
|`connectedAt`|`TIMESTAMP`|`isNull` `isNotNull` `eq` `ne` `gt` `ge` `lt` `le`|
|`access.type`|`STRING`|`eq` `ne` `in` `notIn`|
|`access.authorized`|`BOOLEAN`|`isNull` `isNotNull` `eq` `ne`|
</details>
- **`unifi-pp-cli network list-countries`** - Returns ISO-standard country codes and names,
used for region-based configuration or regulatory compliance.

<details>
<summary>Filterable properties (click to expand)</summary>

|Name|Type|Allowed functions|
|-|-|-|
|`code`|`STRING`|`eq` `ne` `in` `notIn`|
|`name`|`STRING`|`eq` `ne` `in` `notIn` `like`|
</details>
- **`unifi-pp-cli network list-device-tags`** - Returns all device tags defined within a site, which can be used for WiFi Broadcast assignments.

<details>
<summary>Filterable properties (click to expand)</summary>

|Name|Type|Allowed functions|
|-|-|-|
|`id`|`UUID`|`eq` `ne` `in` `notIn`|
|`name`|`STRING`|`eq` `ne` `in` `notIn` `like`|
|`deviceIds`|`SET(UUID)`|`contains` `containsAny` `containsAll` `containsExactly`|
</details>
- **`unifi-pp-cli network list-devices`** - Retrieve a paginated list of all adopted devices on a site, including basic device information.

<details>
<summary>Filterable properties (click to expand)</summary>

|Name|Type|Allowed functions|
|-|-|-|
|`id`|`UUID`|`eq` `ne` `in` `notIn`|
|`macAddress`|`STRING`|`eq` `ne` `in` `notIn`|
|`ipAddress`|`STRING`|`eq` `ne` `in` `notIn`|
|`name`|`STRING`|`eq` `ne` `in` `notIn` `like`|
|`model`|`STRING`|`eq` `ne` `in` `notIn`|
|`state`|`STRING`|`eq` `ne` `in` `notIn`|
|`supported`|`BOOLEAN`|`eq` `ne`|
|`firmwareVersion`|`STRING`|`isNull` `isNotNull` `eq` `ne` `gt` `ge` `lt` `le` `like` `in` `notIn`|
|`firmwareUpdatable`|`BOOLEAN`|`eq` `ne`|
|`features`|`SET(STRING)`|`isEmpty` `contains` `containsAny` `containsAll` `containsExactly`|
|`interfaces`|`SET(STRING)`|`isEmpty` `contains` `containsAny` `containsAll` `containsExactly`|
</details>
- **`unifi-pp-cli network list-dns-policies`** - Retrieve a paginated list of all DNS policies on a site.

<details>
<summary>Filterable properties (click to expand)</summary>

|Name|Type|Allowed functions|
|-|-|-|
|`type`|`STRING`|`eq` `ne` `in` `notIn`|
|`id`|`UUID`|`eq` `ne` `in` `notIn`|
|`enabled`|`BOOLEAN`|`eq` `ne`|
|`domain`|`STRING`|`eq` `ne` `in` `notIn` `like`|
|`ipv4Address`|`STRING`|`eq` `ne` `in` `notIn` `like`|
|`ipv6Address`|`STRING`|`eq` `ne` `in` `notIn` `like`|
|`targetDomain`|`STRING`|`eq` `ne` `in` `notIn` `like`|
|`mailServerDomain`|`STRING`|`eq` `ne` `in` `notIn` `like`|
|`text`|`STRING`|`eq` `ne` `in` `notIn` `like`|
|`serverDomain`|`STRING`|`eq` `ne` `in` `notIn` `like`|
|`ipAddress`|`STRING`|`eq` `ne` `in` `notIn` `like`|
|`ttlSeconds`|`INTEGER`|`eq` `ne` `gt` `ge` `lt` `le` `in` `notIn`|
|`priority`|`INTEGER`|`eq` `ne` `gt` `ge` `lt` `le` `in` `notIn`|
|`service`|`STRING`|`eq` `ne` `in` `notIn`|
|`protocol`|`STRING`|`eq` `ne` `in` `notIn`|
|`port`|`INTEGER`|`eq` `ne` `gt` `ge` `lt` `le` `in` `notIn`|
|`weight`|`INTEGER`|`eq` `ne` `gt` `ge` `lt` `le` `in` `notIn`|
</details>
- **`unifi-pp-cli network list-dpi-applications`** - Lists DPI-recognized applications grouped under categories. Useful for firewall or traffic analytics integration.

<details>
<summary>Filterable properties (click to expand)</summary>

|Name|Type|Allowed functions|
|-|-|-|
|`id`|`INTEGER`|`eq` `ne` `in` `notIn`|
|`name`|`STRING`|`eq` `ne` `in` `notIn` `like`|
</details>
- **`unifi-pp-cli network list-dpi-categories`** - Returns predefined Deep Packet Inspection (DPI) application categories used for traffic identification and filtering.

<details>
<summary>Filterable properties (click to expand)</summary>

|Name|Type|Allowed functions|
|-|-|-|
|`id`|`INTEGER`|`eq` `ne` `in` `notIn`|
|`name`|`STRING`|`eq` `ne` `in` `notIn` `like`|
</details>
- **`unifi-pp-cli network list-firewall-policies`** - Retrieve a list of all firewall policies on a site.

<details>
<summary>Filterable properties (click to expand)</summary>

|Name|Type|Allowed functions|
|-|-|-|
|`id`|`UUID`|`eq` `ne` `in` `notIn`|
|`name`|`STRING`|`eq` `ne` `in` `notIn` `like`|
|`source.zoneId`|`UUID`|`eq` `ne` `in` `notIn`|
|`destination.zoneId`|`UUID`|`eq` `ne` `in` `notIn`|
|`metadata.origin`|`STRING`|`eq` `ne` `in` `notIn`|
</details>
- **`unifi-pp-cli network list-firewall-zones`** - Retrieve a list of all firewall zones on a site.

<details>
<summary>Filterable properties (click to expand)</summary>

|Name|Type|Allowed functions|
|-|-|-|
|`id`|`UUID`|`eq` `ne` `in` `notIn`|
|`name`|`STRING`|`eq` `ne` `in` `notIn` `like`|
|`metadata.origin`|`STRING`|`eq` `ne` `in` `notIn`|
|`metadata.configurable`|`BOOLEAN`|`eq` `ne` `isNull` `isNotNull`|
</details>
- **`unifi-pp-cli network list-hotspot-vouchers`** - Retrieve a paginated list of Hotspot vouchers.

<details>
<summary>Filterable properties (click to expand)</summary>

|Name|Type|Allowed functions|
|-|-|-|
|`id`|`UUID`|`eq` `ne` `in` `notIn`|
|`createdAt`|`TIMESTAMP`|`eq` `ne` `gt` `ge` `lt` `le`|
|`name`|`STRING`|`eq` `ne` `in` `notIn` `like`|
|`code`|`STRING`|`eq` `ne` `in` `notIn`|
|`authorizedGuestLimit`|`INTEGER`|`isNull` `isNotNull` `eq` `ne` `gt` `ge` `lt` `le`|
|`authorizedGuestCount`|`INTEGER`|`eq` `ne` `gt` `ge` `lt` `le`|
|`activatedAt`|`TIMESTAMP`|`eq` `ne` `gt` `ge` `lt` `le`|
|`expiresAt`|`TIMESTAMP`|`eq` `ne` `gt` `ge` `lt` `le`|
|`expired`|`BOOLEAN`|`eq` `ne`|
|`timeLimitMinutes`|`INTEGER`|`eq` `ne` `gt` `ge` `lt` `le`|
|`dataUsageLimitMBytes`|`INTEGER`|`isNull` `isNotNull` `eq` `ne` `gt` `ge` `lt` `le`|
|`rxRateLimitKbps`|`INTEGER`|`isNull` `isNotNull` `eq` `ne` `gt` `ge` `lt` `le`|
|`txRateLimitKbps`|`INTEGER`|`isNull` `isNotNull` `eq` `ne` `gt` `ge` `lt` `le`|
</details>
- **`unifi-pp-cli network list-pending-devices`** - Retrieve a paginated list of devices pending adoption, including basic device information.

<details>
<summary>Filterable properties (click to expand)</summary>

|Name|Type|Allowed functions|
|-|-|-|
|`macAddress`|`STRING`|`eq` `ne` `in` `notIn`|
|`ipAddress`|`STRING`|`eq` `ne` `in` `notIn`|
|`model`|`STRING`|`eq` `ne` `in` `notIn`|
|`state`|`STRING`|`eq` `ne` `in` `notIn`|
|`supported`|`BOOLEAN`|`eq` `ne`|
|`firmwareVersion`|`STRING`|`isNull` `isNotNull` `eq` `ne` `gt` `ge` `lt` `le` `like` `in` `notIn`|
|`firmwareUpdatable`|`BOOLEAN`|`eq` `ne`|
|`features`|`SET(STRING)`|`isEmpty` `contains` `containsAny` `containsAll` `containsExactly`|
</details>
- **`unifi-pp-cli network list-radius-profiles`** - Returns available RADIUS authentication profiles, including configuration origin metadata.

<details>
<summary>Filterable properties (click to expand)</summary>

|Name|Type|Allowed functions|
|-|-|-|
|`id`|`UUID`|`eq` `ne` `in` `notIn`|
|`name`|`STRING`|`eq` `ne` `in` `notIn` `like`|
|`metadata.origin`|`STRING`|`eq` `ne` `in` `notIn`|
</details>
- **`unifi-pp-cli network list-sites`** - Retrieve a paginated list of local sites managed by this Network application.
Site ID is required for other UniFi Network API calls.

<details>
<summary>Filterable properties (click to expand)</summary>

|Name|Type|Allowed functions|
|-|-|-|
|`id`|`UUID`|`eq` `ne` `in` `notIn`|
|`internalReference`|`STRING`|`eq` `ne` `in` `notIn`|
|`name`|`STRING`|`eq` `ne` `in` `notIn`|
</details>
- **`unifi-pp-cli network list-switching-lags`** - Retrieve a paginated list of all LAGs (Link Aggregation Groups) on a site.

<details>
<summary>Filterable properties (click to expand)</summary>

|Name|Type|Allowed functions|
|-|-|-|
|`id`|`UUID`|`eq` `ne` `in` `notIn`|
|`type`|`STRING`|`eq` `ne` `in` `notIn`|
|`switchStackId`|`UUID`|`eq` `ne` `in` `notIn` `isNull` `isNotNull`|
|`mcLagDomainId`|`UUID`|`eq` `ne` `in` `notIn` `isNull` `isNotNull`|
|`members.deviceId`|`SET(UUID)`|`contains` `containsAny` `containsAll` `containsExactly`|
|`members.portIdxs`|`SET(INTEGER)`|`contains` `containsAny` `containsAll` `containsExactly`|
|`metadata.origin`|`STRING`|`eq` `ne` `in` `notIn`|
</details>
- **`unifi-pp-cli network list-switching-mc-lag-domains`** - Retrieve a paginated list of all MC-LAG Domains on a site.

<details>
<summary>Filterable properties (click to expand)</summary>

|Name|Type|Allowed functions|
|-|-|-|
|`id`|`UUID`|`eq` `ne` `in` `notIn`|
|`name`|`STRING`|`eq` `ne` `in` `notIn` `like`|
|`peers.deviceId`|`SET(UUID)`|`contains` `containsAny` `containsAll` `containsExactly`|
|`metadata.origin`|`STRING`|`eq` `ne` `in` `notIn`|
</details>
- **`unifi-pp-cli network list-switching-switch-stacks`** - Retrieve a paginated list of all Switch Stacks on a site.

<details>
<summary>Filterable properties (click to expand)</summary>

|Name|Type|Allowed functions|
|-|-|-|
|`id`|`UUID`|`eq` `ne` `in` `notIn`|
|`name`|`STRING`|`eq` `ne` `in` `notIn` `like`|
|`units.id`|`SET(INTEGER)`|`contains` `containsAny` `containsAll` `containsExactly`|
|`units.macAddress`|`SET(STRING)`|`contains` `containsAny` `containsAll` `containsExactly`|
|`metadata.origin`|`STRING`|`eq` `ne` `in` `notIn`|
</details>
- **`unifi-pp-cli network list-traffic-matching-lists`** - Retrieve all traffic matching lists on a site.

<details>
<summary>Filterable properties (click to expand)</summary>

|Name|Type|Allowed functions|
|-|-|-|
|`id`|`UUID`|`eq` `ne` `in` `notIn`|
|`name`|`STRING`|`eq` `ne` `in` `notIn` `like`|
</details>
- **`unifi-pp-cli network list-vpn-servers`** - Retrieve a paginated list of all VPN servers on a site.

<details>
<summary>Filterable properties (click to expand)</summary>

|Name|Type|Allowed functions|
|-|-|-|
|`type`|`STRING`|`eq` `ne` `in` `notIn`|
|`id`|`UUID`|`eq` `ne` `in` `notIn`|
|`name`|`STRING`|`eq` `ne` `in` `notIn` `like`|
|`enabled`|`BOOLEAN`|`eq` `ne`|
|`metadata.origin`|`STRING`|`eq` `ne` `in` `notIn`|
</details>
- **`unifi-pp-cli network list-vpn-site-to-site-tunnels`** - Retrieve a paginated list of all site-to-site VPN tunnels on a site.

<details>
<summary>Filterable properties (click to expand)</summary>

|Name|Type|Allowed functions|
|-|-|-|
|`type`|`STRING`|`eq` `ne` `in` `notIn`|
|`id`|`UUID`|`eq` `ne` `in` `notIn`|
|`name`|`STRING`|`eq` `ne` `in` `notIn` `like`|
|`metadata.origin`|`STRING`|`eq` `ne` `in` `notIn`|
|`metadata.source`|`STRING`|`eq` `ne` `in` `notIn` `isNull` `isNotNull`|
</details>
- **`unifi-pp-cli network list-wans`** - Returns available WAN interface definitions for a given site,
including identifiers and names. Useful for network and NAT configuration.
- **`unifi-pp-cli network list-wifi-broadcasts`** - Retrieve a paginated list of all Wifi Broadcasts on a site.

<details>
<summary>Filterable properties (click to expand)</summary>

|Name|Type|Allowed functions|
|-|-|-|
|`type`|`STRING`|`eq` `ne` `in` `notIn`|
|`id`|`UUID`|`eq` `ne` `in` `notIn`|
|`enabled`|`BOOLEAN`|`eq` `ne`|
|`name`|`STRING`|`eq` `ne` `in` `notIn` `like`|
|`broadcastingFrequenciesGHz`|`SET(DECIMAL)`|`contains` `containsAny` `containsAll` `containsExactly`|
|`metadata.origin`|`STRING`|`eq` `ne` `in` `notIn`|
|`network.type`|`STRING`|`eq` `ne` `in` `notIn` `isNull` `isNotNull`|
|`network.networkId`|`UUID`|`eq` `ne` `in` `notIn`|
|`securityConfiguration.type`|`STRING`|`eq` `ne` `in` `notIn`|
|`broadcastingDeviceFilter.type`|`STRING`|`eq` `ne` `in` `notIn` `isNull` `isNotNull`|
|`broadcastingDeviceFilter.deviceIds`|`SET(UUID)`|`contains` `containsAny` `containsAll` `containsExactly`|
|`broadcastingDeviceFilter.deviceTagIds`|`SET(UUID)`|`contains` `containsAny` `containsAll` `containsExactly`|
|`hotspotConfiguration.type`|`STRING`|`eq` `ne` `in` `notIn`|
</details>
- **`unifi-pp-cli network patch-firewall-policy`** - Patch an existing firewall policy on a site.
- **`unifi-pp-cli network remove-device`** - Removes (unadopts) an adopted device from the site. If the device is online, it will be reset to factory defaults.
- **`unifi-pp-cli network run-client-action`** - Perform an action on a specific connected client. The request body must include the action name and any applicable input arguments.
- **`unifi-pp-cli network run-device-action`** - Perform an action on an specific adopted device. The request body must include the action name and any applicable input arguments.
- **`unifi-pp-cli network run-port-action`** - Perform an action on a specific device port. The request body must include the action name and any applicable input arguments.
- **`unifi-pp-cli network set-acl-rule-ordering`** - Reorder user-defined ACL rules on a site.
- **`unifi-pp-cli network set-firewall-policy-ordering`** - Reorder user-defined firewall policies for a specific source/destination zone pair.
- **`unifi-pp-cli network update`** - Update an existing network on a site.
- **`unifi-pp-cli network update-acl-rule`** - Update an existing user defined ACL rule on a site.
- **`unifi-pp-cli network update-dns-policy`** - Update an existing DNS policy on a site.
- **`unifi-pp-cli network update-firewall-policy`** - Update an existing firewall policy on a site.
- **`unifi-pp-cli network update-firewall-zone`** - Update a firewall zone on a site.
- **`unifi-pp-cli network update-traffic-matching-list`** - Update an exist traffic matching list on a site.
- **`unifi-pp-cli network update-wifi-broadcast`** - Update an existing Wifi Broadcast on the specified site.

### protect

Manage protect

- **`unifi-pp-cli protect alarm-hub-trigger`** - Trigger an alarm hub output channel. Can be used to turn on/off connected devices like sirens, lights, or other actuators.
- **`unifi-pp-cli protect alarm-webhook`** - Send a webhook to the alarm manager to trigger configured alarms
- **`unifi-pp-cli protect arm-disable`** - Disable the arm alarm feature. Only available when using local alarm manager.
- **`unifi-pp-cli protect arm-enable`** - Enable the arm alarm feature using the currently selected arm profile. Only available when using local alarm manager.
- **`unifi-pp-cli protect create-arm-profile`** - Create a new arm profile. Only available when using local alarm manager.
- **`unifi-pp-cli protect create-camera-rtsps-stream`** - Returns RTSPS stream URLs for specified quality levels
- **`unifi-pp-cli protect create-file`** - Upload a new device asset file
- **`unifi-pp-cli protect create-liveview`** - Create a new live view
- **`unifi-pp-cli protect create-talkback-session`** - Returns the talkback stream URL and audio configuration for a specific camera
- **`unifi-pp-cli protect delete-arm-profile`** - Delete an arm profile by ID. Only available when using local alarm manager.
- **`unifi-pp-cli protect delete-camera-rtsps-stream`** - Remove the RTSPS stream for a specified camera
- **`unifi-pp-cli protect disable-camera-mic-permanently`** - Disable the microphone for a specific camera. This action cannot be undone unless the camera is reset.
- **`unifi-pp-cli protect get-alarm-hub`** - Get detailed information about a specific alarm hub
- **`unifi-pp-cli protect get-bridge`** - Get detailed information about a specific bridge
- **`unifi-pp-cli protect get-camera`** - Get detailed information about a specific camera
- **`unifi-pp-cli protect get-camera-rtsps-stream`** - Returns existing RTSPS stream URLs for camera
- **`unifi-pp-cli protect get-camera-snapshot`** - Get a snapshot image from a specific camera. Use channel=package for cameras with a package camera (hasPackageCamera: true).
- **`unifi-pp-cli protect get-chime`** - Get detailed information about a specific chime
- **`unifi-pp-cli protect get-file`** - Get a list of all device asset files
- **`unifi-pp-cli protect get-fob`** - Get detailed information about a specific fob
- **`unifi-pp-cli protect get-light`** - Get detailed information about a specific light
- **`unifi-pp-cli protect get-link-station`** - Get detailed information about a specific link station (non-alarm hub gateways)
- **`unifi-pp-cli protect get-liveview`** - Get detailed information about a specific live view
- **`unifi-pp-cli protect get-relay`** - Get detailed information about a specific relay
- **`unifi-pp-cli protect get-sensor`** - Get detailed information about a specific sensor
- **`unifi-pp-cli protect get-siren`** - Get detailed information about a specific siren
- **`unifi-pp-cli protect get-speaker`** - Get detailed information about a specific speaker
- **`unifi-pp-cli protect get-ulp-user`** - Get detailed information about a specific UniFi Identity user.
- **`unifi-pp-cli protect get-user`** - Get detailed information about a specific Protect user.
- **`unifi-pp-cli protect get-viewer`** - Get detailed information about a specific viewer
- **`unifi-pp-cli protect get-info`** - Get generic information about the Protect application
- **`unifi-pp-cli protect list-alarm-hubs`** - Get detailed information about all alarm hubs
- **`unifi-pp-cli protect list-arm-profiles`** - Get a list of all arm profiles. Only available when using local alarm manager.
- **`unifi-pp-cli protect list-bridges`** - Get detailed information about all bridges
- **`unifi-pp-cli protect list-cameras`** - Get detailed information about all cameras
- **`unifi-pp-cli protect list-chimes`** - Get detailed information about all chimes
- **`unifi-pp-cli protect list-fobs`** - Get detailed information about all fobs
- **`unifi-pp-cli protect list-lights`** - Get detailed information about all lights
- **`unifi-pp-cli protect list-link-stations`** - Get detailed information about all link stations (non-alarm hub gateways)
- **`unifi-pp-cli protect list-liveviews`** - Get detailed information about all live views
- **`unifi-pp-cli protect list-nvrs`** - Get detailed information about the NVR
- **`unifi-pp-cli protect list-relays`** - Get detailed information about all relays
- **`unifi-pp-cli protect list-sensors`** - Get detailed information about all sensors
- **`unifi-pp-cli protect list-sirens`** - Get detailed information about all sirens
- **`unifi-pp-cli protect list-speakers`** - Get detailed information about all speakers
- **`unifi-pp-cli protect list-ulp-users`** - Get all UniFi Identity users with enrolled credentials (NFC cards, fingerprints).
- **`unifi-pp-cli protect list-users`** - Get all Protect users. Users are filtered based on access permissions.
- **`unifi-pp-cli protect list-viewers`** - Get detailed information about all viewers
- **`unifi-pp-cli protect patch-alarm-hub`** - Patch the settings for a specific alarm hub
- **`unifi-pp-cli protect patch-arm-profile`** - Update an existing arm profile. Only available when using local alarm manager.
- **`unifi-pp-cli protect patch-bridge`** - Patch the settings for a specific bridge
- **`unifi-pp-cli protect patch-camera`** - Patch the settings for a specific camera
- **`unifi-pp-cli protect patch-chime`** - Patch the settings for a specific chime
- **`unifi-pp-cli protect patch-fob`** - Patch the settings for a specific fob
- **`unifi-pp-cli protect patch-light`** - Patch the settings for a specific light
- **`unifi-pp-cli protect patch-link-station`** - Patch the settings for a specific link station
- **`unifi-pp-cli protect patch-liveview`** - Patch the configuration about a specific live view
- **`unifi-pp-cli protect patch-relay`** - Patch the settings for a specific relay
- **`unifi-pp-cli protect patch-sensor`** - Patch the settings for a specific sensor
- **`unifi-pp-cli protect patch-siren`** - Patch the settings for a specific siren
- **`unifi-pp-cli protect patch-speaker`** - Patch the settings for a specific speaker
- **`unifi-pp-cli protect patch-viewer`** - Patch the settings for a specific viewer
- **`unifi-pp-cli protect pos-transaction`** - Records a point of sale transaction as a camera event so the transaction details can be overlaid on recorded footage. Requires write access to the target camera. Footage capture is best-effort: the overlay only appears where the target camera was recording the transaction window (e.g. cameras in detections or adaptive recording modes), and a backdated timestamp may fall outside retained footage. A 200 response confirms the event was recorded, not that video exists for the window.
- **`unifi-pp-cli protect ptz-goto`** - Adjust the PTZ camera position to a specified preset
- **`unifi-pp-cli protect ptz-patrol-start`** - Start a camera PTZ patrol
- **`unifi-pp-cli protect ptz-patrol-stop`** - Stop active camera PTZ patrol
- **`unifi-pp-cli protect relay-activate`** - Control a relay output state. Use "state" to set a specific state (on/off), or omit to toggle. When state is "on", you can optionally provide "pulseDuration" to auto-turn off after the specified milliseconds.
- **`unifi-pp-cli protect set-arm-settings`** - Set the current arm profile to be used when enabling the arm alarm. Only available when using local alarm manager.
- **`unifi-pp-cli protect siren-play`** - Activate the siren alarm for the specified duration. The siren status will be tracked and can be stopped early using the stop endpoint.
- **`unifi-pp-cli protect siren-stop`** - Stop an active siren.
- **`unifi-pp-cli protect siren-test-sound`** - Test the siren sound for 5 seconds at the specified volume.
- **`unifi-pp-cli protect speaker-test-sound`** - Test the speaker sound at the specified volume.

### sd-wan-configs

Manage sd wan configs

- **`unifi-pp-cli sd-wan-configs get-sdwan-config-by-id`** - Retrieves detailed information about a specific SD-WAN configuration by ID.
- **`unifi-pp-cli sd-wan-configs list-sdwan-configs`** - Retrieves a list of all SD-WAN configurations associated with the UI account making the API call.

### sites

Manage sites

- **`unifi-pp-cli sites`** - Retrieves a list of all sites (from hosts running the UniFi Network application) associated with the UI account making the API call. <br/><br/>**Note**: The structure of the `meta` and `statistics` fields may vary depending on the UniFi Network version. The example provided is based on UniFi OS 4.1.13.


### change

- **`unifi-pp-cli change apply`** - Send one change to an official Network or Protect path with a journaled before-image, a read-back check and undo. Nothing is sent without --confirm.
- **`unifi-pp-cli change list`** - List journaled changes, so an undo can find its id.
- **`unifi-pp-cli change undo`** - Revert a journaled change when current state still matches its after-image. Nothing is sent without `--confirm`.


### digest


- **`unifi-pp-cli digest`** - One timeline, built from local syncs, of state changes, ISP dips, drift, newcomers and journaled changes.


### drift


- **`unifi-pp-cli drift`** - See field-level config changes between two synced snapshots, marked attributed (in the change journal) or out-of-band.


### impact


- **`unifi-pp-cli impact`** - Estimate from the last sync what a proposed action would take down and whether it risks your way back in. It never reports safe.


### newcomer


- **`unifi-pp-cli newcomer`** - List clients, devices and cameras first seen at a sync in a window.


### reach


- **`unifi-pp-cli reach`** - See which transports and products answer on each console. Write status is yes, no or unproven, and a probe alone never reports yes.


### rule-predict


- **`unifi-pp-cli rule-predict`** - Predict which firewall policy would match a hypothetical packet. This is a local prediction with no API confirmation. This is a local prediction with no API confirmation.


### topology


- **`unifi-pp-cli topology`** - See gateway, switches and APs as a tree per console, from the last sync.




### raw

- **`unifi-pp-cli raw [method] [path]`** - Call a path on the selected console under `/proxy`, for example `network/api/s/default/stat/health`. This is the only route to controller paths, which are unofficial and unverified. Reads run directly. Other methods print a preview until you pass `--confirm`. Site Manager paths (`v1/...`) and paths that leave `/proxy` are refused.

### fleet

- **`unifi-pp-cli fleet [health|inventory|firmware]`** - Read hosts and devices from Site Manager across every console the key can see, then probe each console through the connector (`health` and bare `fleet` only). A console that does not answer stays listed with `connector=failed` and is named in `fetch_failures`. Partial failures still exit 0, so read `fetch_failures`. Needs `UNIFI_API_KEY`. `--no-probe` skips the probes, `--max-probes` caps them (default 50).

### audit

- **`unifi-pp-cli audit`** - Read the local log of attempted writes. Outcomes are `applied`, `refused` and `ambiguous`. Filter with `--since`, `--method`, `--path`, `--outcome` and `--console`.

### sync

- **`unifi-pp-cli sync`** - Mirror API data to local SQLite for the selected console, then fetch device details and take the snapshot that `drift`, `digest` and `newcomer` read. A failed sync takes no snapshot. Device details are one request each: `--max-hydrate` caps them per sync (default 100, `0` for no cap), and `--no-hydrate` skips them, which leaves `topology` flat and `impact` at `cannot-determine` for what sits below a device.

### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`unifi-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`unifi-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`unifi-pp-cli learnings list`** - Inspect taught rows
- **`unifi-pp-cli learnings forget <query>`** - Undo a teach
- **`unifi-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`unifi-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`unifi-pp-cli teach-pattern`** - Install a query/resource template up front
- **`unifi-pp-cli teach-lookup`** - Add an entity mapping (e.g. a site alias) for pattern substitution

Pass `--no-learn` or set `UNIFI_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `unifi-pp-cli` opens the database, older binaries refuse it with a version error: upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
unifi-pp-cli devices

# JSON for scripting and agents
unifi-pp-cli devices --json
# Filter to specific fields
unifi-pp-cli devices --json --select devices,hostId,hostName

# Dry run: show the request without sending
unifi-pp-cli devices --dry-run

# Agent mode: JSON + compact + no prompts in one flag
unifi-pp-cli devices --agent
```

`--csv` prefixes a leading `'` on cells and header keys that start with `=`, `+`, `-`, `@`, a tab, or a line break (including fullwidth `＝＋－＠`) so a spreadsheet does not run them as formulas. Plain signed numbers such as `-5` and `+2.5e3` are left unchanged.

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select <field>[,<field>...]` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending. The output shape varies by command: rely on the exit code and `dry_run: true`
- **Explicit retries** - add `--idempotent` to create retries and add `--ignore-missing` to delete retries when a no-op success is acceptable
- **Writes send immediately unless the command says otherwise** - `network`, `protect`, `carrier` and `mobility` writes have no confirmation step, so use `--dry-run` first. `raw`, `change apply` and `change undo` preview until `--confirm`. `--agent` implies neither `--confirm` nor `--yes`
- **Check before planning a write** - run `reach`. Docs say cloud keys are read-only today, and only one voucher create and delete has been proven, so a write through the connector may be refused, and local writes are untested live
- **Fence an agent** - `UNIFI_READ_ONLY` refuses all writes. `UNIFI_CONSOLE_ALLOWLIST` limits connector requests to named consoles, but not Site Manager reads or the local route A named console is also checked on the local route.
- **Piped input** - write commands can accept structured input when their help lists `--stdin`
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `1` other error, `2` usage error, `3` not found, `4` auth error (including a missing local key), `5` API error (including a console's HTML page), `6` partial failure, `7` rate limited, `10` config error (including no console selected, a console outside the allowlist, and writes refused by read-only mode). `fleet` and `reach` report per-console failures in their output and can still exit 0.

## Health Check

```bash
unifi-pp-cli doctor
```

Reports config, paths, the resolved transport, whether a credential is present, and whether `api.ui.com` answers. It does not validate the key and does not contact a local gateway. `doctor --dry-run` only confirms the binary runs. To test a key or a route, use:

```bash
unifi-pp-cli reach --agent
unifi-pp-cli reach --gateway 192.168.1.1 --json
```

## Configuration

Run `unifi-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/unifi-pp-cli/config.toml` on macOS/Linux and `%USERPROFILE%\.config\unifi-pp-cli\config.toml` on Windows; `--home`, `UNIFI_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

`--ca-cert`, `UNIFI_CA_CERT`, or `ca_cert` adds a PEM CA bundle to the system trust roots. On a private gateway address, verification is off by default because UniFi OS uses a self-signed certificate, and the CLI warns about it. Setting a CA turns verification on, which is the safer setup because otherwise anyone who can answer at that address on your LAN receives the local key. `--tls-server-name`, `UNIFI_TLS_SERVER_NAME`, or `tls_server_name` is the name checked against the certificate when the dialed address does not match it. Precedence is flag, then environment, then the config file. A config file other users can change does not supply these settings. Certificate verification failures are not retried.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `UNIFI_API_KEY` | per_call | For cloud routes | Cloud key from unifi.ui.com, Settings > API Keys. Never sent to a gateway. |
| `UNIFI_LOCAL_API_KEY` | per_call | For the local route | Local key from the Network app, Settings > Control Plane > Integrations. Also `local_api_key` in config. |
| `UNIFI_CONSOLE_ID` | routing | With several consoles | Console id for the connector. Same as `--console`, or `console_id` in config. |
| `UNIFI_GATEWAY` | routing | For the local route | Gateway address. Same as `--gateway`, or `gateway` in config. |
| `UNIFI_TRANSPORT` | routing | No | `auto`, `cloud` or `local`. Same as `--transport`, or `transport` in config. |
| `UNIFI_GATEWAY_HOST` | routing | No | Deprecated alias for `UNIFI_GATEWAY`, kept for existing installs. When it picks the gateway and no local key is set, `UNIFI_API_KEY` is used as that gateway's key only, and is not sent to the Site Manager API. |
| `UNIFI_INSECURE_SKIP_VERIFY` | TLS | No | `1`, `true`, `yes` or `on` skips gateway certificate checks; `0`, `false`, `no` or `off` forces them. Default: skip for literal private addresses only. |
| `UNIFI_READ_ONLY` | safety | No | Any value except empty, `0`, `false`, `no` or `off` refuses all API writes from the CLI and MCP server. |
| `UNIFI_CONSOLE_ALLOWLIST` | safety | No | Comma-separated console ids allowed on the connector. Does not limit Site Manager reads or the local route. A named console is also checked on the local route. |
| `UNIFI_BASE_URL` | routing | No | Replaces `https://api.ui.com`. For mocks and proxies, not for reaching a gateway. |
| `UNIFI_MCP_READ_ONLY` | safety | No | Exactly `1`: the MCP server drops tools not marked read-only. |
| `UNIFI_MCP_HTTP_TOKEN` | MCP | For MCP over HTTP | Bearer token of at least 32 characters. |
| `UNIFI_MCP_HTTP_ALLOWED_ORIGINS` | MCP | No | Non-loopback origins the MCP HTTP server accepts. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `unifi-pp-cli doctor` reports `agentcookie: detected` and `auth status` labels the source as `agentcookie`. It syncs `UNIFI_API_KEY` only, not `UNIFI_LOCAL_API_KEY`. Skip this section if you don't use agentcookie.

## Troubleshooting

**Authentication errors (exit code 4)**
- Run `unifi-pp-cli doctor` to see the resolved route and whether a credential is present. It does not validate the key.
- Cloud route: check `UNIFI_API_KEY` is set. Local route: check `UNIFI_LOCAL_API_KEY`. A cloud key does not work on a gateway.

**Not found errors (exit code 3)**
- Check the resource ID is correct.
- Run the matching `list` command to see available items.

### API-specific
- **HTML body or exit 5** — A console page means the path missed the API. Check the route with doctor and run reach. Pass --gateway as a bare host: the CLI adds /proxy itself and rejects a path.
- **no console selected (exit 10)** — Several consoles are visible, or none. Choose one with --console or UNIFI_CONSOLE_ID, or use --gateway.
- **local gateway transport needs UNIFI_LOCAL_API_KEY (exit 4)** — Cloud keys are not sent to a gateway. Create a key in the Network app under Settings > Control Plane > Integrations.
- **401, 403 or 405 on a write through the connector** — Docs say cloud keys are read-only today. Try --gateway with a local key; local writes are also untested live.
- **403 not the owner of the host** — Non-organization keys reach only the key owner's consoles; use the owner's key or an organization key.
- **408 through the connector** — The connector limits requests to 25 seconds; narrow the query. A write may still have applied and is not replayed, so read state first.
- **429 through the connector** — The connector documents 100 requests per minute per console. The CLI paces itself per process, so parallel runs can exceed it; wait for Retry-After.
- **Response too large through the connector** — The connector caps responses at 10 MB. Page with --limit and --offset, or use --gateway.
- **read-only mode (exit 10)** — UNIFI_READ_ONLY is set, so writes are refused. Unset it to write; --dry-run still works.
- **console is not in UNIFI_CONSOLE_ALLOWLIST (exit 10)** — Add the console id to the comma-separated list, or target an allowed console.

### Local history
- **`topology` shows every device as a root, or `impact` says `cannot-determine`**: uplinks come from per-device detail requests. Run `sync` without `--no-hydrate`, and raise `--max-hydrate` if devices were skipped.
- **`drift` or `digest` is empty**: each successful sync takes one snapshot, and a comparison needs two. A failed sync takes none.
- **`rule-predict` says a zone is not in the mirror (exit 2)**: run `sync`, then use a zone name or id from the known list it prints.

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**unifi-mcp**](https://github.com/sirkirby/unifi-mcp): Python (979 stars)
- [**unifi-mcp-server**](https://github.com/enuno/unifi-mcp-server): Python (333 stars)
- [**mcp-unifi**](https://github.com/pete-builds/mcp-unifi): Python (29 stars)
- [**mcp-unifi (Workers)**](https://github.com/jason-mspkickstart/mcp-unifi): TypeScript (1 stars)
- [**unifi-cli**](https://github.com/lucasilverentand/unifi-cli): TypeScript

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
