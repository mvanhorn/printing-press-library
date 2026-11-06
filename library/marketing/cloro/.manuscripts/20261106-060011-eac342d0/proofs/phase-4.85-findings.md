# Phase 4.85: agentic output review

Reviewed inline by the printing agent against real output.

- `cited nike.com --prompt "best running shoes for flat feet" --country US --engines aimode,gemini` (live): valid JSON view with `domain`, `country`, `cited_by`, one row per engine (`cited`, `positions`, `urls`, `sources`). Both engines answered; nike.com was not cited in that run. Output shape is correct for an empty match (`cited_by: []`, empty arrays, not null).
- `tasks create --task-type GOOGLE --payload ...` then `tasks get <id>` (live; the commands are now `async create` and `async get`): task went QUEUED to COMPLETED in about 3 seconds; `credits.creditsCharged` 3; the Google result is under `response`.
- `google ... --include-aioverview --dry-run`: request body carries `include.aioverview: true`.
- `chatgpt --prompt '["a","b"]' --dry-run`: a JSON array prompt is sent as a multi-turn array; a plain string stays a string.

Warnings (not blocking):
- W1: the async result is directly under `response`, not `response.result`, so a `--select response.result...` path on `async get` matches nothing. The README does not show such a path.
- W2: nike.com was not cited by AI Mode or Gemini for the example prompt in this run. Answers vary between runs; the example still shows the output shape.

status: PASS
