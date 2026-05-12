// Hand-authored: novel feature `oversized` — flag computers whose
// CPU/RAM is high relative to their actual usage, and whose auto-stop
// is disabled or generously long. Created during Phase 3 build;
// survives regen because it lives in its own file.
package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/cloud/orgo/internal/store"

	"github.com/spf13/cobra"
)

func newOversizedCmd(flags *rootFlags) *cobra.Command {
	var minCores int
	var idleDays float64
	var workspace string

	cmd := &cobra.Command{
		Use:   "oversized",
		Short: "Flag computers with CPU >= --min-cores or RAM >= 16 GB whose last CLI-recorded action is older than the threshold and whose auto-stop is disabled or large.",
		Long: `Crawl the local computers store, join with the actions ledger, and
flag downsize candidates: high CPU or RAM specs with low recent usage
and auto-stop either off or set well above an hour.

The auto_stop_minutes field is read from the cached computer record
when the API or sync supplied it. Computers without an explicit value
are assumed to be running with the default (>= 60 minutes), which the
filter treats as "not aggressive enough to count as auto-stopped".`,
		Example: strings.Trim(`
  # Default: 4+ cores or 16+ GB RAM, idle > 7 days, weak auto-stop
  orgo-pp-cli oversized

  # Tighter: anything with 8+ cores idle for more than 14 days
  orgo-pp-cli oversized --min-cores 8 --idle-days 14
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}

			dbPath := defaultDBPath("orgo-pp-cli")
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			defer db.Close()

			computerRows, source, err := loadFleetComputers(flags, db)
			if err != nil {
				return fmt.Errorf("loading computers: %w", err)
			}
			if len(computerRows) == 0 {
				fmt.Fprintln(cmd.ErrOrStderr(), "No computers found locally or via the live API.")
				return notFoundErr(fmt.Errorf("no computers found"))
			}
			_ = source // provenance label, unused for now

			lastTimes, err := db.LastActionTimes(cmd.Context())
			if err != nil {
				return fmt.Errorf("loading last action times: %w", err)
			}

			now := time.Now().UTC()
			cutoff := time.Duration(idleDays * 24 * float64(time.Hour))

			type oversizedRow struct {
				ID              string  `json:"id"`
				Name            string  `json:"name"`
				CPU             int64   `json:"cpu"`
				RAM             int64   `json:"ram"`
				DaysIdle        float64 `json:"days_idle"`
				AutoStopMinutes any     `json:"auto_stop_minutes"`
				WorkspaceID     string  `json:"workspace_id,omitempty"`
			}

			var rows []oversizedRow
			for _, raw := range computerRows {
				var obj map[string]any
				if err := json.Unmarshal(raw, &obj); err != nil {
					continue
				}
				id := stringField(obj, "id")
				if id == "" {
					continue
				}
				if workspace != "" && stringField(obj, "workspace_id") != workspace {
					continue
				}
				cpu := intField(obj, "cpu")
				ram := intField(obj, "ram")

				// Brief: filter by CPU >= min-cores OR RAM >= 16
				if cpu < int64(minCores) && ram < 16 {
					continue
				}

				// Idle filter: never-seen counts as infinitely idle.
				var daysIdle float64
				if last, ok := lastTimes[id]; ok && !last.IsZero() {
					age := now.Sub(last)
					daysIdle = age.Hours() / 24
					if age < cutoff {
						continue
					}
				} else {
					daysIdle = -1
				}

				// Auto-stop: null OR >= 60 minutes counts as oversized risk.
				autoStopRaw := store.LookupFieldValue(obj, "auto_stop_minutes")
				if autoStopRaw != nil {
					if minutes := toInt64(autoStopRaw); minutes < 60 && minutes > 0 {
						continue
					}
				}

				rows = append(rows, oversizedRow{
					ID:              id,
					Name:            stringField(obj, "name"),
					CPU:             cpu,
					RAM:             ram,
					DaysIdle:        daysIdle,
					AutoStopMinutes: autoStopRaw,
					WorkspaceID:     stringField(obj, "workspace_id"),
				})
			}

			// Sort by RAM desc, then CPU desc.
			sort.SliceStable(rows, func(i, j int) bool {
				if rows[i].RAM != rows[j].RAM {
					return rows[i].RAM > rows[j].RAM
				}
				return rows[i].CPU > rows[j].CPU
			})

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
			}
			if len(rows) == 0 {
				fmt.Fprintln(cmd.ErrOrStderr(), "No oversized computers above the threshold.")
				return nil
			}
			tw := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintln(tw, bold("ID")+"\t"+bold("NAME")+"\t"+bold("CPU")+"\t"+bold("RAM")+"\t"+bold("DAYS_IDLE")+"\t"+bold("AUTO_STOP_MIN"))
			for _, r := range rows {
				days := fmt.Sprintf("%.1f", r.DaysIdle)
				if r.DaysIdle < 0 {
					days = "never"
				}
				autoStop := "—"
				if r.AutoStopMinutes != nil {
					autoStop = fmt.Sprintf("%v", r.AutoStopMinutes)
				}
				fmt.Fprintf(tw, "%s\t%s\t%d\t%d\t%s\t%s\n", truncate(r.ID, 12), r.Name, r.CPU, r.RAM, days, autoStop)
			}
			return tw.Flush()
		},
	}

	cmd.Flags().IntVar(&minCores, "min-cores", 4, "Minimum CPU cores to consider a computer oversized (or RAM >= 16 GB triggers regardless)")
	cmd.Flags().Float64Var(&idleDays, "idle-days", 7, "Days since last CLI-recorded action before counting as idle")
	cmd.Flags().StringVar(&workspace, "workspace", "", "Filter by workspace ID")

	return cmd
}

// intField extracts an integer from a JSON object, accepting numeric or
// string-encoded values. Returns 0 when missing or unparsable.
func intField(obj map[string]any, key string) int64 {
	v := store.LookupFieldValue(obj, key)
	return toInt64(v)
}

func toInt64(v any) int64 {
	switch n := v.(type) {
	case nil:
		return 0
	case int:
		return int64(n)
	case int64:
		return n
	case float64:
		return int64(n)
	case string:
		// best-effort parse
		var x int64
		_, _ = fmt.Sscanf(n, "%d", &x)
		return x
	default:
		return 0
	}
}
