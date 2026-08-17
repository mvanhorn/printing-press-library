# Air Quality Live Validation Summary

The first print was validated without committing API keys. OpenAQ and AirNow live commands are auth-gated and return setup guidance when the relevant environment variable is missing.

## Verified Without Secrets

- `air-quality-pp-cli --help`
- `air-quality-pp-cli --version`
- `air-quality-pp-cli sources --agent`
- `air-quality-pp-cli doctor --agent`
- `air-quality-pp-cli current --lat 40.7128 --lon -74.0060 --agent`
- `air-quality-pp-cli nearest --lat 40.7128 --lon -74.0060 --agent`
- `air-quality-pp-cli location 2178 --agent`
- `air-quality-pp-cli history --sensor 3917 --days 7 --agent`
- `air-quality-pp-cli compare --lat-a 40.7128 --lon-a -74.0060 --lat-b 34.0522 --lon-b -118.2437 --agent`
- `air-quality-pp-cli airnow current --zip 10001 --agent`

## Source Proof

- OpenAQ docs confirm API v3, `X-API-Key`, 60/minute and 2,000/hour free limits, and retired v1/v2 endpoints.
- OpenAQ OpenAPI confirms v3 locations, sensors, latest measurement, and measurement endpoints.
- AirNow web-service docs confirm current forecasts, current observations, historical observations, and contour maps.
- AirNow fact sheet confirms account/API-key access and CSV/JSON/XML output.
- AirNow Data Exchange Guidelines state AirNow observations are preliminary and non-regulatory.

## Validation Mode

Unit tests mock OpenAQ v3 with `httptest` to prove `X-API-Key` request wiring and latest-measurement parsing. No API key or live response payload is committed.
