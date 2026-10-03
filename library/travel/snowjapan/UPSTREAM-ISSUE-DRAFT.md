# Printing Press: factual HTML adapters need a typed-MCP handler extension seam

SnowJapan's current module uses Wix SSR and source-owned charts rather than REST JSON. Its CLI adapter returns bounded factual projections, but emitted typed HTML endpoint MCP handlers bypass that adapter and return raw HTML. This drops chart facts and can expose report prose.

Add a durable source-specific handler registry to the emitted typed MCP path, or let the runtime Cobra handlers replace typed HTML endpoints with the same public names. The SnowJapan module currently records one narrow `makeAPIHandler` dispatch seam in `.printing-press-patches/`; a future regen must preserve its factual/no-prose contract. Core evidence is in the source run's `discovery/source-contracts.json`, independent review and packaged MCP runtime proofs.

A separate artifact check found an MCPB built before a hand-authored CLI update retained an old scaffold companion even after the MCP binary changed. Rebuilding with `bundle --cli-binary <current stage binary> --cli-skip-build` fixed the pairing. A first-class shared post-edit artifact refresh should validate both embedded companions against the current staged binaries.
