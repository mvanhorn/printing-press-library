# agent-capture

Record, screenshot, and convert macOS windows and screens for AI agent evidence.

Consolidates macOS screen capture, window recording, GIF conversion, frame stitching, and styled code screenshots into one agent-native CLI. Built on ScreenCaptureKit via Swift CoreGraphics bridge.

## Quick Start

```bash
go install github.com/mvanhorn/agent-capture-pp-cli/cmd/agent-capture@latest
agent-capture doctor
```

Grant Screen Recording permission when prompted (System Settings > Privacy & Security > Screen Recording).

```bash
# Screenshot an app
agent-capture screenshot --app "Preview" demo.png

# Record a window for 10 seconds
agent-capture record --app "Preview" --duration 10 demo.mp4

# Record and convert to GIF in one command
agent-capture pipeline --app "Preview" --duration 5 demo.gif

# Stitch screenshots into animated GIF
agent-capture stitch step1.png step2.png step3.png -o demo.gif
```

## Agent Usage

Every command supports `--json` for machine-parseable output.

```bash
# Discover capture targets
agent-capture list windows --json
agent-capture find "pull request" --json

# Screenshot with metadata
agent-capture screenshot --app "Finder" --json /tmp/shot.png

# Full evidence bundle
agent-capture evidence --app "Preview" --screenshots 3 --record 5 --output evidence/ --json

# Health check for CI preflight
agent-capture health --json
```

Exit codes: 0 = success, 1 = error.

## Health Check

```bash
agent-capture doctor
```

Checks:
- macOS platform
- Screen Recording permission
- ffmpeg availability (optional, for video recording)
- Swift availability (for window enumeration)

## Commands

| Command | Description |
|---------|-------------|
| `list` | List available windows and displays |
| `screenshot` | Capture a window, app, display, or region |
| `record` | Record video of a target |
| `convert` | Convert video to optimized GIF |
| `stitch` | Stitch screenshots into animated GIF |
| `pipeline` | Record + convert to GIF in one command |
| `evidence` | Full evidence bundle (screenshots + recording + GIF) |
| `find` | Fuzzy search window titles |
| `watch` | Interval screenshots for monitoring |
| `diff` | Visual diff against a baseline |
| `batch` | Screenshot multiple apps at once |
| `ocr` | Extract text from a window via Vision framework |
| `preset` | Save and load capture configurations |
| `doctor` | Check permissions and environment |
| `health` | Machine-readable health check |
| `permissions` | Permission setup guide |

## Troubleshooting

**"Screen Recording permission not granted"**

System Settings > Privacy & Security > Screen Recording > enable your terminal app. Restart the terminal completely after enabling.

**"listing windows" returns empty**

Run `agent-capture permissions` for step-by-step setup. The first time you grant Screen Recording permission, you must restart your terminal.

**GIF is too large**

Use `--max-size` to auto-reduce: `agent-capture convert demo.mp4 demo.gif --max-size 5mb`. The tool will skip frames and reduce dimensions until the file fits.

**ffmpeg not found**

Install with `brew install ffmpeg`. Required for video recording and video-to-GIF conversion. Not needed for screenshots or frame stitching.

## Cookbook

### PR Evidence Workflow

```bash
# Take 3 screenshots at 2-second intervals
agent-capture watch --app "MyApp" --interval 2 --count 3 --output shots/

# Stitch into a GIF
agent-capture stitch shots/frame-*.png -o evidence.gif --frame-duration 3

# Or do it all at once
agent-capture evidence --app "MyApp" --screenshots 3 --record 8 --output evidence/
```

### Save Common Settings

```bash
agent-capture preset save pr-evidence --duration 8 --fps 12 --width 640 --max-size 5mb
```

### Diff Before/After

```bash
agent-capture screenshot --app "MyApp" before.png
# ... make changes ...
agent-capture diff --before before.png --app "MyApp" --output diff.png
```

### Batch Multi-App Screenshots

```bash
agent-capture batch --apps "Finder,Safari,Terminal" --output screenshots/
```

## Requirements

- macOS 12.3+ (ScreenCaptureKit)
- Xcode Command Line Tools (`xcode-select --install`)
- ffmpeg (optional, for video recording): `brew install ffmpeg`

## Acknowledgments

Built with research from [Peekaboo](https://github.com/steipete/Peekaboo), [macosrec](https://github.com/xenodium/macosrec), [SwiftCapture](https://github.com/GlennWong/SwiftCapture), [freeze](https://github.com/charmbracelet/freeze), [termshot](https://github.com/homeport/termshot), and [Aperture](https://github.com/wulkano/Aperture).
