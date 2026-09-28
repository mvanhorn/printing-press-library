# Saved trip lists

Fetch a canonical restaurant page before using its bare ID on a fresh machine:

```bash
tabelog-pp-cli show https://tabelog.com/en/tokyo/A1301/A130101/13005012/ --agent
tabelog-pp-cli lists add tokyo-bars 13005012 --note "Ginza bar option" --agent
tabelog-pp-cli lists show tokyo-bars --agent
```

`lists show` without a name lists notebooks. Notes and membership are local user data; fetching source facts preserves them. An unseen ID requires its canonical source URL first.

A stored detail snapshot keeps its original retrieval time when a later search returns a newer, smaller listing summary. Use `lists refresh` to replace the saved detail facts. Explicit relocation/closure notices remain visible even when an old listing was fetched recently.

```bash
tabelog-pp-cli lists compare tokyo-bars --agent
tabelog-pp-cli lists alternatives tokyo-bars --for 13005012 --match area,category --meal dinner --budget-max 5000 --agent
```

Alternatives scan saved candidates only. Area matching uses verified source identifiers; category matching uses source labels. The optional local budget test requires a known bracket ceiling within the amount. Read unmatched and unevaluable counts. Matches retain list order and do not imply availability, walking distance or a new quality ranking.

```bash
tabelog-pp-cli lists audit tokyo-bars --require hours,payment,reservation,dinner_budget --max-age 24h --agent
tabelog-pp-cli lists refresh tokyo-bars 13005012 --agent
```

Audit distinguishes `detail_not_fetched`, `source_unknown`, and `older_than_threshold`. Refresh may leave source-unknown fields unresolved. Newly fetched information is added evidence, not necessarily a real-world change. Per-ID failures retain valid snapshots; partial failure returns non-success status with usable results.

Use `lists note` and `lists remove` help for maintenance. Dry runs preview actions without network or persistent writes.
