---
name: pp-obs
description: "Use this skill whenever the user wants to control OBS Studio — switching scenes, starting/stopping streams or recordings, building interview or screen-share layouts, managing VDO.Ninja guest sources, running preflight checks, or checking OBS performance stats. Triggers on phrases like 'switch to interview scene', 'start recording', 'add a guest', 'set up my show layout', 'preflight check', 'is OBS ready', 'stop the stream', 'how's OBS performing'."
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
        module: github.com/mvanhorn/printing-press-library/library/streaming/obs/cmd/obs-pp-cli
---

# OBS Studio — Printing Press CLI

## Prerequisites: OBS Setup

1. OBS Studio 28 or newer must be installed and running
2. Enable the WebSocket server: **OBS → Tools → WebSocket Server Settings → Enable WebSocket server → OK**
3. Note your port (default: 4455) and password if authentication is enabled

## Prerequisites: Install the CLI

```bash
npx -y @mvanhorn/printing-press install obs
```

Or install directly with Go:
```bash
go install github.com/mvanhorn/printing-press-library/library/streaming/obs/cmd/obs-pp-cli@latest
```

Verify: `obs-pp-cli --version`

## First-Time Configuration

Run once to save your OBS connection settings (stored in `~/.obs-pp/config.json`):

```bash
obs-pp-cli configure
```

For non-interactive / agent use:
```bash
obs-pp-cli configure --non-interactive --host localhost --port 4455 --password ""
```

## Command Reference

### Scenes
```bash
obs-pp-cli scene list                    # List all scenes, mark current
obs-pp-cli scene current                 # Print current scene name
obs-pp-cli scene switch "Main Shot"      # Switch to a scene
obs-pp-cli scene create "New Scene"      # Create a new empty scene
```

### Streaming
```bash
obs-pp-cli stream start                  # Start streaming
obs-pp-cli stream stop                   # Stop streaming
obs-pp-cli stream status                 # Show live/duration/bytes
```

### Recording
```bash
obs-pp-cli record start                  # Start recording
obs-pp-cli record stop                   # Stop and print saved file path
obs-pp-cli record pause                  # Toggle pause/resume
obs-pp-cli record status                 # Show record state/duration
```

### Layouts (build scenes programmatically)
```bash
obs-pp-cli layout interview              # Two-person side-by-side (creates "Interview" scene)
obs-pp-cli layout interview --name "EP5" # Custom scene name
obs-pp-cli layout solo                   # Host fullscreen
obs-pp-cli layout screenshare            # Guest screen + host PiP
obs-pp-cli layout brb                    # Empty BRB scene
```

### Guest Sources (VDO.Ninja)
```bash
obs-pp-cli guest list                    # List browser sources in current scene
obs-pp-cli guest add "Jason" "https://vdo.ninja/?view=abc123&cleanoutput&transparent"
obs-pp-cli guest remove "Jason"
obs-pp-cli guest add "Jason" <url> --scene "Interview"   # Target a specific scene
```

### Preflight & Stats
```bash
obs-pp-cli preflight                     # Pre-show readiness check (exits 1 if issues)
obs-pp-cli stats                         # CPU, FPS, memory, frame drops
```

## Typical Show Setup Flow

```bash
# 1. Build the layout
obs-pp-cli layout interview

# 2. Add your guest's VDO.Ninja link
obs-pp-cli guest add "Guest Camera" "https://vdo.ninja/?view=GUESTID&cleanoutput&transparent"

# 3. Run preflight
obs-pp-cli preflight

# 4. Switch to the scene and go live
obs-pp-cli scene switch "Interview"
obs-pp-cli stream start
```

## VDO.Ninja Quick Reference

VDO.Ninja is free browser-based WebRTC for remote guests. No account required.

- **Guest push link** (send to guest): `https://vdo.ninja/?push=ROOMID`
- **View link** (put in OBS): `https://vdo.ninja/?view=ROOMID&cleanoutput&transparent`
- Generate a room at: https://vdo.ninja/

## Notes

- Config is stored in `~/.obs-pp/config.json` (chmod 600). No credentials in source.
- OBS must be running for any command to work.
- Layout commands create new scenes without affecting your existing ones.
- `preflight` exits with code 1 if checks fail — useful for scripted show automation.
