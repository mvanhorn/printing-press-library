# Fix 33: paid-loop monotonic progress

Reject every post-initial nonterminal line-type verification response unless
`remaining` strictly decreases. The rejection happens before batch totals are
updated, preserving accurate partial metadata and preventing repeated paid POSTs.
Regression tests cover unchanged, increasing, and normally decreasing progress.
