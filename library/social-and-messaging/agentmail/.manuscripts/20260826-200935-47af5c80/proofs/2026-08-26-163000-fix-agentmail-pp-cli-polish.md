# AgentMail Polish

The deterministic polish pass inspected dead-code candidates but removed none because its first removal set caused generated data-source references to fail compilation (`decodeWriteThroughNonEmptyArray` remained referenced). The pass restored all attempted removals. No polish source changes remain from this pass; the final CLI remains buildable and the generator-owned helper references are preserved.

Candidates reported: BindMCPClient, decodeWriteThroughNonEmptyArray, extractPageItems, registerClientHook, registerNovelCommand, registerPlatformSource, resolvePaginatedRead, resolveRead, resolveReadWithStrategy.

Recommendation: proceed with the already verified CLI; file the helper reachability classification as a generator polish candidate rather than deleting referenced functions.
