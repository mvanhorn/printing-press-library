# Public artifact cleanup

Printing Press packaging and secret/PII gates passed. A further structural review removed raw HTTP capture/cache payloads, the compiled acceptance-test binary, unrelated catalog snapshots, and raw full-dogfood transcripts from the public bundle. Research, reviewed access findings, source-bound acceptance, test summaries/logs and measurements remain. Original generation artifacts and pre-cleanup copies are retained locally.

Absolute host paths in retained manuscript text were replaced with <cli-dir>, <press-home>, <workspace> and <home> placeholders. Recorded measurement numbers and test verdicts are unchanged. Two generic email-shaped documentation placeholders in the archive were replaced with PII_EMAIL_EXAMPLE before packaging. No credentials were used for Walkerplus access. Public event/venue facts are intentionally retained.

Removed 73 artifact files; scrubbed host paths in 34 retained files. No Go source, dependencies, event facts or acceptance hashes changed.
