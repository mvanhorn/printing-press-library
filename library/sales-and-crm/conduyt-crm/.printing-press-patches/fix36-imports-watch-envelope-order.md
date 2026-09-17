# Fix 36: imports watch envelope order

Guard import job, launch-monitor, and verification responses for API error envelopes before decoding or unwrapping them. Regression tests cover top-level and data-nested string/object `error` and `message` envelopes carrying otherwise valid terminal data.
