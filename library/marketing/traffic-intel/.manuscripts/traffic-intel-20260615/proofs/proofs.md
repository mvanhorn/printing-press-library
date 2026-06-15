# Traffic Intel private composite CLI

This private print combines Google Search Console, Google Analytics 4, and Ahrefs child CLI output into a local-first traffic intelligence layer.

## Verification

- `go test ./...`
- `traffic-intel-pp-cli --agent agent-context`
- `traffic-intel-pp-cli --agent sources doctor`
- live Ahrefs child sync verified against `bestself.co` with temporary local state
