package grants

import (
	"encoding/json"
	"fmt"

	"github.com/silentnw/govprocure-pp-cli/internal/api"
	"github.com/silentnw/govprocure-pp-cli/internal/config"
	"github.com/silentnw/govprocure-pp-cli/internal/db"
	"github.com/silentnw/govprocure-pp-cli/internal/output"
	"github.com/spf13/cobra"
)

func newSyncCmd(opts *output.Options) *cobra.Command {
	var keyword string
	var maxRecords int

	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Pull latest opportunities from grants.gov into local SQLite",
		Example: `  govprocure-pp-cli grants sync
  govprocure-pp-cli grants sync --keyword "AI technology"
  govprocure-pp-cli grants sync --max 200`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}

			if opts.DryRun {
				output.DryRunMsg(fmt.Sprintf("grants sync keyword=%q max=%d", keyword, maxRecords))
				return nil
			}

			database, err := db.Open(cfg.DBPath)
			if err != nil {
				return fmt.Errorf("open database: %w", err)
			}
			defer database.Close()

			fmt.Printf("Syncing grants.gov (keyword=%q, max=%d)...\n", keyword, maxRecords)
			client := api.NewGrantsClient(cfg.GrantsURL)
			opps, err := client.SyncAll(keyword, maxRecords)
			if err != nil {
				_ = database.LogSync("grants.gov", 0, "error", err.Error())
				return fmt.Errorf("grants.gov sync: %w", err)
			}

			count := 0
			for _, opp := range opps {
				raw, _ := json.Marshal(opp)
				g := &db.Grant{
					OpportunityID: opp.Number,
					Title:         opp.Title,
					Agency:        opp.Agency,
					CFDANumber:    api.CFDAString(opp.CFDAList),
					CloseDate:     opp.CloseDate,
					PostDate:      opp.OpenDate,
					Eligibility:   opp.EligApplicants,
					Synopsis:      opp.Synopsis,
					AwardFloor:    opp.AwardFloor,
					AwardCeiling:  opp.AwardCeiling,
					RawJSON:       string(raw),
				}
				if err := database.UpsertGrant(g); err != nil {
					output.Warn("upsert %s: %v", opp.Number, err)
					continue
				}
				count++
			}

			_ = database.LogSync("grants.gov", count, "ok", "")

			if output.EffectiveFormat(opts) == output.FormatJSON {
				output.PrintJSON(map[string]interface{}{
					"source":  "grants.gov",
					"synced":  count,
					"status":  "ok",
				})
			} else {
				fmt.Printf("✓ Synced %d grants from grants.gov\n", count)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&keyword, "keyword", "", "keyword filter for sync")
	cmd.Flags().IntVar(&maxRecords, "max", 500, "maximum records to sync")
	return cmd
}
