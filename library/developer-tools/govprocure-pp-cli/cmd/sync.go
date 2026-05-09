package cmd

import (
	"fmt"

	"github.com/silentnw/govprocure-pp-cli/internal/config"
	"github.com/silentnw/govprocure-pp-cli/internal/db"
	"github.com/silentnw/govprocure-pp-cli/internal/api"
	"github.com/silentnw/govprocure-pp-cli/internal/output"
	"github.com/spf13/cobra"
	"encoding/json"
)

func newSyncAllCmd() *cobra.Command {
	var all bool
	var keyword string

	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Sync all data sources to local SQLite",
		Long:  `Pull latest records from grants.gov, SAM.gov, and USASpending.gov into the local database.`,
		Example: `  govprocure-pp-cli sync --all
  govprocure-pp-cli sync --all --keyword "AI consulting"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if !all {
				return fmt.Errorf("use --all to sync all sources, or use source-specific sync commands (e.g. grants sync)")
			}

			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}

			if GlobalOpts.DryRun {
				output.DryRunMsg("sync all sources: grants.gov, SAM.gov, USASpending.gov")
				return nil
			}

			database, err := db.Open(cfg.DBPath)
			if err != nil {
				return fmt.Errorf("open database: %w", err)
			}
			defer database.Close()

			type syncResult struct {
				Source  string `json:"source"`
				Records int    `json:"records"`
				Status  string `json:"status"`
				Error   string `json:"error,omitempty"`
			}
			var results []syncResult

			// Sync grants.gov
			fmt.Println("Syncing grants.gov...")
			grantsCount, grantsErr := syncGrants(database, cfg, keyword)
			sr := syncResult{Source: "grants.gov", Records: grantsCount, Status: "ok"}
			if grantsErr != nil {
				sr.Status = "error"
				sr.Error = grantsErr.Error()
			}
			results = append(results, sr)
			reportSync("grants.gov", grantsCount, grantsErr)

			// Sync SAM.gov
			fmt.Println("Syncing SAM.gov...")
			samCount, samErr := syncSAM(database, cfg, keyword)
			sr = syncResult{Source: "SAM.gov", Records: samCount, Status: "ok"}
			if samErr != nil {
				sr.Status = "error"
				sr.Error = samErr.Error()
			}
			results = append(results, sr)
			reportSync("SAM.gov", samCount, samErr)

			// Sync USASpending (awards for cross-reference)
			fmt.Println("Syncing USASpending.gov...")
			usaCount, usaErr := syncUSASpending(database, cfg, keyword)
			sr = syncResult{Source: "USASpending.gov", Records: usaCount, Status: "ok"}
			if usaErr != nil {
				sr.Status = "error"
				sr.Error = usaErr.Error()
			}
			results = append(results, sr)
			reportSync("USASpending.gov", usaCount, usaErr)

			if output.EffectiveFormat(GlobalOpts) == output.FormatJSON {
				output.PrintJSON(results)
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&all, "all", false, "sync all sources")
	cmd.Flags().StringVar(&keyword, "keyword", "", "filter keyword for sync (default: broad fetch)")
	return cmd
}

func syncGrants(database *db.DB, cfg *config.Config, keyword string) (int, error) {
	client := api.NewGrantsClient(cfg.GrantsURL)
	opps, err := client.SyncAll(keyword, cfg.SyncDays*10)
	if err != nil {
		_ = database.LogSync("grants.gov", 0, "error", err.Error())
		return 0, err
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
			output.Warn("grants upsert %s: %v", opp.Number, err)
			continue
		}
		count++
	}
	_ = database.LogSync("grants.gov", count, "ok", "")
	return count, nil
}

func syncSAM(database *db.DB, cfg *config.Config, keyword string) (int, error) {
	if cfg.SAMAPIKey == "" {
		return 0, fmt.Errorf("SAM API key not configured — run: govprocure-pp-cli auth set-key --sam KEY")
	}
	client := api.NewSAMClient(cfg.SAMURL, cfg.SAMAPIKey)
	opps, err := client.SyncAll(keyword, "", 500)
	if err != nil {
		_ = database.LogSync("sam.gov", 0, "error", err.Error())
		return 0, err
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
			output.Warn("SAM upsert %s: %v", opp.NoticeID, err)
			continue
		}
		count++
	}
	_ = database.LogSync("sam.gov", count, "ok", "")
	return count, nil
}

func syncUSASpending(database *db.DB, cfg *config.Config, keyword string) (int, error) {
	client := api.NewUSASpendingClient(cfg.USASpendURL)
	var agencies []string
	if keyword != "" {
		agencies = []string{keyword}
	}
	resp, err := client.SearchAwards(agencies, nil, nil, 100)
	if err != nil {
		_ = database.LogSync("usaspending.gov", 0, "error", err.Error())
		return 0, err
	}

	count := 0
	for _, award := range resp.Results {
		raw, _ := json.Marshal(award)
		a := &db.Award{
			AwardID:       award.AwardID,
			RecipientName: award.RecipientName,
			Agency:        award.Agency,
			CFDANumber:    award.CFDANumber,
			Amount:        award.AwardAmount,
			StartDate:     award.StartDate,
			EndDate:       award.EndDate,
			Description:   award.Description,
			RawJSON:       string(raw),
		}
		if err := database.UpsertAward(a); err != nil {
			output.Warn("award upsert %s: %v", award.AwardID, err)
			continue
		}
		count++
	}
	_ = database.LogSync("usaspending.gov", count, "ok", "")
	return count, nil
}

func reportSync(source string, count int, err error) {
	if err != nil {
		fmt.Printf("  ✗ %s: %v\n", source, err)
	} else {
		fmt.Printf("  ✓ %s: %d records synced\n", source, count)
	}
}
