# Printing Press: factual HTML adapters need a typed-MCP handler extension seam

SnowJapan's current module uses Wix SSR and source-owned charts rather than REST JSON. Its CLI adapter returns bounded factual projections, but emitted typed HTML endpoint MCP handlers bypass that adapter and return raw HTML. This drops chart facts and can expose report prose.

Add a durable source-specific handler registry to the emitted typed MCP path, or let the runtime Cobra handlers replace typed HTML endpoints with the same public names. The SnowJapan module currently records one narrow `makeAPIHandler` dispatch seam in `.printing-press-patches/`; a future regen must preserve its factual/no-prose contract. Core evidence is in the source run's `discovery/source-contracts.json`, independent review and packaged MCP runtime proofs.

A separate artifact check found an MCPB built before a hand-authored CLI update retained an old scaffold companion even after the MCP binary changed. Rebuilding with `bundle --cli-binary <current stage binary> --cli-skip-build` fixed the pairing. A first-class shared post-edit artifact refresh should validate both embedded companions against the current staged binaries.


## Committed open WAL and alias cache correctness

Generated `internal/store/store.go` `OpenReadOnlyContext` uses `immutable=1` and intentionally ignores uncheckpointed WAL frames. A committed open writer changed a resort peak from 1676 to 3000, but actual local get returned 1676 and changes returned an empty list. Hard-link aliases can also use separate WAL names and lose another writer’s newer observations. The source-specific seam in `internal/store/snowjapan_readonly.go` rejects sidecars, resolves symlinks, checks file identity around reads and pins one actual SQL connection. `snowjapan_writable.go` pins the canonical writer, rejects ambiguous hard links before open and checks selected identity around saves. The generated runtime remains unchanged; upstream should provide a guarded reader/writer contract with equivalent alias and lazy-connection regressions. The source patch index and cache guard tests identify the temporary seam.
