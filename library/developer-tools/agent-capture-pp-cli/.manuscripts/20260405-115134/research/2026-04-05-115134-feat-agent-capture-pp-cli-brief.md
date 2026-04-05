# agent-capture CLI Brief

## API Identity
- Domain: macOS native screen/window capture (ScreenCaptureKit)
- Users: AI agents (primarily OSC evidence pipeline), developers who need programmatic screen recording
- Data profile: No persistent data layer needed. Stateless capture tool. Outputs: PNG, JPG, MP4, MOV, GIF. Input: macOS window/display/app targets.

## Reachability Risk
- None. Local macOS API, no network dependency. ScreenCaptureKit is available macOS 12.3+, screenshots via SCScreenshotManager on macOS 14+. Most developer Macs are on 14+.

## Top Workflows
1. **Evidence capture for PRs**: Record a native app window for 5-10s, convert to GIF, attach to PR
2. **Screenshot-reel stitching**: Take 3-5 screenshots of different states, stitch into animated GIF
3. **Code screenshot generation**: Render source code as styled PNG (silicon replacement)
4. **Window discovery**: Agent lists available windows to find the right capture target
5. **Full-screen demo recording**: Record entire display for longer-form demos

## Table Stakes (Features Every Competitor Has)
- Window capture by app name or window ID
- Display/screen capture (full or region)
- List windows and displays
- PNG/JPG screenshot output
- MP4/MOV video recording with duration control
- FPS control (15/30/60)
- GIF conversion from video
- Cursor show/hide
- JSON output for machine parsing
- Delay/countdown before capture
- Retina/2x support

## Data Layer
- No SQLite store needed. This is a stateless capture tool.
- Potential future: capture history log, but out of scope for v1.

## User Vision
- Consolidate 6+ fragmented tools (ffmpeg avfoundation, silicon, Pillow, ffmpeg palette+concat) into one binary
- Drop-in replacement for underlying tool calls in the OSC capture-evidence.py pipeline
- Native macOS window targeting that ffmpeg can't do (specific windows, not just full screen)
- Go implementation so the printing press pipeline works end-to-end

## Go + ScreenCaptureKit Bridge Assessment
- **No production-ready Go library exists** for ScreenCaptureKit
- **tfsoares/screencapturekit-go**: Subprocess bridge (shells out to a compiled Swift CLI). Works today but adds a Swift binary dependency.
- **progrium/darwinkit PR #279**: Proper cgo bindings for ScreenCaptureKit. Open since March 2025, not merged (blocked on code generation concerns).
- **Recommended approach**: Use tfsoares/screencapturekit-go as the initial bridge (proven, works today), with option to swap to darwinkit bindings when PR merges. Avoids writing 300-400 lines of custom cgo+ObjC bridge code.
- **Fallback for screenshots**: kbinani/screenshot uses CGDisplayCreateImage (older API, still works). Good enough for display-level screenshots.

## Go Library Stack
- CLI: cobra (PP standard)
- ScreenCaptureKit: tfsoares/screencapturekit-go (subprocess bridge)
- Screenshots fallback: kbinani/screenshot (CGDisplayCreateImage)
- GIF encoding: image/gif stdlib + ericpauley/go-quantize (palette) + nathanbaulch/gifx (frame delta)
- Video frames: u2takey/ffmpeg-go (shell to ffmpeg for MP4 decode)
- Code screenshots: alecthomas/chroma (syntax highlighting) + fogleman/gg (2D rendering)
- Image processing: disintegration/imaging (resize, crop)

## Competitive Landscape
- **Peekaboo** (3,100 stars, Swift): The 800-pound gorilla. Full desktop automation + MCP server + AI vision. Way broader scope than agent-capture (mouse/keyboard automation, app control, agent loop). We don't compete on automation; we compete on capture quality, GIF pipeline, and code screenshots.
- **macosrec** (150 stars, Swift): Closest competitor for our scope. Screenshots, recording, GIF output, OCR. But no code screenshots, no stitching, no agent-native flags.
- **SwiftCapture** (7 stars, Swift): Clean ScreenCaptureKit CLI. Presets, quality control, audio. No GIF, no code screenshots, no stitching.
- **freeze** (4,400 stars, Go): Code screenshot tool. Our code screenshot mode competes directly. We beat it by also doing screen capture and GIF in the same binary.
- **termshot** (747 stars, Go): Terminal ANSI screenshot renderer. Niche but relevant for our code screenshot mode.

## Product Thesis
- Name: agent-capture
- Why it should exist: AI agents need to capture native macOS windows as evidence. Today that requires 6+ tools with fragile shell pipelines. agent-capture consolidates capture, screenshot, GIF conversion, frame stitching, and styled code rendering into one binary with --json output and agent-native flags. It's the missing complement to agent-browser (which handles browser/CDP) and VHS (which handles terminal recording).

## Build Priorities
1. P0: CLI skeleton + ScreenCaptureKit bridge (list, screenshot, record)
2. P1: All absorbed features from competitor audit (GIF convert, stitch, region capture, retina, cursor, countdown, format options, clipboard)
3. P2: Transcendence features (code screenshots, pipeline mode, permission doctor, capture presets)
