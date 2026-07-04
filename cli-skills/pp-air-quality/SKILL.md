---
name: pp-air-quality
description: "Use OpenAQ and AirNow source-backed recipes for air-quality snapshots, nearby monitors, sensor history, point comparison, and AirNow AQI setup/current observations. Trigger phrases: air quality, OpenAQ, AirNow, AQI, PM2.5, nearby monitor, air-quality-pp-cli."
author: "Dhilip Subramanian"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - air-quality-pp-cli
    install:
      - kind: go
        bins: [air-quality-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/other/air-quality/cmd/air-quality-pp-cli
---
<!-- GENERATED FILE — DO NOT EDIT.
     This file is a verbatim mirror of library/other/air-quality/SKILL.md,
     regenerated post-merge by tools/generate-skills/. Hand-edits here are
     silently overwritten on the next regen. Edit the library/ source instead.
     See the repository agent guide, section "Generated artifacts: registry.json, cli-skills/". -->

# Air Quality - Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `air-quality-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install air-quality --cli-only
   ```
2. Verify: `air-quality-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.4 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/other/air-quality/cmd/air-quality-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

Security note: this module declares `toolchain go1.26.4` so environments with Go toolchain auto-download disabled should install Go 1.26.4 or newer before using the direct `go install` fallback.

## When To Use

Use `air-quality-pp-cli` when an agent needs source-backed air-quality context:

- Find nearby OpenAQ monitoring locations.
- Fetch latest OpenAQ physical pollutant measurements near coordinates.
- Fetch latest measurements for a known OpenAQ location.
- Fetch a bounded recent measurement window for one OpenAQ sensor.
- Compare two coordinate pairs using nearest OpenAQ snapshots.
- Use AirNow current AQI observations by ZIP when an AirNow API key is configured.

## When Not To Use

- Do not use it for medical advice, emergency guidance, or regulatory decisions.
- Do not convert OpenAQ physical measurements into AQI.
- Do not use web-service commands for broad AirNow database population.

## Setup

OpenAQ live commands require:

```bash
export AIR_QUALITY_OPENAQ_API_KEY="..."
```

AirNow commands require:

```bash
export AIR_QUALITY_AIRNOW_API_KEY="..."
```

## Recipes

### Current OpenAQ Snapshot

```bash
air-quality-pp-cli current --lat 40.7128 --lon -74.0060 --agent
```

Use this to find the nearest OpenAQ location and latest physical pollutant measurements.

### Nearby OpenAQ Locations

```bash
air-quality-pp-cli nearest --lat 40.7128 --lon -74.0060 --agent
```

Use this when an agent needs candidate locations, sensors, and provider metadata.

### Known OpenAQ Location

```bash
air-quality-pp-cli location 2178 --agent
```

Use this when a workflow already has an OpenAQ location ID.

### Sensor History

```bash
air-quality-pp-cli history --sensor 3917 --days 7 --agent
```

Use this for a bounded recent measurement window. The CLI caps the window at 31 days.

### Compare Two Points

```bash
air-quality-pp-cli compare --lat-a 40.7128 --lon-a -74.0060 --lat-b 34.0522 --lon-b -118.2437 --agent
```

Use this for source-backed comparison prose. Treat it as a snapshot, not exposure or health guidance.

### AirNow Current AQI

```bash
air-quality-pp-cli airnow current --zip 10001 --agent
```

Use this only when `AIR_QUALITY_AIRNOW_API_KEY` is configured.

### Source Coverage

```bash
air-quality-pp-cli sources --agent
air-quality-pp-cli doctor --agent
```

Use these before a larger workflow to confirm which source families are configured.
