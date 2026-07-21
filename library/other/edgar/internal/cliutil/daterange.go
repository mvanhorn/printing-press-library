// Copyright 2026 magoo242 and contributors. Licensed under Apache-2.0. See LICENSE.
// PATCH(amend-efts-date-range): hand-added helper — see .printing-press-patches/.

package cliutil

import "strings"

// SplitDateRange splits a combined "start,end" date-range flag value into its
// two bounds, trimming surrounding whitespace on each side. Either bound may be
// empty to express an open-ended range:
//
//	"2024-01-01,2026-05-13" -> ("2024-01-01", "2026-05-13")
//	"2024-01-01,"           -> ("2024-01-01", "")
//	",2026-05-13"           -> ("",           "2026-05-13")
//	"2024-01-01"            -> ("2024-01-01", "")   // no comma: start-only
//
// It exists because efts.sec.gov's full-text search endpoint filters by two
// separate query params (startdt, enddt) rather than a single combined range
// value; callers map the two returned bounds onto those params. Sending the
// combined "start,end" string as one param is silently ignored by the API.
func SplitDateRange(s string) (start, end string) {
	parts := strings.SplitN(s, ",", 2)
	start = strings.TrimSpace(parts[0])
	if len(parts) == 2 {
		end = strings.TrimSpace(parts[1])
	}
	return start, end
}
