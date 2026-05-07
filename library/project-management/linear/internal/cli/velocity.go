package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/project-management/linear/internal/store"

	"github.com/spf13/cobra"
)

func newVelocityCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var jsonOut bool
	var weeks int
	cmd := &cobra.Command{
		Use:         "velocity",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short:       "Show sprint velocity trends over recent cycles",
		Long:        "Display completed vs planned scope for recent cycles to track team velocity trends.",
		Example: `  linear-pp-cli velocity
  linear-pp-cli velocity --weeks 8
  linear-pp-cli velocity --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dbPath == "" {
				dbPath = defaultDBPath("linear-pp-cli")
			}
			db, err := store.Open(dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w\nRun 'linear-pp-cli sync' first.", err)
			}
			defer db.Close()

			cycles, err := db.ListCycles("")
			if err != nil {
				return err
			}

			type cycleInfo struct {
				Name           string               `json:"name"`
				Number         int                  `json:"number"`
				Team           struct{ Key string } `json:"team"`
				ScopeCount     int                  `json:"scopeCount"`
				CompletedCount int                  `json:"completedScopeCount"`
				Progress       float64              `json:"progress"`
				StartsAt       string               `json:"startsAt"`
				EndsAt         string               `json:"endsAt"`
				CompletedAt    string               `json:"completedAt"`
			}

			var infos []cycleInfo
			for _, raw := range cycles {
				var ci cycleInfo
				json.Unmarshal(raw, &ci)
				if ci.ScopeCount > 0 {
					infos = append(infos, ci)
				}
			}

			sort.Slice(infos, func(i, j int) bool {
				return infos[i].StartsAt > infos[j].StartsAt
			})

			if weeks > 0 && len(infos) > weeks {
				infos = infos[:weeks]
			}

			if jsonOut {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(infos)
			}

			if len(infos) == 0 {
				fmt.Println("No cycles with scope data found. Run 'sync' first.")
				return nil
			}

			fmt.Printf("%-6s %-5s %-10s %-8s %-8s %-8s %s\n", "CYCLE", "TEAM", "STARTS", "SCOPE", "DONE", "RATE", "BAR")
			fmt.Println(strings.Repeat("-", 75))
			for _, ci := range infos {
				rate := 0.0
				if ci.ScopeCount > 0 {
					rate = float64(ci.CompletedCount) / float64(ci.ScopeCount) * 100
				}
				bar := strings.Repeat("=", int(rate/5)) + strings.Repeat(" ", 20-int(rate/5))
				startDate := ""
				if len(ci.StartsAt) >= 10 {
					startDate = ci.StartsAt[:10]
				}
				name := ci.Name
				if name == "" {
					name = fmt.Sprintf("#%d", ci.Number)
				}
				fmt.Printf("%-6s %-5s %-10s %-8d %-8d %5.0f%%  [%s]\n", name, ci.Team.Key, startDate, ci.ScopeCount, ci.CompletedCount, rate, bar)
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&weeks, "weeks", 8, "Number of recent cycles to show")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "Output as JSON")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}
