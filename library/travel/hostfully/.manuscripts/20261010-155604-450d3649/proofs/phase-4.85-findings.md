# Phase 4.85 output review (direct)
Live outputs compared against the prior printed CLI on the same agency:
- today: identical check-in/check-out/occupied/vacant counts.
- availability --days 5: byte-identical result payload.
- whois by surname: identical match count.
- units: 9 properties discovered; numbering from property names reproduces the prior unit numbers.
- quote: itemized rent/tax/fees present; jobs: jobs with service and provider maps.
Warning: under --agent the generated CLI wraps output as {meta, results}; the prior CLI emitted the bare payload. Consumers parsing the old top-level shape must read .results.
Status: WARN (shape change only)
