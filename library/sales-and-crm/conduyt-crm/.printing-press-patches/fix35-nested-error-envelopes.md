# Fix 35: nested error envelopes

Reject string and structured error/message envelopes before unwrapping `data`
in the hand-written collection decoders. Dialer coverage and the other audited
commands now report partial results with API-class failures instead of treating
error-bearing HTTP 200 responses as empty successful data.
