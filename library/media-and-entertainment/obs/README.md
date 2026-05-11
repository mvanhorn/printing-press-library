# obs-pp-cli

Control OBS Studio from your terminal or AI agent via its built-in WebSocket server.

Scenes, layouts, guest sources, streaming, recording, and preflight checks — all from one binary.

## Install

The recommended path installs both the `obs-pp-cli` binary and the `pp-obs` agent skill in one shot:

```bash
npx -y @mvanhorn/printing-press install obs
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install obs --cli-only
```

### Without Node (Go fallback)

If `npx` isn't available, install directly via Go (requires Go 1.23+):

```bash
go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/obs/cmd/obs-pp-cli@latest
```

Add `$(go env GOPATH)/bin` to your PATH if not already there.

## OBS Setup

Enable the WebSocket server in OBS before using this CLI:

**Tools → WebSocket Server Settings → ✅ Enable WebSocket server → OK**

Default port: `4455`. Authentication is optional for local use.

## Quick Start

```bash
# Configure your connection (run once)
obs-pp-cli configure --non-interactive --host localhost --port 4455 --password ""

# Verify everything is working
obs-pp-cli doctor

# Build an interview layout
obs-pp-cli layout interview

# Add your guest's VDO.Ninja link
obs-pp-cli guest add --source "Guest Camera" --url "https://vdo.ninja/?view=GUESTID&cleanoutput&transparent"

# Run preflight checks
obs-pp-cli preflight

# Switch to the scene and go live
obs-pp-cli scene switch "Interview"
obs-pp-cli stream start
```

## Agent Usage

All commands support structured JSON output for LLM and agent use:

```bash
# Force JSON output
obs-pp-cli scene list --format=json
obs-pp-cli stream status --format=json
obs-pp-cli health --format=json

# Agent mode: JSON by default, no prompts, structured exit codes
obs-pp-cli --agent scene list
obs-pp-cli --agent preflight && obs-pp-cli --agent stream start

# Dry-run mode: see what would happen without changing OBS
obs-pp-cli --dry-run layout interview --name "Test"
obs-pp-cli --dry-run stream start

# Search for scenes or sources
obs-pp-cli search --query "interview" --format=json
obs-pp-cli search --query "camera" --type source --format=json

# Export full scene inventory for backup or handoff
obs-pp-cli export > scene-backup.json
obs-pp-cli export --scene "Interview" > interview-scene.json
```

Exit codes:
- `0` — success
- `1` — OBS connection error
- `2` — resource not found
- `3` — config missing (run `obs-pp-cli configure`)
- `4` — invalid argument
- `5` — preflight check failed

## Doctor / Health Check

Run the diagnostics command to verify your setup:

```bash
obs-pp-cli doctor
obs-pp-cli doctor --format=json
```

Checks:
1. Config file exists and is readable
2. OBS WebSocket port is reachable
3. WebSocket auth succeeds
4. OBS version >= 28 (WebSocket v5 required)

For a live system health snapshot (scene, stream, record, CPU, FPS):

```bash
obs-pp-cli health
obs-pp-cli health --format=json
```

## Recipes / Cookbook

### Live Interview Show

```bash
# Build the interview layout
obs-pp-cli layout interview --name "EP05"

# Add guest feeds (get VDO.Ninja links at https://vdo.ninja)
obs-pp-cli guest add --source "Host Camera" --url "https://vdo.ninja/?push=myroom&cleanoutput&transparent" --scene "EP05"
obs-pp-cli guest add --source "Jason" --url "https://vdo.ninja/?view=jasonroom&cleanoutput&transparent" --scene "EP05"

# Pre-show checks
obs-pp-cli preflight

# Go live
obs-pp-cli scene switch "EP05"
obs-pp-cli stream start
obs-pp-cli record start

# Intermission
obs-pp-cli scene switch "BRB"

# Wrap up
obs-pp-cli scene switch "EP05"
obs-pp-cli stream stop
obs-pp-cli record stop
```

### Scene Audit

```bash
# Find all scenes containing "interview"
obs-pp-cli search --query "interview" --type scene --format=json

# Find all camera sources across all scenes
obs-pp-cli search --query "camera" --type source --format=json

# Export full scene inventory
obs-pp-cli export | python3 -m json.tool
```

### Performance Monitoring

```bash
# Quick health snapshot
obs-pp-cli health

# Full perf stats (CPU, FPS, frame drops, disk)
obs-pp-cli stats --format=json

# Watch performance live (every 5 seconds)
watch -n5 obs-pp-cli stats
```

### Save and Switch Profiles

```bash
# List saved OBS profiles
obs-pp-cli profile list

# Switch to a profile
obs-pp-cli profile switch "Gaming"

# Show current profile
obs-pp-cli profile current
```

### Post-Production Delivery

```bash
# Export recording metadata after a session
obs-pp-cli deliver --title "Episode 5" --notes "Great interview, check audio at 12:30"

# View delivery history
obs-pp-cli deliver list
```

## Troubleshooting

### "Could not connect to OBS"

- Is OBS Studio open?
- Check **Tools → WebSocket Server Settings** → WebSocket server is enabled
- Verify port matches your config: `obs-pp-cli configure`
- Run diagnostics: `obs-pp-cli doctor`

### "Config not found"

```bash
obs-pp-cli configure --non-interactive --host localhost --port 4455 --password ""
```

### "Authentication failed"

- Open OBS → **Tools → WebSocket Server Settings**
- Either uncheck "Enable Authentication" or update your password
- Re-run: `obs-pp-cli configure`

### "Scene item not found after creation"

This can happen if a source name contains special characters. Use plain alphanumeric names.

## Commands Reference

| Command | Description |
|---------|-------------|
| `configure` | Save OBS WebSocket connection settings |
| `doctor` | Diagnose connection and configuration |
| `health` | Live system health snapshot |
| `scene list/current/switch/create` | Scene management |
| `stream start/stop/status` | Streaming control |
| `record start/stop/pause/status` | Recording control |
| `layout interview/solo/screenshare/brb` | Build production layouts |
| `guest add/remove/list` | VDO.Ninja guest sources |
| `preflight` | Pre-show readiness gate |
| `stats` | Performance metrics |
| `search` | Find scenes and sources by name |
| `export` | Export scene inventory to JSON |
| `profile list/current/switch` | OBS profile management |
| `deliver` | Post-production session delivery |
| `feedback` | Annotate or rate a production |

## Security

- Config stored in `~/.obs-pp/config.json` with `chmod 600`
- No credentials in source code or environment variables
- OBS WebSocket is local-only by default
- Enable authentication in OBS if exposing over a network

## License

Apache-2.0
