# Publication review fixes

Four Greptile findings were fixed: displayed sale windows survive absent/invalid/one-sided variant fields; bounded comparisons select sessions round-robin and reject caps below the input count; partial international search failures reach stderr while per-URL metadata remains; the optional authenticated MCP HTTP transport bounds header reads at five seconds.

Focused regression tests reproduce the prior window/diagnostic failures and pass after the fixes. Tests cover uneven comparison groups, insufficient limits and JSON/stderr separation. The full live gate reran after all source edits: 42 mandatory probes passed, 32 optional probes remain unverified. Patch records preserve these contracts across reprint. Fresh security scanning reports no hand-authored-code findings.
