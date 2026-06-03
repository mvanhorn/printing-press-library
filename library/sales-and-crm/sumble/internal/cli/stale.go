package cli

import (
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

type staleRow struct {
	Type     string `json:"type"`
	ID       string `json:"id"`
	Label    string `json:"label"`
	SyncedAt string `json:"synced_at"`
	AgeHours int    `json:"age_hours"`
}

// staleTables maps each cached entity type to the JSON field used as a
// human-readable label.
var staleTables = []struct {
	typ        string
	labelField string
}{
	{"organizations", "name"},
	{"people", "name"},
	{"postings", "job_title"},
	{"technologies", "name"},
}

func newStaleCmd(flags *rootFlags) *cobra.Command {
	var olderThan, typ string

	cmd := &cobra.Command{
		Use:   "stale",
		Short: "List cached entities older than Sumble's freshness window",
		Long: strings.Trim(`
List cached organizations, people, postings, and technologies whose local copy
is older than a cutoff (default 24h, matching Sumble's ~24h data freshness lag).
Use this to re-bill only the rows that are actually stale instead of refreshing
the whole cache.

--older-than accepts a Go duration (e.g. 24h, 90m) or a day count (e.g. 7d).
`, "\n"),
		Example: strings.Trim(`
  sumble-pp-cli stale
  sumble-pp-cli stale --older-than 7d --type organizations
  sumble-pp-cli stale --older-than 24h --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			cutoff, err := parseAgeDuration(olderThan)
			if err != nil {
				return usageErr(err)
			}
			// PATCH(stale-type-validation): an unknown --type value (e.g.
			// "organization" singular, typo) used to silently match nothing
			// and return an empty list. Fail loud with the valid set so the
			// user sees what's allowed.
			if typ != "" {
				known := make([]string, 0, len(staleTables))
				ok := false
				for _, t := range staleTables {
					known = append(known, t.typ)
					if typ == t.typ {
						ok = true
					}
				}
				if !ok {
					return usageErr(fmt.Errorf("unknown --type %q (valid: %s)", typ, strings.Join(known, ", ")))
				}
			}
			db, derr := openCreditStore()
			if derr != nil {
				return configErr(derr)
			}
			defer db.Close()

			threshold := time.Now().UTC().Add(-cutoff)
			var out []staleRow
			for _, t := range staleTables {
				if typ != "" && typ != t.typ {
					continue
				}
				q := fmt.Sprintf(
					`SELECT id, COALESCE(json_extract(data, '$.%s'), ''), synced_at
					 FROM %s WHERE synced_at < ? ORDER BY synced_at ASC`, t.labelField, t.typ)
				rows, qerr := db.DB().Query(q, threshold.Format("2006-01-02 15:04:05"))
				if qerr != nil {
					continue
				}
				for rows.Next() {
					var id, label string
					var syncedAt sql.NullString
					if err := rows.Scan(&id, &label, &syncedAt); err != nil {
						continue
					}
					age := 0
					if syncedAt.Valid {
						if ts, perr := time.Parse("2006-01-02 15:04:05", syncedAt.String); perr == nil {
							age = int(time.Since(ts).Hours())
						}
					}
					out = append(out, staleRow{Type: t.typ, ID: id, Label: label, SyncedAt: syncedAt.String, AgeHours: age})
				}
				rows.Close()
			}

			if flags.asJSON {
				return flags.printJSON(cmd, map[string]any{
					"older_than": olderThan,
					"count":      len(out),
					"stale":      out,
				})
			}
			w := cmd.OutOrStdout()
			if len(out) == 0 {
				fmt.Fprintf(w, "No cached entities older than %s.\n", olderThan)
				return nil
			}
			fmt.Fprintf(w, "%-14s %-12s %6s  %s\n", "TYPE", "ID", "AGE(h)", "LABEL")
			for _, r := range out {
				fmt.Fprintf(w, "%-14s %-12s %6d  %s\n", r.Type, r.ID, r.AgeHours, truncate(r.Label, 50))
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&olderThan, "older-than", "24h", "Age cutoff: Go duration (24h, 90m) or day count (7d)")
	cmd.Flags().StringVar(&typ, "type", "", "Restrict to one type: organizations, people, postings, technologies")
	return cmd
}

// parseAgeDuration accepts a Go duration (24h) or a day count (7d).
func parseAgeDuration(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 24 * time.Hour, nil
	}
	if strings.HasSuffix(s, "d") {
		days, err := strconv.Atoi(strings.TrimSuffix(s, "d"))
		if err != nil || days < 0 {
			return 0, fmt.Errorf("invalid day count %q", s)
		}
		return time.Duration(days) * 24 * time.Hour, nil
	}
	d, err := time.ParseDuration(s)
	if err != nil || d < 0 {
		return 0, fmt.Errorf("invalid duration %q (use forms like 24h, 90m, or 7d)", s)
	}
	return d, nil
}
