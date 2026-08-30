package fishaudio

import "regexp"

var referenceIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,128}$`)

// ValidReferenceID reports whether s satisfies the API's reference_id rule
// (1..=128 chars of [A-Za-z0-9_-]). Checking client-side avoids a paid call
// that the server would reject with HTTP 400.
func ValidReferenceID(s string) bool { return referenceIDPattern.MatchString(s) }
