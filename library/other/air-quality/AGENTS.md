# Air Quality Print Notes

- Keep OpenAQ physical measurements separate from AirNow AQI categories.
- Do not add medical advice, emergency advice, or regulatory decision support.
- Do not commit API keys or captured live responses containing keys.
- Use `AIR_QUALITY_OPENAQ_API_KEY` and `AIR_QUALITY_AIRNOW_API_KEY` only from the local environment.
- Keep commands recipe-oriented; do not turn this print into a raw endpoint mirror.
- Run `go test ./...` and `cli-printing-press publish validate --dir . --json` before publishing changes.
