# Air Quality CLI

`air-quality-pp-cli` is a source-aware Printing Press CLI for air-quality monitoring recipes. It keeps OpenAQ physical pollutant measurements separate from AirNow AQI observations and returns compact `--agent` JSON for research agents.

## Sources

- OpenAQ API v3: physical pollutant measurements, locations, sensors, and recent history.
- AirNow API web services: AQI observations and forecasts for configured AirNow accounts.

## Setup

OpenAQ live commands require:

```bash
export AIR_QUALITY_OPENAQ_API_KEY="..."
```

AirNow commands require:

```bash
export AIR_QUALITY_AIRNOW_API_KEY="..."
```

If a key is missing, auth-gated commands return structured setup guidance instead of fake data.

For direct Go installation, use Go 1.26.4 or newer when Go toolchain auto-download is disabled.

## Commands

```bash
air-quality-pp-cli current --lat 40.7128 --lon -74.0060 --agent
air-quality-pp-cli nearest --lat 40.7128 --lon -74.0060 --agent
air-quality-pp-cli location 2178 --agent
air-quality-pp-cli history --sensor 3917 --days 7 --agent
air-quality-pp-cli compare --lat-a 40.7128 --lon-a -74.0060 --lat-b 34.0522 --lon-b -118.2437 --agent
air-quality-pp-cli airnow current --zip 10001 --agent
air-quality-pp-cli sources --agent
air-quality-pp-cli doctor --agent
```

## Caveats

- OpenAQ returns physical pollutant measurements in source units, not AQI categories.
- AirNow observations are preliminary and subject to change.
- AirNow data are intended for AQI reporting and forecasting, not regulatory decisions.
- This CLI does not provide medical advice, emergency guidance, or regulatory support.
