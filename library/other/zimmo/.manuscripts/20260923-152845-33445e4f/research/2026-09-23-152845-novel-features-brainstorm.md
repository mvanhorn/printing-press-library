# Novel features brainstorm (subagent af0feb89d6dd36a62, 2026-09-23)
Personas: the investor client (Brussels deal-sourcing investor, daily peb-hunter cron), the tool builder (sourcing-tool builder, immoweb/immovlan stores), Léa (buy-to-let investor, yield screen), Karim (agent/analyst, sold comps).
Candidates (16): comps, underpriced, peb-trap, enrich, same-as, address-fill, yield, motivated, relisted, redflags, worth, momentum, agencies, near, split-candidates, digest.
Survivors (7): enrich 9, same-as 9, comps 9, peb-trap 9, underpriced 8, yield 7, motivated 7 — all hand-code.
Killed: address-fill (writes into sibling DB; same-as --export-address), relisted (folded into motivated), redflags (thin filter; peb-trap), worth (single-endpoint wrapper; comps), momentum (monthly data; underpriced), agencies (analytics --group-by covers it), near (flag-level of find; comps), split-candidates (server-side surface filter), digest (orchestration; peb-trap).
