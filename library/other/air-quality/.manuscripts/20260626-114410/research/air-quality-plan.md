# Air Quality Research Plan

## Goal

Build a read-only Printing Press CLI for air-quality monitoring and location comparison workflows using OpenAQ API v3 and AirNow API web services.

## Sources

OpenAQ provides an official v3 REST API and official OpenAPI document at `https://api.openaq.org/openapi.json`. OpenAQ v1 and v2 were retired on January 31, 2025, so this print must use v3 only. OpenAQ requires an `X-API-Key` header; users sign up through OpenAQ Explorer. The documented free rate limit is 60 requests per minute and 2,000 requests per hour. OpenAQ shares physical pollutant measurements and metadata, not AQI categories.

AirNow provides U.S., Canada, and Mexico AQI observations and forecasts through AirNow API web services. AirNow requires an account/API key. The public documentation lists current forecasts, current observations by zip code or latitude/longitude, observations by monitoring site, historical observations, contour maps, data feeds, and file products. AirNow warns that reported data are preliminary, subject to change, intended for reporting/forecasting AQI, and not suitable for regulatory decisions.

The AirNow Data Exchange Guidelines, last updated August 2025, apply to AirNow API data. They state that AirNow observational data are not fully verified or validated, should be treated as preliminary, and should not be used to formulate or support regulation, trends, guidance, government decisions, or public decision-making.

## Command Surface

The first print keeps the surface intentionally small and agent-native:

- `current --lat <lat> --lon <lon>` finds nearby OpenAQ locations, fetches latest measurements for the best location, and returns a compact pollutant snapshot.
- `nearest --lat <lat> --lon <lon>` lists nearby OpenAQ monitoring locations with coordinates, providers, sensors, and available parameters.
- `location <openaq-location-id>` fetches a known OpenAQ location and its latest measurements.
- `history --sensor <sensor-id> --days <n>` fetches OpenAQ measurements for one sensor over a bounded recent window.
- `compare --lat-a <lat> --lon-a <lon> --lat-b <lat> --lon-b <lon>` compares nearest OpenAQ snapshots for two points.
- `airnow current --zip <zip>` calls AirNow current observations when `AIR_QUALITY_AIRNOW_API_KEY` is configured; without a key it returns setup guidance.
- `sources` reports source coverage, auth mode, limits, and caveats.
- `doctor` reports configured environment variables and which command families can run.

## Live Research Findings

- OpenAQ v3 official docs confirm the API is REST/JSON, covers criteria pollutants, and requires v3 because v1/v2 are retired.
- OpenAQ official docs state API keys are sent via the `X-API-Key` header.
- OpenAQ official docs state free rate limits of 60 requests per minute and 2,000 requests per hour.
- OpenAQ locations support coordinate and radius filters with a maximum radius of 25,000 meters.
- OpenAQ latest measurement endpoints exist by parameter ID and by location ID.
- AirNow web service docs list current forecasts, current observations, historical observations, and contour maps.
- AirNow fact sheet states an AirNow API account is required and web-service data can be returned as CSV, JSON, or XML.
- AirNow fact sheet recommends file products over web services for large time ranges or large geographic extracts.
- AirNow Data Exchange Guidelines state that AirNow observations are preliminary and non-regulatory.
- No API keys are committed; live validation should either use environment-provided keys or prove structured setup guidance for auth-gated paths.

## Authentication

- `AIR_QUALITY_OPENAQ_API_KEY`: optional but required for live OpenAQ requests.
- `AIR_QUALITY_AIRNOW_API_KEY`: optional but required for AirNow web service requests.

If a key is missing, commands must return structured setup guidance instead of pretending to have queried the source.

## Output Contract

All commands are read-only. `--agent` and `--json` output must be compact JSON with these recurring fields where applicable:

- `source`
- `configured`
- `query`
- `location`
- `measurements`
- `freshness`
- `caveats`
- `setup`

The CLI must clearly distinguish physical pollutant measurements from AQI categories. OpenAQ commands should not manufacture AQI values from physical units. AirNow commands may return AQI categories only when returned by AirNow.

## Non-Goals

- No medical advice.
- No emergency-service claims.
- No regulatory decision support.
- No pollutant-to-AQI conversion unless a source explicitly returns AQI.
- No long-running alert daemon in the first print.
- No write-side API calls.
- No raw endpoint dump.
