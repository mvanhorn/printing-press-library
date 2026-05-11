---
name: pp-obs
description: "Use this skill whenever the user wants to control OBS Studio — switching scenes, starting/stopping streams or recordings, building interview or screen-share layouts, managing VDO.Ninja guest sources, running preflight checks, checking OBS performance, or managing production delivery and feedback. Triggers on phrases like 'switch to interview scene', 'start recording', 'add a guest', 'set up my show layout', 'preflight check', 'is OBS ready', 'stop the stream', 'how is OBS performing', 'show OBS health', 'save this session', 'log feedback'."
author: "Ekello Harrid"
license: "Apache-2.0"
argument-hint: "<command> [args]"
allowed-tools: "Bash"
metadata:
  openclaw:
    requires:
      bins:
        - obs-pp-cli
    install:
      - kind: go
        bins: [obs-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/media-and-entertainment/obs/cmd/obs-pp-cli
---

# OBS Studio — Printing Press CLI

## Prerequisites: OBS Setup

1. OBS Studio 28 or newer must be installed and running
2. Enable the WebSocket server: **OBS → Tools → WebSocket Server Settings → Enable WebSocket server → OK**
3. Default port: 4455. Authentication is optional for local use.

## Prerequisites: Install the CLI

This skill drives the `obs-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install obs --cli-only
   ```
2. Verify: `obs-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/obs/cmd/obs-pp-cli@latest
```

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

## First-Time Configuration

Run once to save your OBS connection settings:

```bash
obs-pp-cli configure --non-interactive --host localhost --port 4455 --password ""
```

Or interactive:
```bash
obs-pp-cli configure
```

Config stored in `~/.obs-pp/config.json` (chmod 600). No credentials in source code.

## Diagnostics

```bash
obs-pp-cli doctor
obs-pp-cli health
```

## Command Reference

### Scenes
```bash
obs-pp-cli scene list                    # List all scenes, mark current
obs-pp-cli scene current                 # Print current scene name
obs-pp-cli scene switch "Main Shot"      # Switch to a scene
obs-pp-cli scene create "New Scene"      # Create a new empty scene
obs-pp-cli scene list --format json      # JSON output
```

### Streaming
```bash
obs-pp-cli stream start
obs-pp-cli stream stop
obs-pp-cli stream status
obs-pp-cli stream status --format json
```

### Recording
```bash
obs-pp-cli record start
obs-pp-cli record stop
obs-pp-cli record pause
obs-pp-cli record status
```

### Layouts
```bash
obs-pp-cli layout interview --name "EP5"
obs-pp-cli layout solo
obs-pp-cli layout screenshare
obs-pp-cli layout brb
obs-pp-cli layout interview --dry-run
```

### Guest Sources (VDO.Ninja)
```bash
obs-pp-cli guest list
obs-pp-cli guest add --source "Jason" --url "https://vdo.ninja/?view=ROOMID&cleanoutput&transparent"
obs-pp-cli guest add --source "Jason" --url "https://vdo.ninja/?view=ROOMID&cleanoutput&transparent" --scene "Interview"
obs-pp-cli guest remove --source "Jason"
```

### Health, Stats, and Performance
```bash
obs-pp-cli health
obs-pp-cli stats
obs-pp-cli trends
obs-pp-cli trends --samples 5 --interval 2
```

### Search and Export
```bash
obs-pp-cli search --query "interview"
obs-pp-cli search --query "camera" --type source
obs-pp-cli export
obs-pp-cli export --scene "Interview"
```

### Profiles
```bash
obs-pp-cli profile list
obs-pp-cli profile current
obs-pp-cli profile switch "Streaming"
```

### Preflight
```bash
obs-pp-cli preflight
obs-pp-cli preflight --format json
```

### Production Delivery and Feedback
```bash
obs-pp-cli deliver create "Episode 5"
obs-pp-cli deliver list
obs-pp-cli feedback add --rating 4 --notes "Great energy, mic dipped"
obs-pp-cli feedback list
obs-pp-cli analytics
```

## Live Show Flow

```bash
# 1. Build the layout
obs-pp-cli layout interview --name "EP05"

# 2. Add guest feed
obs-pp-cli guest add --source "Guest Camera" --url "https://vdo.ninja/?view=GUESTID&cleanoutput&transparent" --scene "EP05"

# 3. Preflight
obs-pp-cli preflight

# 4. Go live
obs-pp-cli scene switch "EP05"
obs-pp-cli stream start
obs-pp-cli record start

# 5. Wrap
obs-pp-cli stream stop
obs-pp-cli record stop

# 6. Log session
obs-pp-cli deliver create "Episode 5"
obs-pp-cli feedback add --rating 4
```

## VDO.Ninja Quick Reference

- **Guest push link** (send to guest): `https://vdo.ninja/?push=ROOMID`
- **View link** (add to OBS): `https://vdo.ninja/?view=ROOMID&cleanoutput&transparent`
- Generate a room at: https://vdo.ninja/

## Agent Usage

```bash
# All commands support JSON output
obs-pp-cli --agent scene list
obs-pp-cli --agent health
obs-pp-cli --agent preflight && obs-pp-cli --agent stream start

# Dry-run to preview changes
obs-pp-cli --dry-run layout interview
obs-pp-cli --dry-run scene switch "BRB"
```

Exit codes:
- 0 = success
- 1 = OBS connection error
- 2 = resource not found
- 3 = config missing
- 4 = invalid argument
- 5 = preflight check failed
