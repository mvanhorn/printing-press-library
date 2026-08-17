# Open Library Live Validation Summary

The first print was validated against Open Library's public, keyless JSON APIs. No API key, account cookie, patron data, borrowing action, list mutation, or write-side catalog route is used.

## Verified Without Secrets

- `open-library-pp-cli --help`
- `open-library-pp-cli --version`
- `open-library-pp-cli sources --agent`
- `open-library-pp-cli doctor --agent`
- `open-library-pp-cli book "Designing Data-Intensive Applications" --limit 3 --agent`
- `open-library-pp-cli isbn 9780131103627 --agent`
- `open-library-pp-cli author "Ursula K. Le Guin" --limit 3 --agent`
- `open-library-pp-cli work OL45804W --agent`
- `open-library-pp-cli editions OL45804W --limit 3 --agent`
- `open-library-pp-cli subjects "distributed systems" --details --limit 3 --agent`

## Source Proof

- Open Library's API index confirms public JSON/YAML/RDF APIs, usage guidance, contact-bearing `User-Agent` guidance, rate limits, and bulk-access caveats.
- The Search API docs confirm `/search.json`, `q`, `fields`, pagination, and language preference behavior.
- The Books API docs confirm the preferred search path plus work, edition, and ISBN JSON lookup routes.
- The Authors API docs confirm author search, individual author JSON, and author works with limit/offset.
- The Subjects API docs confirm subject works and `details=true`, while marking the API experimental.

## Validation Mode

Unit tests use an in-memory HTTP transport to prove request paths, query parameters, contact-bearing `User-Agent` behavior, subject slugging, and compact JSON parsing. Live smoke commands are bounded to small limits and use public read-only endpoints.
