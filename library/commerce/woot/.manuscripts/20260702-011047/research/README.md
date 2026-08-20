# Woot Research Bundle

This directory contains the sanitized discovery evidence used for print run
`20260702-011047`.

Included artifacts:

- `../research.json`: the run-level feature brief used by dogfood and live
  scorecard validation.
- `woot-graphql-200-spec.yaml`: the promoted internal API specification.
- `graphql-200-traffic-analysis.json`: the protocol, endpoint, and auth-shape
  analysis derived from seven successful Woot GraphQL requests.
- `get__graphql__220e6fb1.json`: a representative request and truncated
  response sample with sensitive header values replaced by redaction markers.

The raw browser HAR is intentionally excluded because it contained a live API
key. The published sample contains the header name needed to reproduce auth
configuration, but no credential value.

The published `spec.yaml` adds the `SearchOffers.TotalHits` field used by the
hand-curated `deals` command. The promoted source spec in this directory is
preserved unchanged so the generated baseline remains auditable.
