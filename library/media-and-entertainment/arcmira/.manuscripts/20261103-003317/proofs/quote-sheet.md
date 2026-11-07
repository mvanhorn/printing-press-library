# Quote-sheet verification

The compiled CLI reproduced exact text, title, channel, timestamp and watch URL for three saved search passages. Raw API JSON, printed CLI JSON and stdin produced the same rows. Combining overlapping searches preserved both query labels. Duplicate input produced the same sheet, empty results produced an empty list, and malformed records or contradictory snapshots exited with an error. These checks ran without credentials.

From the package directory, run the bundled example:

```sh
arcmira-pp-cli quote-sheet examples/quote-sheet.json --agent
```

The example produces one row with passage ID sample@12 and the query “clip research”. Its input is synthetic.
