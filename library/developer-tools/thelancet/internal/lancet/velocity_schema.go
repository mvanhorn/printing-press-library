package lancet

import (
	"context"
	"database/sql"
	"fmt"
)

// workMetricColumn is a lancet_works column added after the first release.
type workMetricColumn struct{ name, typ string }

// workMetricColumns: OpenAlex's field-weighted citation impact and the value of
// its citation_normalized_percentile (display-only), and counts_synced_at, the
// UTC time the work's yearly counts were last fetched (NULL = never, so its
// velocity is no-data rather than zero). All are nullable.
var workMetricColumns = []workMetricColumn{
	{"fwci", "REAL"},
	{"citation_normalized_percentile", "REAL"},
	{"counts_synced_at", "TEXT"},
}

// yearCountsDDL holds the yearly citation counts that --sort velocity ranks by.
// Years with no citations are simply absent (OpenAlex omits them too).
const yearCountsDDL = `CREATE TABLE IF NOT EXISTS lancet_work_year_counts (
	work_id        TEXT NOT NULL,
	year           INTEGER NOT NULL,
	cited_by_count INTEGER NOT NULL,
	PRIMARY KEY (work_id, year)
)`

func worksColumns(ctx context.Context, q interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}) (map[string]bool, error) {
	rows, err := q.QueryContext(ctx, `PRAGMA table_info(lancet_works)`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	cols := map[string]bool{}
	for rows.Next() {
		var cid, notnull, pk int
		var name, typ string
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &typ, &notnull, &dflt, &pk); err != nil {
			return nil, err
		}
		cols[name] = true
	}
	return cols, rows.Err()
}

// ensureWorkMetricColumns adds the metric columns to a store created before
// they existed. Like the FTS setup it runs under BEGIN IMMEDIATE and re-checks
// under the write lock, so two processes opening the same old mirror cannot
// both try to add a column.
func ensureWorkMetricColumns(ctx context.Context, db *sql.DB) error {
	missing := func(have map[string]bool) []workMetricColumn {
		var out []workMetricColumn
		for _, c := range workMetricColumns {
			if !have[c.name] {
				out = append(out, c)
			}
		}
		return out
	}
	have, err := worksColumns(ctx, db)
	if err != nil {
		return fmt.Errorf("lancet schema: %w", err)
	}
	if len(missing(have)) == 0 {
		return nil
	}
	conn, err := db.Conn(ctx)
	if err != nil {
		return fmt.Errorf("lancet schema: %w", err)
	}
	defer conn.Close()
	var prevBusy int
	if err := conn.QueryRowContext(ctx, `PRAGMA busy_timeout`).Scan(&prevBusy); err != nil {
		return fmt.Errorf("lancet schema: %w", err)
	}
	defer func() {
		_, _ = conn.ExecContext(context.Background(), fmt.Sprintf(`PRAGMA busy_timeout = %d`, prevBusy))
	}()
	if _, err := conn.ExecContext(ctx, `PRAGMA busy_timeout = 30000`); err != nil {
		return fmt.Errorf("lancet schema: %w", err)
	}
	if _, err := conn.ExecContext(ctx, `BEGIN IMMEDIATE`); err != nil {
		return fmt.Errorf("lancet schema: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_, _ = conn.ExecContext(context.Background(), `ROLLBACK`)
		}
	}()
	have, err = worksColumns(ctx, conn)
	if err != nil {
		return fmt.Errorf("lancet schema: %w", err)
	}
	for _, c := range missing(have) {
		if _, err := conn.ExecContext(ctx, `ALTER TABLE lancet_works ADD COLUMN `+c.name+` `+c.typ); err != nil {
			return fmt.Errorf("lancet schema: add column %s: %w", c.name, err)
		}
	}
	if _, err := conn.ExecContext(ctx, `COMMIT`); err != nil {
		return fmt.Errorf("lancet schema commit: %w", err)
	}
	committed = true
	return nil
}
