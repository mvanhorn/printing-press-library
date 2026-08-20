# Vercel Admin Research Brief

## Source Verification

- Official OpenAPI spec: `https://openapi.vercel.sh/`
- Spec format: OpenAPI 3.0.3
- REST base URL: `https://api.vercel.com`
- Authentication: bearer token via `Authorization: Bearer <token>`
- Team scoping: Vercel team resources use `teamId` query parameters.

## Scope

This print targets day-to-day Vercel administration for agents and operators:
deployments, projects, domains, environment variables, teams, storage, logs,
and incident triage. The generated CLI exposes the official REST operations and
adds a small `ops` layer for the repeated admin questions we expect to ask.

## Novel Features

- `ops recent-deployments`: summarize recent deployment state by project,
  target, and state.
- `ops failure-brief`: join deployment metadata, deployment events, check runs,
  and runtime logs into one JSON payload for failure triage.

## Local Constraints

No `VERCEL_ADMIN_TOKEN` was available in this environment, so live Vercel
requests were not sent. The operator layer is covered with `httptest` tests
that verify request paths, query parameters, auth headers, and joined output.
