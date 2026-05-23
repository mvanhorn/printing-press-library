package store

import "time"

// timeNow is overridable in tests if needed.
var timeNow = time.Now
