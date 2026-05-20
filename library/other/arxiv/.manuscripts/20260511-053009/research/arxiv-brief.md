# arXiv CLI research brief

arXiv exposes a free public Atom query API documented at https://info.arxiv.org/help/api/. It has no official OpenAPI document, so this print uses a minimal custom OpenAPI spec for `/api/query` based on the official arXiv API documentation.

The useful CLI shape is intentionally focused: search papers, fetch latest papers by category, and resolve exact arXiv IDs. The generated command parses Atom XML into JSON so agents can consume entries, authors, links, categories, and metadata without writing XML glue.
