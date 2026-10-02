# MCP publication review fixes

Greptile's first review found two P1 security issues and one P2 guidance issue. Focused regressions reproduced both security defects before correction, then passed.

- The actual listener constructor applies 5-second header, 30-second request-read and 1-minute idle deadlines to HTTP and TLS. Streamed responses retain their normal lifetime. Real stalled HTTP and TLS connections close before entering authentication or MCP handling. [Go HTTP server deadline semantics](https://pkg.go.dev/net/http#Server).
- `cache-dir` is a blocked MCP destination argument. The actual `guide_inspect` schema omits it, and injected structured overrides are rejected before a child process or destination directory is created. `cache` and `offline` remain available; direct CLI operators retain `--cache-dir`. Root/home/config/profile overrides were already blocked.
- SQL describes the existing framework store and learnings. Guide facts are extracted JSON snapshots outside that store; no sync prerequisite is advertised.
- The agent skill no longer names an agent-config file or describes the package as unpublished. Both the filename guard and source-consistency verifier pass.

All-package Go tests pass after these changes. Full live dogfood reran after the Go edits: 91/91 mandatory cases PASS; 83 optional/safety/fixture probes explicitly skipped/unverified. Fresh source fingerprint: `2d5a1b0a434c6ad7c937829ade056fd46a4de189840fbd4b9900bbc50b67f5c5`. The promoted, archived, workspace and packaged acceptance copies are identical.

No guide selector, source-fact identity, request budget, opening-status inference or offline freshness semantics changed. The customization record preserves the MCP trust boundary across regeneration.
