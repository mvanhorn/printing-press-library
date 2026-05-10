## homeassistant

Homeassistant API surface as MCP tools. Fully functional CLI with offline search, agent-native output, and a local SQLite data layer.

**API:** homeassistant | **Category:** devices | **Press version:** 4.2.2
**Spec:** /home/adrian/printing-press/.runstate/cli-printing-press-6d0014bc/runs/20260509-211834/research/homeassistant-crowd-spec.yaml

### Publication Path
New print

### Novel Features
- **FTS Entity Search**: Find exact entities instantly using full-text search across friendly names and attributes.
- **State Tailing**: Stream state changes live to the terminal.
- **Service Schema**: Print the exact JSON payload expected by any Home Assistant service.
- **State History**: Query state change history for entities over a time period.
- **Activity Logbook**: View the activity logbook with human-readable event descriptions.
- **Template Rendering**: Render a Home Assistant Jinja2 template server-side.
- **Event Bus**: List event types or fire custom events on the HA event bus.
- **Config Validation**: Validate the Home Assistant configuration.yaml remotely.

### Validation Results
- manifest: PASS
- transcendence: PASS
- phase5: PASS
- go mod tidy: PASS
- govulncheck: PASS
- go vet: PASS
- go build: PASS
- --help: PASS
- --version: PASS

