# OBS Studio CLI — Research Brief

**Date:** 2026-05-10  
**CLI:** obs-pp-cli  
**Protocol:** OBS WebSocket v5 (not REST)

## API Overview

OBS Studio 28+ ships with a built-in WebSocket server (port 4455). The protocol is documented at:  
https://github.com/obsproject/obs-websocket/blob/master/docs/generated/protocol.md

This is a **WebSocket protocol**, not REST/OpenAPI. The Go library `github.com/andreykaipov/goobs` (v1.8.3) provides type-safe access to all OBS WebSocket v5 requests.

## Problem Being Solved

**Goal:** Replace StreamYard-style live production workflows with a fully local, programmable setup using OBS Studio + VDO.Ninja.

**Use case:** Interview shows with remote guests, solo productions, screenshare presentations — all controlled via CLI commands that agents can automate.

## Key Design Decisions

### VDO.Ninja for Guest Video
- Free, browser-based WebRTC — no accounts, no recurring cost
- Guest gets a `?push=ROOMID` link (no install needed)
- OBS gets a `?view=ROOMID&cleanoutput&transparent` browser source URL
- CLI manages room ID generation, URL construction, and browser source lifecycle

### Production Layouts
Built using `SetSceneItemTransform` with `OBS_BOUNDS_STRETCH`:
- **Interview:** Host camera 960×1080 left, Guest camera 960×1080 right
- **Solo:** Single fullscreen camera
- **Screenshare:** Full guest screen + host PiP at 320×180
- **BRB:** Static BRB image fullscreen

### Auth
OBS WebSocket authentication is optional. CLI stores config in `~/.obs-pp/config.json` (chmod 600). Password field left empty when auth is disabled.

## Alternatives Analyzed

| Tool | Language | Stars | OBS WS v5 | Guest Mgmt | Layout Builder |
|------|----------|-------|-----------|------------|----------------|
| obs-cli | TypeScript | 312 | ❌ (v4 only) | ❌ | ❌ |
| obs-cmd | Rust | 264 | ✅ | ❌ | ❌ |
| go-obs-websocket | Go | 88 | ❌ (v4) | ❌ | ❌ |

**Novelty score: 9/10.** No existing CLI combines WebSocket v5 support, VDO.Ninja guest management, production layout building, and agent-optimized JSON output.

## Recommendation

**Proceed.** obs-pp-cli fills a real gap for programmatic live production workflows. The combination of VDO.Ninja guest feeds + one-command layout building is not available in any existing tool.
