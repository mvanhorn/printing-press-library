# agent-capture Shipcheck Report

## Quality Gates: 7/7 PASS
1. go mod tidy - PASS
2. go vet - PASS
3. go build - PASS
4. binary build - PASS
5. --help - PASS
6. version - PASS
7. doctor - PASS (all 4 checks pass: platform, screen_recording, ffmpeg, swift)

## Functional Tests: 28/28 PASS
- version: 2/2 (text + json)
- health: 2/2 (text + json)
- doctor: 2/2 (text + json)
- permissions: 2/2 (text + json)
- list windows: 3/3 (text + json + app filter)
- list displays: 2/2 (text + json)
- find: 2/2 (text + json)
- screenshot: 3/3 (app target + json + code screenshot)
- batch: 1/1
- preset: 5/5 (save + list + show + list-json + delete)
- stitch: 1/1
- diff: 1/1
- error handling: 2/2 (unknown subcommand + missing target)

## Live Smoke Tests
- `list windows` returns 37 real windows with accurate metadata
- `list displays` returns 1 display with correct 2x Retina scale
- `screenshot --app "Finder"` produces 548KB PNG of real Finder window
- `record --app "Finder" --duration 3` produces 1.2MB MP4 of 3-second recording
- `convert` produces GIF from MP4 (with auto-reduce from 21MB to 4.3MB hitting 5MB limit)
- `pipeline --app "Finder" --duration 3` records + converts to GIF in one command
- `stitch` produces 488KB animated GIF from 2 screenshots
- `screenshot --code main.go --theme dracula` produces styled code screenshot with window chrome
- `find "chrome"` returns 7 matches with scores

## Known Gaps
- GIF auto-reduce uses nearest-neighbor resize (not lanczos) for speed. Quality is acceptable.
- OCR requires macOS Vision framework via Python. May not work if PyObjC is missing. Falls back gracefully.
- ANSI terminal rendering (--execute) not implemented.
- Audio capture not implemented (out of v1 scope).
- npm packaging not implemented (deferred).

## Ship Verdict: SHIP

The CLI has 17 working commands, all core capture features functional, code screenshot rendering working, and 28/28 tests passing. No critical gaps for v1.
