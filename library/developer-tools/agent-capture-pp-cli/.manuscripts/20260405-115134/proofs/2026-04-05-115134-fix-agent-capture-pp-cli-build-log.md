# agent-capture Build Log

## What Was Built

### P0 - Foundation
- Go CLI skeleton with cobra (17 commands)
- Swift CoreGraphics bridge for window/display enumeration (no PyObjC dependency)
- macOS `screencapture` integration for screenshots and recording
- JSON output mode on all commands

### P1 - Absorbed Features (44 features from 15 competitors)
- Window listing with app name, title, bundle ID, PID, bounds
- Display enumeration with resolution and scale factor
- Screenshot: by app name (fuzzy match), window ID, display, region
- Recording: duration-based, FPS control (15/30/60), cursor show/hide
- GIF conversion: two-pass palette, auto-reduce to hit size limits
- Frame stitching: normalization, padding, background color
- OCR via macOS Vision framework
- Batch multi-app screenshots
- Presets (save/load/list/delete)
- Doctor, health, permissions diagnostics

### P2 - Transcendence (10 novel features)
- Pipeline mode: record + convert to GIF in one command, zero intermediate files
- Evidence bundle: screenshots + recording + GIF in one invocation
- Smart window finder: fuzzy search across titles and app names
- Watch mode: interval screenshots for monitoring
- Diff capture: visual regression against baseline
- Code screenshots: silicon replacement using chroma + gg (Dracula, Nord, Monokai, GitHub, Solarized themes)
- Window chrome, drop shadow, line numbers, rounded corners
- Permission doctor with deep links to System Settings
- Health check with machine-readable JSON for CI preflight
- Format auto-detect from output extension

## Technology Stack
- Go + cobra (CLI framework)
- Swift CoreGraphics (window enumeration via subprocess)
- macOS screencapture (screenshots and recording)
- ffmpeg (video frame extraction for GIF conversion)
- alecthomas/chroma/v2 (syntax highlighting)
- fogleman/gg (2D rendering for code screenshots)
- Go stdlib image/gif (GIF encoding)

## What Was Intentionally Deferred
- Audio capture (out of v1 scope per plan)
- npm packaging (Unit 8 from plan, deferred to post-shipcheck)
- ScreenCaptureKit direct integration (using screencapture CLI for now, SCK can be added later)
- Video codec selection (h264/hevc flag exists but screencapture handles it)
- ANSI terminal rendering (--execute flag, needs terminal emulator)

## Generator Limitations
- This was a plan-driven run, not spec-driven. No `printing-press generate` was used.
- The CLI was scaffolded manually following PP conventions.
- dogfood/verify/scorecard may have limited applicability without an OpenAPI spec.
