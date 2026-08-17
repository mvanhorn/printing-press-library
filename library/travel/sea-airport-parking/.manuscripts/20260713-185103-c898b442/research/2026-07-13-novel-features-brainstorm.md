# Novel Features Brainstorm — sea-airport-parking (audit trail)

Survivors (5, all hand-code, >=8/10): sweep (10), watch (10), history (8, local),
drift (8, computed), calendar (8, live).

Killed: cheapest (folded into sweep), compare (folded into sweep), stay-length
optimizer (overlaps sweep), savings vs General (unquotable constant), promo-sweep
(speculative), multi-airport (config concern not a command), sellout-forecast
(unverifiable/predictive), export (covered by --json/--csv), best-value-day
(thin variant of drift).

Customer model: peak-date traveler (sellout fear -> watch), flexible-date planner
(-> sweep), automation/agent (Omar's existing browser skill upgraded), repeat
flyer (price-relative-to-history -> drift). Unifying asset: the local
quote-snapshot store; every quote is a fresh live POST that gets persisted.
Full transcript in the run's brainstorm task output.
