# Research artifacts: skipped

airframe was built via the printing-press **plan-driven scaffold path**
(`printing-press generate --plan ~/.claude/plans/aeroapi-real-but-humming-meteor.md --name airframe`)
rather than the spec-driven path. Plan-driven generation does not produce
research artifacts because there is no upstream API spec to research against.

The substitute "research" for airframe lives in the approved plan file (read it
for the full context on data-source selection, schema design, tier model, and
the 72-hour caveat on commercial-flight composition):

  `~/.claude/plans/aeroapi-real-but-humming-meteor.md`

The plan covers:

- Why FAA Releasable Aircraft Database + NTSB CAROL avall.zip were chosen as
  data sources (and why USPTO/TSDR, FEC, USAspending, ClinicalTrials, EPA ECHO,
  GDELT, Crossref, etc. were not).
- Sizing math: 60-70 MB FAA zip + 90 MB NTSB MDB → ~150-200 MB SQLite Core profile.
- Domain-typed schema design (aircraft, make_model, engine, dereg, events,
  event_aircraft, narratives, sync_meta).
- The "Two sync tiers" UX model (FAA = zero deps, NTSB = mdbtools).
- The real-world finding that killed the originally-pitched "shopping for tickets
  → safety dossier" use case (airline tail assignments happen 48-72h before
  departure, not at booking time) and the resulting pivot to standalone
  forensics + a much narrower flight-goat composition window.

The phase5-acceptance.json proof at `.manuscripts/20260513-023951/proofs/`
records the end-to-end smoke verification that replaces the missing
acceptance-test matrix a spec-driven CLI would have produced.
