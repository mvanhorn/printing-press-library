# Spec contract corrections

The mechanical browser-sniff spec selected arbitrary nested arrays as the response_path for campsite detail, plan detail and master data. Corrected authored nap-camp.yaml preserves full objects/arrays and exact public GET paths and default headers, adds verified named parameters and realistic live fixture arguments, and models no authentication. The raw source endpoints keep upstream data; hand-written domain commands supply bounded JSON, null/unknown interpretation, source timestamps and canonical handoffs. Source sniff evidence remains as an audit artifact.

Cache freshness is disabled: source contracts are read-through and focused saved snapshots are user observations, not a replaceable bulk cache. MCP is stdio-only for local use; seven endpoint methods are below the enrichment threshold. Learn seeds use concrete observed Japanese canonical names and equivalent English aliases. Category travel is set before generation.
