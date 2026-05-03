package airbnb

import "errors"

// ErrNotFound is returned when an Airbnb listing fetch resolves to no
// listing — typically because the ID is invalid or the listing was removed.
// Pre-fix the source layer returned the bare string
// "Airbnb deferred state script not found" which the CLI mapped to a
// generic exit code 0.
var ErrNotFound = errors.New("airbnb listing not found")
