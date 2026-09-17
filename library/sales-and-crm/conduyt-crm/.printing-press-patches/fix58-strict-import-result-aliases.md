# Fix 58: strict import result aliases

- Import job row-count aliases use strict non-negative integer parsing: a
  present preferred key is authoritative, every present alias must be valid,
  and dual-key values must agree.
- Import side-effect `actionRequired` aliases use the same preferred-key and
  conflict rules in both `imports watch --verify` and `imports blame`.
- A missing action-required field is optional; watch and blame preserve the
  unknown state as an explicit JSON `null`, and their tables render it as
  `unknown`.
- Import row status uses strict preferred `status` and legacy `outcome` string
  aliases. Malformed, missing, or conflicting values make blame partial and
  identify the affected row and field.
- Endpoint-level regression coverage lives beside both import commands.
