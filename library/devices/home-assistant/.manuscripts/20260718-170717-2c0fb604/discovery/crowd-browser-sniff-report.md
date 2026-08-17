# Home Assistant Crowd-Sniff Report

## npm Packages Analyzed
- `iobroker.lovelace` — irrelevant frontend bundle; extraction ended with an unexpected EOF before endpoint analysis.
- `@hakit/core` — relevant WebSocket client package; extraction ended with an unexpected EOF before endpoint analysis.
- The npm downloads API returned HTTP 400 during discovery.

## GitHub Repos Searched
- Search target: `home-assistant`; GitHub authentication was available through `gh`.
- No endpoint survived the crowd-sniff confidence and normalization gates.
- Manual source analysis separately covered `home-assistant-ecosystem/home-assistant-cli` and `homeassistant-ai/ha-mcp`; those sources are credited in the research brief and absorb manifest, not misrepresented as crowd-sniff output.

## Endpoints Discovered

No endpoints were emitted by the crowd-sniff command.

## Base URL Resolution
- A Home Assistant instance normally uses a user-configured URL such as `http://homeassistant.local:8123`.
- There is no universal cloud API base URL; the CLI must require or discover the user's instance URL.

## Auth Patterns Detected
- Crowd-sniff produced no auth evidence. Official docs and verified source establish bearer long-lived access tokens for REST and access-token authentication for WebSocket.

## Parameter Name Evidence
- No crowd-sniff parameter evidence was produced. Exact parameter names will come from official REST/WebSocket docs and the two verified incumbents' source.

## Coverage Summary
- Total endpoints: 0.
- Result: failed primary discovery; fall back immediately to official docs and verified source, per the Printing Press crowd-sniff contract.
