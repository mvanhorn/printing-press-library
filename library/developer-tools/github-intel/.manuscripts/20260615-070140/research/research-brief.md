# GitHub current intelligence print research

Source: GitHub public REST API.

Verified live public endpoints:

- `GET /search/repositories`
- `GET /repos/{owner}/{repo}`
- `GET /repos/{owner}/{repo}/releases`
- `GET /advisories`

Promoted agent workflows:

- `trending`: approximate current repository momentum with search qualifiers and recent pushed windows.
- `releases`: inspect release velocity for a repository.
- `advisories`: brief security advisories for an ecosystem/package.
- `repo-health`: summarize adoption and maintenance signals.
- `compare`: rank repositories by adoption and recency signals.

No write operations are included. Public unauthenticated requests work, with GitHub public rate limits.
