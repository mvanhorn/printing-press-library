# Scorecard explicit-window anchor

Preserve the scorecard rule that an explicit aligned window owns its anchor: reject an explicit `--as-of` outside the window, and default an omitted anchor to today within a current window or the final day of a past window. Tests cover month, week, quarter, pace, and usage-class failures.
