package cli

import (
	"database/sql"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/chrome-history/internal/source"
)

func visitFilterForWindow(start, end time.Time, opts *RootOptions) source.VisitFilter {
	return source.VisitFilter{Since: start, Until: end, Limit: opts.Output.Limit, Device: opts.Device}
}

func maybePrintEmptyWindowHint(db *sql.DB, since string, isEmpty bool) {
	if !isEmpty || strings.TrimSpace(since) == "" {
		return
	}
	var maxVisit int64
	if err := db.QueryRow(`SELECT COALESCE(MAX(visit_time),0) FROM visits`).Scan(&maxVisit); err != nil || maxVisit <= 0 {
		return
	}
	ts := chromeMicrosToTime(maxVisit).In(time.Local).Format("2006-01-02 15:04")
	_, _ = fmt.Fprintf(os.Stderr, "no activity in %s; most recent activity: %s; try --since 30d\n", strings.TrimSpace(since), ts)
}

const chromeEpochOffsetSeconds int64 = 11644473600

func chromeMicrosToTime(chromeMicros int64) time.Time {
	if chromeMicros <= 0 {
		return time.Time{}
	}
	unixSeconds := chromeMicros/1_000_000 - chromeEpochOffsetSeconds
	remMicros := chromeMicros % 1_000_000
	return time.Unix(unixSeconds, remMicros*1_000).UTC()
}
