# Phase 4.85 Output Review Findings

Status: WARN (4 findings, all fixed in polish)

1. insider-flow sorted by |net| — net sellers ranked in "top buyers"
   → Fixed: signed descending sort (buyers first)
2. rank --by insider was fake (dividend_yield*10)
   → Fixed: removed misleading key; valid keys: composite, roce, profit-var, sales-var, pe
3. qtrend acceleration/deterioration flags never fired on deceleration
   → Fixed: added "profit growth decelerating" flag (positive YOY shrinking QoQ)
4. insider-flow empty result had no diagnostics in agent mode
   → Fixed: stderr warning emitted in all modes on empty parse

All 4 findings were genuine quality issues in hand-authored novel commands and
were fixed in the polish pass. None remain.
