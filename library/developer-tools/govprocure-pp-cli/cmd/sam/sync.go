package sam

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
	var setAside string

	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Pull latest SAM.gov opportunities into local SQLite",
		Example: `  govprocure-pp-cli sam sync
  govprocure-pp-cli sam sync --keyword "IT services" --set-aside sdvosb`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}
			if cfg.SAMAPIKey == "" {
				return fmt.Errorf("SAM API key not configured — run: govprocure-pp-cli auth set-key --sam KEY")
			}

			if opts.DryRun {
				output.DryRunMsg(fmt.Sprintf("sam sync keyword=%q set-aside=%q", keyword, setAside))
				return nil
			}

			database, err := db.Open(cfg.DBPath)
			if err != nil {
				return fmt.Errorf("open database: %w", err)
			}
			defer database.Close()

			var saCode string
			if setAside != "" {
				if code, ok := api.SetAsideCodes[setAside]; ok {
					saCode = code
				} else {
					saCode = setAside
				}
			}

			fmt.Printf("Syncing SAM.gov (keyword=%q, set-aside=%q)...\n", keyword, saCode)
			client := api.NewSAMClient(cfg.SAMURL, cfg.SAMAPIKey)
			opps, err := client.SyncAll(keyword, saCode, 500)
			if err != nil {
				_ = database.LogSync("sam.gov", 0, "error", err.Error())
				return fmt.Errorf("SAM.gov sync: %w", err)
			}

			count := 0
			for _, opp := range opps {
				raw, _ := json.Marshal(opp)
				s := &db.SAMOpportunity{
					NoticeID:           opp.NoticeID,
					Title:              opp.Title,
					Agency:             api.AgencyFromSAM(&opp),
					SubTier:            opp.SubTier,
					NAICSCode:          opp.NAICSCode,
					SetAside:           opp.TypeOfSetAsideCode,
					ResponseDeadline:   opp.ResponseDeadLine,
					PostedDate:         opp.PostedDate,
					Description:        opp.Description,
					SolicitationNumber: opp.SolicitationNumber,
					RawJSON:            string(raw),
				}
				if err := database.UpsertSAM(s); err != nil {
					output.Warn("upsert %s: %v", opp.NoticeID, err)
					continue
				}
				count++
			}

			_ = database.LogSync("sam.gov", count, "ok", "")

			if output.EffectiveFormat(opts) == output.FormatJSON {
				output.PrintJSON(map[string]interface{}{
					"source":  "sam.gov",
					"synced":  count,
					"status":  "ok",
				})
			} else {
				fmt.Printf("✓ Synced %d SAM opportunities\n", count)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&keyword, "keyword", "", "keyword filter")
	cmd.Flags().StringVar(&setAside, "set-aside", "", "set-aside filter: sdvosb|wosb|8a|hubzone|sba")
	return cmd
}
