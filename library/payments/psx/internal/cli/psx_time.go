// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import "time"

// snapshotTimeFormat stamps snapshot rows and the cutoffs compared against them.
//
// Two properties matter, and both rule out the obvious choices:
//
//   - Nanosecond precision. psx_snapshots is keyed by (taken_at, kind, symbol)
//     and written with INSERT OR REPLACE, so two captures of the same kind
//     inside one tick of the timestamp silently overwrite each other and delete
//     retained history. time.RFC3339 is second-resolution, which makes that
//     reachable for concurrent or rapid captures.
//   - Fixed width. taken_at is compared as TEXT (lexicographic ordering must
//     equal chronological ordering). time.RFC3339Nano strips trailing zeros, so
//     its width varies and string ordering can disagree with time ordering.
//
// This layout keeps all nine fractional digits, so it satisfies both.
const snapshotTimeFormat = "2006-01-02T15:04:05.000000000Z07:00"

// rfc3339 is retained for display-only formatting where precision is noise.
const rfc3339 = time.RFC3339

// nowUTC is a seam so time-dependent novel commands stay testable.
var nowUTC = func() time.Time { return time.Now().UTC() }
