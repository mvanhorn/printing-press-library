# search relevance check (resume item: dropped patch `search-scan-based-relevance-and-fuzzy`)

Old-vs-new comparison is infeasible: the 1.0.0 binary cannot open the operator store since it was migrated to schema 9
(2026-09-04), and no pre-migration copy survives. Characterised the 4.31.7 stock FTS5 `search` on a `.backup` copy of the
store (650 expenses, 56 friends, 45 groups, 164 notifications; counts only, copy deleted afterwards):

| query  | hits (limit 50) | hits where the word appears in a human field (description/name/content) |
|--------|-----------------|--------------------------------------------------------------------------|
| dinner | 39 | 38 |
| rent   | 8  | 8 |
| taxi   | 50 | 50 |
| coffee | 16 | 16 |
| uber   | 47 | 47 |
| paid   | 5  | 5 |

Prefix/substring over-matching: `search "din"` → 0 hits (no accidental match on "dinner"), so raw-JSON key noise is not
driving results. Verdict: no relevance regression worth a hand `find` command; the prior patch's word-boundary scan is
not needed on the 4.31.7 FTS5 content schema (`resourcesFTSContentSchemaVersion` 4 indexes extracted text). Item closed;
no machine issue filed.
