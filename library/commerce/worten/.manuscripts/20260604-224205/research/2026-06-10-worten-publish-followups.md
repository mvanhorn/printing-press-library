# Worten Publish Followups

This follow-up publish commit closes the library review gaps found after the initial submission.

Changes in this follow-up:

- add the missing `research/` manuscript directory to the published package
- keep a root `dogfood-results.json` alongside the packaged CLI
- make `gtin13` extraction case-insensitive so uppercase UUIDs still resolve
- make the challenge-page retry backoff honor context cancellation

These changes preserve the promoted CLI behavior while satisfying the library publication contract and the review findings on PR `#1136`.
