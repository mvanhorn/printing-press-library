# Raw keyword helper diagnosis

The focused client already sends `Accept: text/html,application/xhtml+xml` and passes live searches. Generated raw HTML helpers defaulted to `Accept: application/json`. A controlled pair of anonymous requests used identical URL, query, charset and User-Agent: JSON Accept returned HTTP 500 with an empty keyword error page; HTML Accept returned HTTP 200 and the requested Japanese keyword. See raw-keyword-accept-probe.json. The source params were present in dry-run and are not the defect.

The research spec and checked-in spec now declare the required HTML Accept header for regeneration. A local durable default-header integration is being added to the generated-client seam; no shared Printing Press setting is changed.
