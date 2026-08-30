# Acceptance Report: catapult-openfield-connect
  Level: Full Dogfood (live, CATAPULT_TOKEN, read-only)
  Tests: 163/163 passed (prior run: 158/166)
  Failures fixed between runs:
    - benchmark/rtp happy+json probes: Cobra Example strings carried fictional fixtures
      ("Centre Back" position, nonexistent athlete); replaced with resolvable forms
      (squad-wide benchmark; rtp --athlete top). The commands themselves were correct.
    - sensor devices: bare probe hit the API's 422 (requires an activity/period id);
      endpoint dropped from the spec as a duplicate of activities devices (which passes
      with a harvested real id).
    - stats: unbounded whole-history aggregation exceeded the 30s cap (same server
      pathology as the date-filter hang); spec example + happy_args now carry a
      lastActivities filter.
  Live data verified (redacted): squad ACWR with real risk zones for the test squad;
  session diff between the two most recent sessions; period heatmap of the most recent
  walk-thru; local sync of 5,463 items (2,628 activities, 523 athletes, 1,829 parameters).
  Printing Press issues (retro): fabricated Freshness covered-paths in generated
  SKILL/README; dead generated helper collectionItemsForOutput; learn-loop SQLITE_BUSY
  sentinel warning under concurrent invocations; stats engine date-filter server hang
  worth a spec-authoring note.
  Gate: PASS
