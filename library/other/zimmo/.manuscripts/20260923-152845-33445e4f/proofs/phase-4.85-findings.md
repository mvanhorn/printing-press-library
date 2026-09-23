# Phase 4.85 agentic output review — zimmo-pp-cli (2026-09-23)
status: WARN (4 warnings), all fixed in-session:
1. underpriced ranked APARTMENT_BUILDING (whole buildings) against the commune apartment €/m² → whole buildings, mixed-use houses and service flats now skipped unless --include-special; subtype shown.
2. motivated --min-days only weighted the score → now a hard filter (default 0); age starts scoring at 90 days.
3. yield top rows dominated by one address with €/m² ~half the local level → rows under 60% of the local median asking €/m² flagged price_check=far_below_local_median (and "!" in the table).
4. peb-trap renovation_obligation mixed "NO" and "" → empty normalised to UNKNOWN.
Clean: comps widening disclosure, same-as missing-store explanation, no mojibake/entities/bad URLs.
