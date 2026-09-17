# Fix 57: side-effect counter validation

- `imports blame` and `imports watch --verify` must parse every inspected import
  side-effect counter as a non-negative integer and return their existing partial
  API failure when any present counter is malformed or negative.
- Preferred `retryableFailed` and `exhaustedFailed` keys retain their legacy-key
  fallback and conflict checks, but use the same counter parser as all other
  side-effect keys.
- Endpoint-level regression coverage lives beside both import commands.
