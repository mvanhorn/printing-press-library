# Publication review corrections

All four initial PR2417 findings were confirmed and corrected. Literal search now uses Unicode case-folded substrings, including punctuation; read-only existing mirrors require no schema migration. MCP launch and printed connection settings preserve the selected home and profile. Mirror creation rejects an unsafe existing parent without chmod or DB creation, while newly created private directories remain supported.

Validation: full Go test suite and go vet passed. Regression coverage includes substrings, punctuation, accented text, multi-rune case folding, read-only reopening, real subprocess environment propagation, printed MCP settings, and parent permission preservation. The fresh full read-only live matrix passed 120/120 with zero failures and 85 separate safety/framework/fixture skips.
