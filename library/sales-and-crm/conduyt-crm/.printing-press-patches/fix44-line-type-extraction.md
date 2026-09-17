# Fix 44: shared contact line-type extraction

Use one extractor for local estimates, live estimates, and send-check. It honors
top-level aliases before `customFields.sms_line_type` and trims values so every
consumer classifies the same contact consistently.
