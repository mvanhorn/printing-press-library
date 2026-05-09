# obs-pp-cli

Control OBS Studio from your terminal or AI agent via its built-in WebSocket server.

Scenes, layouts, guest sources, streaming, recording, and preflight checks — all from one binary.

## Requirements

- [OBS Studio](https://obsproject.com/) 28 or newer (WebSocket server is built in)
- Go 1.21+ (for direct install) or `npx` (for Printing Press installer)

## Install

```bash
npx -y @mvanhorn/printing-press install obs
```

Or with Go directly:

```bash
go install github.com/mvanhorn/printing-press-library/library/streaming/obs/cmd/obs-pp-cli@latest
```

Add `$(go env GOPATH)/bin` to your PATH if not already there.

## OBS Setup

Enable the WebSocket server in OBS:

**Tools → WebSocket Server Settings → ✅ Enable WebSocket server → OK**

Default port: `4455`. Authentication is optional for local use.

## Configure

Run once to save your connection settings:

```bash
obs-pp-cli configure
```

Settings are stored in `~/.obs-pp/config.json` (chmod 600). No credentials in source code.

For agent/non-interactive use:

```bash
obs-pp-cli configure --non-interactive --host localhost --port 4455 --password ""
```

## Commands

### Scenes

```bash
obs-pp-cli scene list                   # List all scenes, mark current
obs-pp-cli scene current                # Print current scene name only
obs-pp-cli scene switch "Main Shot"     # Switch to a scene
obs-pp-cli scene create "New Scene"     # Create a new empty scene
```

### Streaming

```bash
obs-pp-cli stream start
obs-pp-cli stream stop
obs-pp-cli stream status                # Duration, bytes sent, dropped frames
```

### Recording

```bash
obs-pp-cli record start
obs-pp-cli record stop                  # Prints saved file path
obs-pp-cli record pause                 # Toggle pause/resume
obs-pp-cli record status
```

### Layouts

Build production-ready scenes programmatically. Each command creates a named scene with pre-positioned browser sources.

```bash
obs-pp-cli layout interview             # Two-person side-by-side (creates "Interview" scene)
obs-pp-cli layout interview --name "EP5" # Custom name
obs-pp-cli layout solo                  # Host fullscreen
obs-pp-cli layout screenshare          # Guest screen + host camera PiP
obs-pp-cli layout brb                  # Empty BRB scene
```

**Interview layout** (1920×1080):
- `Host Camera` — left half (960×1080 at 0,0)
- `Guest Camera` — right half (960×1080 at 960,0)

**Screen share layout:**
- `Guest Screen` — fullscreen background
- `Host Camera` — 320×180 PiP bottom-right

### Guest Sources (VDO.Ninja)

[VDO.Ninja](https://vdo.ninja/) is free browser-based WebRTC for remote guests. No account required.

```bash
# Add a guest (create or update browser source)
obs-pp-cli guest add "Jason" "https://vdo.ninja/?view=abc123&cleanoutput&transparent"

# Add to a specific scene
obs-pp-cli guest add "Jason" "<url>" --scene "Interview"

# List browser sources in current scene
obs-pp-cli guest list

# Remove a guest source
obs-pp-cli guest remove "Jason"
```

**VDO.Ninja quick links:**
- Guest push link (send to guest): `https://vdo.ninja/?push=ROOMID`
- View link (paste into `guest add`): `https://vdo.ninja/?view=ROOMID&cleanoutput&transparent`

### Preflight

```bash
obs-pp-cli preflight
```

Checks: OBS reachable, current scene has visible sources, mics not muted, stream/record not already running. Exits with code 1 if any check fails — useful for scripted show automation.

### Stats

```bash
obs-pp-cli stats                        # CPU, memory, FPS, frame drops, disk space
```

## Typical Show Flow

```bash
# Build the layout
obs-pp-cli layout interview

# Add your guest's VDO.Ninja view link
obs-pp-cli guest add "Guest Camera" "https://vdo.ninja/?view=GUESTID&cleanoutput&transparent"

# Preflight check
obs-pp-cli preflight

# Switch scene and go live
obs-pp-cli scene switch "Interview"
obs-pp-cli stream start
```

## Security

- Config stored in `~/.obs-pp/config.json` with `chmod 600`
- No credentials stored in source code or environment variables
- OBS WebSocket is local-only by default — keep it that way for personal use
- If exposing over a network, always enable authentication in OBS WebSocket settings

## License

Apache-2.0
