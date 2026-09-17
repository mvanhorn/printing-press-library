# Fix 39: scorecard calendar-period pace

Scorecards now query the complete UTC calendar period containing `--as-of` and
compute pace against those same boundaries. Rolling or non-aligned windows are
usage errors; regression tests cover partial months, weeks, quarters, and typed
rejection, while command and cookbook examples use valid period selection.
