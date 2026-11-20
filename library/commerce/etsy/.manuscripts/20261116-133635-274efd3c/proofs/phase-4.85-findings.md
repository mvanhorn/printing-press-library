# Phase 4.85 agentic output review — etsy-pp-cli
Status: WARN (2 warnings; Wave B = non-blocking). Live-check: 14/15 samples pass; market gaps prerequisite_unsynced (no --rules file in harness).
1. format-bugs — market price pooled currencies (USD/GBP/EUR/VND/...) in one taxonomy group, labelled with rs[0].Currency ("USD max 600000" was a VND listing). market_price.go:51. Disposition: FIX (orchestrator-verified in source).
2. result-ordering — market demand with no review data orders by search position under a "demand" label. market_demand.go:100-112. Disposition: FIX (state ordering basis + top-level hint).
Not flagged: snapshot harness terms.txt (correct CLI error), shop attention 0+note (documented gap), tags/shops ordering OK, &amp; is JSON escaping. TTY table HTML-entity rendering not checked.
Both findings FIXED in 5e6130d; verified on live snapshot (1062/USD max 75; demand order_basis search_position + note).
