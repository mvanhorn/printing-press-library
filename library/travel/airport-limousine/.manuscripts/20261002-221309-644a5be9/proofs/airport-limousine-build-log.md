Manifest transcendence rows: 5 planned, 5 built. All 5 shipping command paths exist and return useful public-source data.

Generated with Printing Press v4.32.5; browser-chrome transport explicitly selected after the browser-http scaffold proved to be plain HTTP. Chrome TLS runtime is cookie-free for all data reads except public stop keyword search, which uses an ephemeral private jar and never imports auth cookies.

Implemented route/stop discovery, boarding maps and routes, dated terminal timetables, current-vs-standard estimates, both airport transfers, served-pair adult/child fare arithmetic, concise conditions and canonical booking handoff. Parser preserves unknowns, Japanese names, source dates/clocks and day rollover, while omitting inventory.

Full go test -count=1 ./... passed. Consequential tests cover malformed/cyclic data, time rollover, reversed roles, selected pairs, mixed/unknown fares, unavailable durations, public form state, 429 errors, cancellation and body limits. Source-mode and capability-index checks added after independent review. Priority 1 help/dry-run/output checks passed on routes, stops get and handoff; all shipping features live-tested with assertions.

Live integration: 16/16 content checks pass at 2026-10-02 15:37 UTC. Representative end-to-end latency 127–1172ms, JSON output 479–38540 bytes for bounded samples. Fresh runtime source paths and recipes work with --agent --select. First live assertion exposed double-wrapping of contextual envelope fields; the fixed envelope has exactly meta/results and puts station/route context under meta.

Review fixes: reject incompatible --data-source local before requests; preserve numeric condition values/units; add routes/stops/handoff to which capability index; replace empty README config-path claim; add domain source/bounds documentation. No shipping feature deferred, no stubs, no bookings/payments/logins.

## Final corrections

Refused provider redirects so every wire request stays inside the eight-request limiter/counter. Made the two stop-command read-only maps statically visible and added durable descriptions for all four raw page tools via mcp-sync. Corrected the live workflow manifest command syntax; all five real steps and the final seven-leg shipcheck pass. No approved feature was dropped. Full live acceptance and local promotion remain the final gates.
