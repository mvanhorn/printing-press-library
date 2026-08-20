package cli

import (
	"database/sql"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/safari-history/internal/source"
)

func visitFilterForWindow(start, end time.Time, opts *RootOptions) source.VisitFilter {
	return source.VisitFilter{Since: start, Until: end, Limit: opts.Output.Limit, Device: opts.Device}
}

func maybePrintEmptyWindowHint(db *sql.DB, since string, isEmpty bool) {
	if !isEmpty || strings.TrimSpace(since) == "" {
		return
	}
	var maxVisit float64
	if err := db.QueryRow(`SELECT COALESCE(MAX(visit_time),0) FROM history_visits`).Scan(&maxVisit); err != nil || maxVisit <= 0 {
		return
	}
	ts := source.SafariSecondsToTime(maxVisit).In(time.Local).Format("2006-01-02 15:04")
	_, _ = fmt.Fprintf(os.Stderr, "no activity in %s; most recent activity: %s; try --since 30d\n", strings.TrimSpace(since), ts)
}
