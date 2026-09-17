# Fix 34: coverage and scorecard response shapes

Require the documented non-null collection arrays in dialer coverage and reports
scorecard envelopes, while preserving explicit empty arrays. Invalid, missing,
and error-shaped responses now emit partial metadata and return API-class errors;
regression tests cover both scorecard endpoints and dial-order decoding.
