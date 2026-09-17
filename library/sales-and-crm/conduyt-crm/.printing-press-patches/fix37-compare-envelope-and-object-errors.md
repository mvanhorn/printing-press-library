# Fix 37: compare envelope and object errors

Guard report comparison responses before decoding or unwrapping them. The shared guard now rejects every non-null `error` value and string/object `message` envelopes, with regression coverage for current, prior, top-level, nested, and audited handcrafted response paths.
