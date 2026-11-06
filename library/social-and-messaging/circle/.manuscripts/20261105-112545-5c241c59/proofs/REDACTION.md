# Redaction note

These proofs come from a real run against a live Circle community. Before public release:

- Every live Circle id (spaces, space groups, events, posts, sections, lessons, members, users, tags, profile fields) in these proofs, the README, SKILL.md, command examples and test fixtures was replaced with a synthetic id. Each id range was shifted by one fixed offset, so ids that were equal stay equal and ordering is unchanged.
- The test account email, community id, slug, URL and member display name were replaced with placeholders.
- `phase5-acceptance.json` and `runtime-matrix-release.json` contain no live ids and are unchanged. Their verdicts, counts and the recorded source fingerprint are from the release run; the fingerprint covers the source before these literal swaps.
