# Local code review

16 distinct findings autofixed in place across two fix passes and three complete review rounds; this generated working tree has no commits. The recorded patch manifest and current source, plus the retained sanitized reviewer regression proofs, are the change record.

Template-shape and reserved-package retro candidates: none established. No generator-reserved package was modified.

Surface-to-user findings: none pending. The supported private Mac target and all seventeen approved capabilities remain intact. Windows storage fails closed and is explicitly documented as unverified.

Findings cleared at round 3: the merged correctness, security and reliability reports are all clear. Full Go tests and vet pass.

Review path chosen: direct subagent dispatch, with correctness/API contract, security/privacy, and reliability/maintainability reviewers. Every round reran all three reviewers before root merged and fixed the set. Claude-only simplify is inapplicable on Codex.

## Live-evidence correction after review

The phase18 archived-thread read falsified the reviewers' transcript encoding assumption. This endpoint uses positional PBLite strings, including literal UTF-8 for transcript byte fields; canonical protojson base64 rules do not apply. The decoder now preserves those tokens literally, including base64-shaped ordinary text, and a regression test covers the distinction. The original reviewer reports are historical source-inspection results, not live protocol proof. Phase18 tests and the final binary-owned matrix own acceptance of this later correction.
