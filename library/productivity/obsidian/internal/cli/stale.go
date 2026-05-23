package cli

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/productivity/obsidian/internal/cliutil"
)

func newStaleCmd(flags *rootFlags) *cobra.Command {
	var typeFilter, olderThan, statusFilter string
	cmd := &cobra.Command{
		Use:   "stale",
		Short: "List notes whose mtime predates a threshold (find abandoned active entities).",
		Long: "Find notes that haven't been touched in a while. Useful for promoting\n" +
			"stale meeting/journal entries to Patterns, or for finding active\n" +
			"entities that have gone cold and should move to status=paused.",
		Example:     "  obsidian-pp-cli stale --type meeting --older-than 90d --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if cliutil.IsVerifyEnv() {
				return nil
			}
			if dryRunOK(flags) {
				return nil
			}
			vc, err := openVaultAndStore(cmd.Context(), flags)
			if err != nil {
				return err
			}
			defer vc.Close()
			cutoff := time.Now()
			if olderThan != "" {
				dur, err := parseDuration(olderThan)
				if err != nil {
					return usageErr(fmt.Errorf("invalid --older-than: %w", err))
				}
				cutoff = time.Now().Add(-dur)
			} else {
				cutoff = time.Now().AddDate(0, 0, -90)
			}
			where := []string{"mtime < ?"}
			argsList := []interface{}{cutoff.Unix()}
			if typeFilter != "" {
				where = append(where, "type = ?")
				argsList = append(argsList, typeFilter)
			}
			if statusFilter != "" {
				where = append(where, "status = ?")
				argsList = append(argsList, statusFilter)
			}
			q := `SELECT path, COALESCE(type,''), COALESCE(status,''), mtime, COALESCE(description,'') FROM notes WHERE ` +
				strings.Join(where, " AND ") + " ORDER BY mtime"
			rows, err := vc.S.DB().QueryContext(cmd.Context(), q, argsList...)
			if err != nil {
				return apiErr(err)
			}
			defer rows.Close()
			type entry struct {
				Path        string `json:"path"`
				Type        string `json:"type,omitempty"`
				Status      string `json:"status,omitempty"`
				Mtime       string `json:"mtime"`
				DaysStale   int    `json:"days_stale"`
				Description string `json:"description,omitempty"`
			}
			var out []entry
			for rows.Next() {
				var e entry
				var mt int64
				if err := rows.Scan(&e.Path, &e.Type, &e.Status, &mt, &e.Description); err != nil {
					return apiErr(err)
				}
				e.Mtime = time.Unix(mt, 0).Format("2006-01-02")
				e.DaysStale = int(time.Since(time.Unix(mt, 0)).Hours() / 24)
				out = append(out, e)
			}
			if flags.asJSON {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(out)
			}
			for _, e := range out {
				fmt.Fprintf(cmd.OutOrStdout(), "%s (%s, %d days stale): %s\n", e.Path, e.Type, e.DaysStale, e.Description)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&typeFilter, "type", "", "Filter by note type")
	cmd.Flags().StringVar(&statusFilter, "status", "", "Filter by status (e.g. active)")
	cmd.Flags().StringVar(&olderThan, "older-than", "", "Duration: 7d, 30d, 90d, 6m, 1y (default 90d)")
	return cmd
}

// parseDuration extends time.ParseDuration with day/month/year shorthands.
func parseDuration(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("empty duration")
	}
	if d, err := time.ParseDuration(s); err == nil {
		return d, nil
	}
	if len(s) >= 2 {
		suffix := s[len(s)-1]
		nStr := s[:len(s)-1]
		n, err := strconv.Atoi(nStr)
		if err != nil {
			return 0, err
		}
		switch suffix {
		case 'd':
			return time.Duration(n) * 24 * time.Hour, nil
		case 'w':
			return time.Duration(n) * 7 * 24 * time.Hour, nil
		case 'm':
			return time.Duration(n) * 30 * 24 * time.Hour, nil
		case 'y':
			return time.Duration(n) * 365 * 24 * time.Hour, nil
		}
	}
	return 0, fmt.Errorf("unknown duration format: %s", s)
}
