# agent-capture Absorb Manifest

## Sources Cataloged
1. macOS `screencapture` (built-in)
2. Peekaboo (3,100 stars, Swift) - desktop automation + MCP
3. SwiftCapture/scap (7 stars, Swift) - ScreenCaptureKit CLI
4. macosrec (150 stars, Swift) - recording + OCR
5. freeze (4,400 stars, Go) - code screenshots
6. termshot (747 stars, Go) - terminal screenshots
7. carbon-now-cli (6,000 stars, TypeScript) - code screenshots via Carbon
8. Aperture (1,300 stars, Swift) - recording library
9. alexdelorenzo/screenshot (188 stars, Python) - window screenshots
10. node-mac-recorder (25 stars, JS/C++) - ScreenCaptureKit Node bindings
11. macos-screen-mcp (13 stars, Python) - MCP screenshot server
12. mcp-desktop-pro (7 stars, JS) - MCP desktop capture
13. terminalizer (16,100 stars, JS) - terminal recording
14. gifski (8,400 stars, Swift/Rust) - high-quality GIF encoder
15. Kap (19,200 stars, TypeScript) - screen recorder app

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-----------|-------------------|-------------|
| 1 | List windows | Peekaboo `list windows`, macosrec `-l`, SwiftCapture `--app-list` | `agent-capture list windows` | --json with bounds, bundle ID, PID; filter by --app name |
| 2 | List displays | SwiftCapture `--screen-list`, screencapture `-D` | `agent-capture list displays` | --json with resolution, scale factor, primary flag |
| 3 | Screenshot window by app name | Peekaboo `see --app`, macosrec `-x`, alexdelorenzo `-t` | `agent-capture screenshot --app "Preview"` | Fuzzy app name match, --json metadata output |
| 4 | Screenshot window by ID | screencapture `-l`, macosrec | `agent-capture screenshot --window-id 12345` | Same + --json metadata |
| 5 | Screenshot display | screencapture, SwiftCapture `--screen` | `agent-capture screenshot --display 1` | Multi-display aware, --json |
| 6 | Screenshot region | screencapture `-R`, SwiftCapture `--area` | `agent-capture screenshot --region 0,0,800,600` | Agent-friendly coordinate syntax |
| 7 | Record window by app | SwiftCapture `-A`, macosrec `-r`, Aperture | `agent-capture record --app "Preview" --duration 10` | Duration-based (no start/stop dance), --json progress |
| 8 | Record window by ID | SwiftCapture, node-mac-recorder | `agent-capture record --window-id 12345 --duration 10` | Same |
| 9 | Record display | SwiftCapture `-s`, screencapture `-v`, Aperture | `agent-capture record --display 1 --duration 30` | Same |
| 10 | Record region | SwiftCapture `--area`, node-mac-recorder `captureArea` | `agent-capture record --region 0,0,800,600 --duration 10` | Combined with window/display targeting |
| 11 | FPS control | SwiftCapture `--fps`, Aperture `framesPerSecond` | `--fps 15\|30\|60` (default 15) | Lower default for smaller files (agent use case) |
| 12 | Video format MP4 | Kap, macshot, screencapture | `--format mp4` | Default output format |
| 13 | Video format MOV | SwiftCapture, macosrec `--mov`, Aperture | `--format mov` | Native AVFoundation write |
| 14 | GIF conversion | macosrec `--gif`, gifski, ffmpeg pipeline | `agent-capture convert input.mp4 output.gif` | Built-in two-pass palette, no ffmpeg needed for GIF |
| 15 | GIF max size control | gifski quality slider, capture-evidence.py | `--max-size 10mb` | Auto-reduce quality/fps/dimensions to hit target |
| 16 | GIF FPS control | gifski, capture-evidence.py | `--fps 12` on convert | Independent from recording FPS |
| 17 | GIF width control | gifski dimensions, capture-evidence.py | `--width 640` on convert | Auto aspect ratio |
| 18 | Frame stitching | capture-evidence.py browser-reel/screenshot-reel | `agent-capture stitch frame1.png frame2.png -o out.gif` | Built-in normalization, padding, two-pass palette |
| 19 | Frame duration control | capture-evidence.py | `--frame-duration 3.0` on stitch | Per-frame or uniform |
| 20 | Frame background padding | capture-evidence.py (Pillow) | `--background white\|black\|transparent` on stitch | Replaces Pillow dependency |
| 21 | Cursor show/hide | SwiftCapture `--show-cursor`, screencapture `-C` | `--cursor` / `--no-cursor` | Default: no cursor (agent evidence is cleaner) |
| 22 | Click highlighting | Aperture `highlightClicks`, screencapture `-k` | `--highlight-clicks` | Visual feedback in recordings |
| 23 | Countdown/delay | SwiftCapture `--countdown`, screencapture `-T` | `--countdown 3` | Countdown with progress in --json mode |
| 24 | Retina/2x support | Peekaboo `--retina`, screencapture | `--retina` (default: true on Retina displays) | Auto-detect, explicit override |
| 25 | Clipboard output | screencapture `-c`, termshot `--clipboard` | `--clipboard` on screenshot | Copy to clipboard instead of file |
| 26 | Shadow control | screencapture `-o`, alexdelorenzo `--shadow` | `--shadow` / `--no-shadow` | Default: no shadow (cleaner for evidence) |
| 27 | Output format PNG | screencapture, freeze, everyone | `--format png` (default for screenshot) | Standard |
| 28 | Output format JPG | screencapture, screenshot-desktop | `--format jpg` | With quality control |
| 29 | JSON output mode | Peekaboo `--json`, SwiftCapture `--json` | `--json` global flag | Every command, structured output for agents |
| 30 | Permission check | Peekaboo `permissions status`, node-mac-recorder | `agent-capture permissions` | Check + clear instructions to fix |
| 31 | Quality presets | SwiftCapture `--quality low\|medium\|high`, node-mac-recorder | `--quality low\|medium\|high` | Maps to fps + resolution + bitrate |
| 32 | Video codec selection | Aperture `.h264/.hevc/.proRes422/.proRes4444` | `--codec h264\|hevc` | H.264 default, HEVC for smaller files |
| 33 | Overwrite control | SwiftCapture `--force` | `--force` to overwrite existing output | Safe default: error if file exists |
| 34 | Code screenshot (styled) | freeze, carbon-now-cli, silicon | `agent-capture screenshot --code file.py --theme dracula` | Built-in, no external dependency, same binary |
| 35 | Code theme selection | freeze `-t`, carbon-now-cli, silicon | `--theme dracula\|nord\|monokai\|github\|solarized` | 5+ themes via chroma |
| 36 | Code line numbers | freeze `--show-line-numbers`, silicon | `--line-numbers` | On by default for code screenshots |
| 37 | Code window chrome | freeze `-w`, silicon | `--window-chrome` | macOS-style traffic lights |
| 38 | Code font control | freeze `--font.family/size`, carbon-now-cli | `--font-family`, `--font-size` | JetBrains Mono default |
| 39 | Code language override | freeze `-l`, chroma | `--lang rust` | Auto-detect from extension, override available |
| 40 | Code from stdin | silicon, freeze | `--code --stdin --lang python` | Pipe code from other commands |
| 41 | Code padding/margin | freeze `-p/-m`, silicon | `--padding`, `--margin` | Configurable spacing |
| 42 | Code shadow | freeze `--shadow.*`, termshot | `--shadow` on code screenshots | Drop shadow with configurable blur |
| 43 | Terminal ANSI rendering | termshot, freeze `--execute` | `agent-capture screenshot --execute "ls --color"` | Run command, capture styled output as image |
| 44 | OCR text extraction | macosrec `--ocr`, Peekaboo AI vision | `agent-capture ocr --app "Preview"` | Extract text from window via macOS Vision framework |

## Transcendence (only possible with our consolidated tool)

| # | Feature | Command | Why Only We Can Do This |
|---|---------|---------|------------------------|
| T1 | Pipeline mode | `agent-capture pipeline --app "Preview" --duration 5 --gif --max-size 5mb` | Record + convert + optimize in one command. No intermediate files. Other tools require 3 separate invocations (record, convert, optimize). |
| T2 | Evidence bundle | `agent-capture evidence --app "Preview" --screenshots 3 --record 5 --output evidence/` | Takes N screenshots at intervals, records a video, converts to GIF, bundles all outputs. One command replaces an entire capture-evidence.py invocation for simple cases. |
| T3 | Smart window finder | `agent-capture find "pull request"` | Fuzzy search across all window titles. Returns best match with window ID, app name, bounds. Agents don't know window IDs; they know what they're looking for. |
| T4 | Permission doctor | `agent-capture doctor` | Checks all required permissions (Screen Recording, Accessibility), detects common issues (wrong terminal authorized, permission granted but not refreshed), provides exact fix steps with deep links to System Settings. Other tools just say "permission denied." |
| T5 | Watch mode | `agent-capture watch --app "Preview" --interval 2 --output frames/ --count 10` | Take screenshots at regular intervals. For monitoring UI state changes during automated tests. No competing CLI does interval capture. |
| T6 | Diff capture | `agent-capture diff --before before.png --app "Preview" --output diff.png` | Take screenshot, diff against a baseline, highlight changed regions. For visual regression evidence. |
| T7 | Capture presets | `agent-capture preset save pr-evidence --duration 8 --gif --max-size 5mb --fps 12 --width 640` | Save and recall capture configurations. The OSC pipeline always uses the same settings; presets avoid re-specifying 6 flags. SwiftCapture has presets but not for the full pipeline. |
| T8 | Format auto-detect | Output path determines format: `.gif` = record+convert, `.png` = screenshot, `.mp4` = record | `agent-capture --app "Preview" --duration 5 demo.gif` does record+convert automatically. Fewer flags, smarter defaults. |
| T9 | Batch capture | `agent-capture batch --apps "Finder,Preview,Terminal" --output shots/` | Screenshot multiple apps in one command. For multi-app evidence where you need to show several windows. |
| T10 | Health check | `agent-capture health` | Verify: ScreenCaptureKit available, permissions granted, ffmpeg found (optional), disk space adequate. Machine-readable --json output for CI/agent preflight. |
