// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import "time"

const rfc3339 = time.RFC3339

// nowUTC is a seam so time-dependent novel commands stay testable.
var nowUTC = func() time.Time { return time.Now().UTC() }
