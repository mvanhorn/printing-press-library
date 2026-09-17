# Fix 60: watch requires rendered side-effect counters

- `imports watch --verify` must require every rendered job-scoped side-effect
  counter, including `total`, `completed`, `failed`, and `held`, using the same
  strict preferred/legacy parsing contract as `imports blame`.
- Missing or invalid counters return the existing partial API failure, and
  missing watch-verification counters render as `null` rather than zero.
- Endpoint-level regression coverage lives beside the watch command.
