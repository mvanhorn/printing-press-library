# Changelog

This file is maintained by printing-press-library release automation. Do not hand-edit release sections in normal PRs.

## 2026.11.2 - 2026-11-08

- fix(ars-sicilia): --select reaches nested rows, --csv survives empty sections (#2439).

## 2026.11.1 - 2026-11-07

- fix(ars-sicilia): truncation hint states the declared total, --select hint gives a working selector (#2436).

## 2026.10.1 - 2026-10-12

- fix(ars-sicilia): repeated numbers on per-committee archives are distinct acts, not versions (#2309).

## 2026.9.7 - 2026-09-19

- fix(ars-sicilia): the empty-search hint measures the source lag instead of declaring a fixed band (#2019).

## 2026.9.6 - 2026-09-14

- fix(ars-sicilia): sync stale no longer reports an unreadable store as never synced (#1997).

## 2026.9.5 - 2026-09-13

- feat(ars-sicilia): resoconti get answers with the /bd/ scheda, its blocks, and the sitting text (#1992).

## 2026.9.4 - 2026-09-11

- fix: fail closed on ambiguous fleet write retries (#1978).

## 2026.9.3 - 2026-09-06

- fix(ars-sicilia): punctuation in a search value no longer fails the search (#1952).

## 2026.9.2 - 2026-09-05

- fix(ars-sicilia): a multi-day sitting is not an anomaly, plus five fixes from the news-driven checks (#1941).

## 2026.9.1 - 2026-09-02

- fix(ars-sicilia): --frase conjunction rewrite + aggregated-laws warning (leftover from #1900) (#1905).

## 2026.8.10 - 2026-08-31

- fix(ars-sicilia): iter events say where they happened, and a cut ddl search names the way in (#1879).

## 2026.8.9 - 2026-08-30

- fix(ars-sicilia): the portal's refusal page was read as zero results (#1858).

## 2026.8.8 - 2026-08-29

- fix(ars-sicilia): three silences that made the CLI cite the wrong document (#1855).

## 2026.8.7 - 2026-08-25

- refactor(ars-sicilia): build the /bd/ request once, and let the preview print it (#1799).

## 2026.8.6 - 2026-08-23

- feat(ars-sicilia): make --dry-run tell the truth about the request it would send (#1790).

## 2026.8.5 - 2026-08-22

- feat(ars-sicilia): stop dropping a nested list under --select, and name laws the way you filter them (#1781).

## 2026.8.4 - 2026-08-21

- fix(ars-sicilia): check sitting/date coherence both ways, drop the unmet emendamenti claim (#1772).

## 2026.8.3 - 2026-08-17

- fix(catalog): align Go fallback docs with toolchain floor (#1735).

## 2026.8.2 - 2026-08-17

- fix(cli): bump golang.org/x/text to v0.39.0 in affected published CLIs (#1636).

## 2026.8.1 - 2026-08-10

- feat(ars-sicilia): /bd/ backend for sitting archives + stralcio navigation, per-law results, phrase search (#1574).

## 2026.7.2 - 2026-07-22

- feat(ars-sicilia): ISIS date/codcom filters, --escludi, profilo exit fix (#1400).

## 2026.7.1 - 2026-07-08

- fix(catalog): require Go 1.26.5 across published modules (#1467).

## 2026.6.2 - 2026-06-21

- fix(catalog): require Go 1.26.4 across published modules (#1308).

## 2026.6.1 - 2026-06-08

- Baseline release metadata added for this published CLI.
