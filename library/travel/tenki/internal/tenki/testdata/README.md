# Dated source fixtures

`source-metadata.json` maps each unmodified raw HTML capture to its exact tenki.jp URL, snapshot date and local acquisition-file timestamp. Full source files preserve competing analytics/sidebar timestamps, malformed optional metadata, missing numeric columns and the actual SSR layout so tests cannot pass by reading simplified invented page structures.

All current source snapshots are dated 2026-09-27 in Japan. They are test evidence rather than current weather. Sakura updates in these snapshots ended for 2026. Foliage retains a 2025 sidebar alongside the active 2026 product. Current-year source report publication and continuing municipal weather have separate clocks.

`TestConstructedActiveSeasonAndWrongYearSidebar` uses an explicitly constructed active-season layout with optional predicted dates because the captured September Sakura source has no active predicted dates. The test comments and name identify this as a constructed variant, not an observed active snapshot. Temporal and null-value boundary variants are similarly constructed inside tests, retain existing source shape and are explicitly named.

These fixtures support the approved bounded public HTML implementation. No official provider API is claimed. Source terms Article 8(6) limits non-browser/RSS acquisition; that access limitation was disclosed in the approved scope. HTML markup and seasonal/model refresh cadence can change.
