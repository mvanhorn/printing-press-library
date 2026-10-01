# kuma-pp-cli

`kuma-pp-cli` is a small operator CLI for Uptime Kuma v2. It uses Kuma's Socket.IO protocol rather than pretending the dashboard is a REST API.

Created by [@keithah](https://github.com/keithah) (Keith).
Contributors: [@cathrynlavery](https://github.com/cathrynlavery) (Cathryn Lavery).

This catalog package is intentionally separate from the native `kumactl` HTTP MCP service. The package currently contains the generated Go CLI and its Socket.IO client; it does not install or embed a Python `kumactl` dependency, and it does not provide an MCP transport.

## Configuration

Set `UPTIME_KUMA_URL`, `UPTIME_KUMA_USERNAME`, and `UPTIME_KUMA_PASSWORD` (or pass `--url`, `--username`, and `--password`). Never commit credentials.

## Commands

```text
kuma-pp-cli health
kuma-pp-cli monitors [--query text] [--json]
kuma-pp-cli heartbeats [--hours 3] [--monitor-id N] [--json]
kuma-pp-cli incident-context --monitor id-or-name [--lookback-minutes 60] [--json]
kuma-pp-cli set-retries --monitor id-or-name --maxretries N       # dry-run
kuma-pp-cli set-retries --monitor id-or-name --maxretries N --yes
kuma-pp-cli agent-context [--pretty]
kuma-pp-cli version
kuma-pp-cli --version
```

`UPTIME_KUMA_URL` must be the server origin (for example `https://kuma.example.com`). A dashboard page URL is reduced to its origin automatically.

Use HTTPS for a remote server. Plain HTTP is accepted only for a local loopback server. Redirects to another host or port are refused because login credentials travel in Socket.IO requests. An incident lookup with an ambiguous monitor name asks for an exact name or ID, and an edit reports an error unless the readback confirms the change.

`agent-context` emits machine-readable JSON describing every command, flag, and auth variable, so agents can introspect the CLI without parsing `--help`.

## Native HTTP MCP service

For MCP clients, deploy the separately published native `kumactl` service rather than this generated Go CLI:

```bash
kumactl-mcp --transport streamable-http --host 127.0.0.1 --port 40108 --path /mcp
```

The native service uses the same `UPTIME_KUMA_URL`, `UPTIME_KUMA_USERNAME`, and `UPTIME_KUMA_PASSWORD` environment variables. This package preserves those variables for `kuma-pp-cli`; it does not rename the CLI to `kumactl` or add a dependency on the native service.

All mutations are operator-gated and dry-run by default. The client sends complete monitor objects for `editMonitor`, because Kuma v2 treats edits as full replacement.

## Development

```bash
go test ./...
go vet ./...
go build ./...
```
