# Building a quote sheet from saved searches

A transcript search returns a query and a list of matching passages. The same passage can match several queries. Its ID identifies the passage across saved results. The text, start time and watch URL let a reader check the quotation against the recording.

The quote sheet groups results by passage ID and keeps the queries that found each passage. Two passages from the same video can have different IDs and both belong in the sheet. If two saved copies of one ID disagree about the text, timestamp or URL, the researcher needs to check the source before citing it.

The bundled example in `examples/quote-sheet.json` illustrates the response fields with a synthetic passage. It can be read offline.

[Transcript search documentation](https://arcmira.com/docs/search)
