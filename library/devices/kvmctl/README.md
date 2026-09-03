# kvmctl

`kvmctl` is a Go CLI and MCP server for KVMD-compatible KVM devices such as GLKVM. It provides a verified KVMD REST client plus safe, agent-friendly workflows for device status, screenshots, HID control, target selection, recovery, OCR-assisted actions, and immutable workflows.

Status: **live-verified against KVMD 4.82**. Read-only API and capability checks were exercised against a real device. Mutating hardware actions remain explicitly gated and were not used during acceptance.

## Install

### Printing Press installer

Install the CLI and the companion agent skill:

```bash
npx -y @mvanhorn/printing-press-library install kvmctl
```

CLI only:

```bash
npx -y @mvanhorn/printing-press-library install kvmctl --cli-only
```

Skill only:

```bash
npx -y @mvanhorn/printing-press-library install kvmctl --skill-only
```

### Go

From the published Printing Press catalog:

```bash
go install github.com/mvanhorn/printing-press-library/library/devices/kvmctl/cmd/kvmctl-pp-cli@latest
go install github.com/mvanhorn/printing-press-library/library/devices/kvmctl/cmd/kvmctl-pp-mcp@latest
```

To build this repository directly:

```bash
make build-all
```

### Release binaries

Download the CLI or MCP server archives from the [kvmctl releases](https://github.com/keithah/kvmctl/releases). Archives are published for macOS, Linux, and Windows on amd64 and arm64 where supported. Verify `checksums.txt` before installing a downloaded binary.

## Configure authentication

KVMD credentials are supplied through the environment or the CLI's private credential store. Never commit credentials or put them in a shell history.

```bash
export KVMCTL_KVMD_TOKEN="<your-kvmd-token>"
kvmctl-pp-cli doctor --json
```

The persisted form is private to the local user:

```bash
printf '%s\n' "$KVMCTL_KVMD_TOKEN" | kvmctl-pp-cli auth set-token
kvmctl-pp-cli doctor --json
```

The token is stored in `credentials.toml` under the resolved data directory, not in `config.toml`.

## Quick start

Start with read-only checks:

```bash
kvmctl-pp-cli doctor --json
kvmctl-pp-cli capabilities --json
kvmctl-pp-cli info --json
kvmctl-pp-cli hid get-state --json
```

Use agent mode when invoking from an automation tool:

```bash
kvmctl-pp-cli info --agent
kvmctl-pp-cli semantic capabilities --agent
```

The live acceptance probe confirmed an authenticated KVMD device, its capabilities, and online keyboard/mouse HID state. The probe did not send keyboard or mouse input.

## Safety model

- Read-only status, capability, screenshot, and inspection commands can run normally.
- Commands that can affect a KVM, host, target, or workflow require explicit confirmation and/or write policy.
- `--agent` selects machine-readable output; it does **not** imply `--yes`.
- Use `--dry-run --agent` before an unfamiliar mutating command.
- Do not use reboot, target switching, OTG, HID input, or workflow execution against production hardware without identifying the target and reviewing the command's help.
- OCR commands require real image bytes. The CLI never invents screenshots, OCR text, coordinates, or hardware results.

Example dry run:

```bash
kvmctl-pp-cli semantic send-key --key Enter --dry-run --agent
```

## Core capabilities

### KVMD API and device state

```bash
kvmctl-pp-cli capabilities --json
kvmctl-pp-cli info --json
kvmctl-pp-cli status --json
kvmctl-pp-cli screenshot --output ./screen.jpg
kvmctl-pp-cli hid get-state --json
```

### Keyboard and mouse

```bash
kvmctl-pp-cli keyboard --help
kvmctl-pp-cli mouse --help
kvmctl-pp-cli hid send-key --help
```

These commands are write operations. Review the help, identify the target, and pass `--yes` only when the input is intentional.

### Semantic operations

The semantic surface exposes the Python oracle's operation catalog through stable evidence envelopes. Discover the available operations and their read/write policy at runtime:

```bash
kvmctl-pp-cli semantic capabilities --agent
kvmctl-pp-cli semantic snapshot --agent
kvmctl-pp-cli semantic verify --agent
kvmctl-pp-cli semantic host-identity --agent
```

The MCP server exposes the same structured `semantic_dispatch` surface for agents.

### OCR observation loop

Use the purpose-built commands for the bounded observe → act → verify loop. They delegate only to the semantic core; they are not a planner and never infer a next UI action.

```bash
# Configure a local OCR command. It receives screenshot bytes on stdin.
export KVMCTL_OCR_COMMAND=tesseract
export KVMCTL_OCR_PROTOCOL=json

kvmctl-pp-cli observe --agent
kvmctl-pp-cli act click-text "Advanced" --observation <observation-id> --yes --agent
kvmctl-pp-cli act press-key F10 --observation <observation-id> --yes --agent
kvmctl-pp-cli verify --expect-text "Save Changes" --agent
```

`observe` returns the OCR observation ID. Both actions require that exact `--observation` value and explicit `--yes`; the semantic core captures the screen again and refuses a stale, unavailable, ambiguous, or non-matching observation. `verify` always captures a new observation and checks one exact high-confidence text match. `KVMCTL_OCR_PROTOCOL=json` selects the JSON OCR protocol expected by the configured command. These commands have automated HTTP/OCR fixtures only; no live BIOS interaction is claimed.

For MCP, call `semantic_dispatch` with `operation: "observe"`, `"verify-text"`, `"click-text"`, or `"press-key"`. The tool schema describes the required arguments; mutating operations additionally need `arguments.write_enabled: true` and the MCP host must permit writes with `KVMCTL_WRITE_ENABLED=1`.

### Immutable workflows

Workflows are loaded from JSON, listed deterministically, inspected with action values redacted, authorized once, and then executed only against the resolved target and revision.

```bash
kvmctl-pp-cli workflow-list --repository ./workflows.json --agent
kvmctl-pp-cli workflow-inspect --repository ./workflows.json --name safe-check --agent
kvmctl-pp-cli workflow-authorize --repository ./workflows.json --name safe-check --target <target> --agent
kvmctl-pp-cli workflow-execute --repository ./workflows.json --name safe-check --target <target> --yes --agent
```

Keep workflow files free of passwords, tokens, private URLs, and machine-specific secrets.

### Machine selection and recovery

```bash
kvmctl-pp-cli machines --help
kvmctl-pp-cli target-switch --help
kvmctl-pp-cli sequence --help
kvmctl-pp-cli workflow --help
```

The implementation includes bounded verification, session-integrity checks, target locking, cancellation-safe recovery, and checkpointed host reboot support. Hardware-changing paths remain opt-in.

## Output and agent use

Every command supports the generated CLI's machine-output flags where applicable:

```bash
kvmctl-pp-cli info --json
kvmctl-pp-cli info --agent
kvmctl-pp-cli info --json --select ok,result
kvmctl-pp-cli info --dry-run --agent
```

- JSON goes to stdout; errors go to stderr.
- `--agent` expands to JSON, compact output, no prompts, and no color.
- `--select` limits returned fields.
- `--dry-run` previews a request without sending it.
- Exit codes distinguish usage, missing resources, authentication, API, rate-limit, and configuration failures.

Run `kvmctl-pp-cli --help` and `kvmctl-pp-cli <command> --help` for the current command tree rather than relying on a copied list.

## Paths and environment

The CLI separates configuration, durable data, runtime state, and cache files:

| Kind | Contents |
| --- | --- |
| `config` | settings, profiles, and `config.toml` |
| `data` | `credentials.toml`, SQLite data, cookies, and auth sidecars |
| `state` | persisted queries, jobs, journals, and learning state |
| `cache` | regenerable HTTP/cache files |

Resolution order is the per-kind variable, `--home`, `KVMCTL_HOME`, XDG variables, then platform defaults.

```bash
export KVMCTL_HOME=/srv/kvmctl
kvmctl-pp-cli doctor --json
```

Supported environment variables include:

| Variable | Purpose |
| --- | --- |
| `KVMCTL_KVMD_TOKEN` | KVMD API credential |
| `KVMCTL_HOME` | relocate all local data kinds under one root |
| `KVMCTL_CONFIG_DIR` | override configuration directory |
| `KVMCTL_DATA_DIR` | override durable data directory |
| `KVMCTL_STATE_DIR` | override runtime state directory |
| `KVMCTL_CACHE_DIR` | override cache directory |
| `KVMCTL_NO_LEARN` | disable the local learning loop |
| `KVMCTL_OCR_COMMAND` | local OCR executable; it receives fresh screenshot bytes on stdin |
| `KVMCTL_OCR_PROTOCOL` | OCR response protocol (`json` for the structured observation loop) |
| `KVMCTL_LOCK_DIR` | shared directory used to serialize physical device actions (default `/tmp/kvmctl-locks`) |

For MCP, put these variables in the host's MCP server environment. The MCP binary does not receive CLI flags.

## MCP server

Install and run the MCP server:

```bash
go install github.com/mvanhorn/printing-press-library/library/devices/kvmctl/cmd/kvmctl-pp-mcp@latest
kvmctl-pp-mcp
```

Example Claude Desktop configuration:

```json
{
  "mcpServers": {
    "kvmctl": {
      "command": "kvmctl-pp-mcp",
      "env": {
        "KVMCTL_KVMD_TOKEN": "<your-kvmd-token>"
      }
    }
  }
}
```

The MCP server never receives secrets through committed configuration. Use the host environment or the CLI's private credential store.

## Development and verification

Requirements: Go 1.26.6 or newer.

```bash
make test
make build-all
go vet ./...
git diff --check
```

The release acceptance path additionally runs the official Printing Press validation and live dogfood checks. The live acceptance record is stored under `.manuscripts/` and contains only redacted metadata and source fingerprints.

The original Python implementation used for parity comparison is maintained outside this repository and is not required at runtime.

## Related documentation

- [`SKILL.md`](SKILL.md) — agent installation and operation protocol
- [`AGENTS.md`](AGENTS.md) — generated-tree and safety invariants
- [`CHANGELOG.md`](CHANGELOG.md) — maintained by Printing Press release automation
- [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
- [Printing Press library](https://github.com/mvanhorn/printing-press-library)

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press).
