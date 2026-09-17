# Fix 59: watch side-effect completeness

- `imports watch --verify` and `imports blame` must require `isComplete == true` in the job-scoped
  side-effect response, parse it strictly as a JSON boolean, and return the
  existing partial API failure when it is false, absent, or invalid.
- `imports watch --verify` and `imports blame` share the same strict parser and
  render the value as `side_effects.is_complete` for both `true` and `false`.
- Endpoint-level regression coverage lives beside both import commands.
