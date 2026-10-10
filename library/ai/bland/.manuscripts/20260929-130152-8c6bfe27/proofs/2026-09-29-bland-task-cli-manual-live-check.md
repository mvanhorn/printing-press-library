# Bland task CLI manual live check

- Authorized CLI invocation completed with terminal status `completed`.
- `calls search-task weather --agent` returned valid JSON and one matching completed call record.
- A later read-only `calls list` query showed the latest call to the authorized destination as `completed`, with a summary present. No new call was placed for this check.
- The call and local search output were captured in a mode-700 temporary directory for verification. That directory, including the temporary SQLite database and raw output, was removed after the check.
- No call identifier, phone number, transcript, summary text, or weather/home details are retained in this proof.
- The fresh authenticated full matrix passed 107 checks with no failures. `calls search-task` now has a passing non-dry-run happy path on an empty local store. `calls task run` remains mechanically hollow because the runner appends `--dry-run` to mutating commands and excludes those probes from novelty coverage. This limitation is preserved in `phase5-acceptance.json` rather than hidden.
