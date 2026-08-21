# Catasto live endpoint research

The promoted Catasto print uses two public data sources:

- Agenzia delle Entrate's public AJAX endpoint for GPS-to-cadastral lookup.
- `ondata/dati_catastali` regional Parquet files for cadastral-to-GPS centroids.

The implementation and the Phase 5 live gate use the same endpoints and the
same representative Rome reference (`H501`, foglio `508`, particella `B`) so
the acceptance evidence exercises both directions of the published CLI.
