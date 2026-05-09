package sam

import (
	"fmt"

	"github.com/silentnw/govprocure-pp-cli/internal/api"
	"github.com/silentnw/govprocure-pp-cli/internal/config"
	"github.com/silentnw/govprocure-pp-cli/internal/db"
	"github.com/silentnw/govprocure-pp-cli/internal/output"
	"github.com/spf13/cobra"
)

func newGetCmd(opts *output.Options) *cobra.Command {
	return &cobra.Command{
		Use:   "get <notice-id>",
		Short: "Fetch a single SAM.gov notice",
		Args:  cobra.ExactArgs(1),
		Example: `  govprocure-pp-cli sam get abc123def456
  govprocure-pp-cli sam get abc123def456 --agent`,
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]
			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}

			if opts.DryRun {
				output.DryRunMsg(fmt.Sprintf("sam get %q", id))
				return nil
			}

			database, err := db.Open(cfg.DBPath)
			if err != nil {
				return fmt.Errorf("open database: %w", err)
			}
			defer database.Close()

			s, err := database.GetSAM(id)
			if err != nil {
				return fmt.Errorf("db lookup: %w", err)
			}

			if s == nil {
				// Fall back to live
				if cfg.SAMAPIKey == "" {
					output.Error("notice %q not in local DB and no SAM API key configured", id)
					return &exitCodeErr{output.ExitNotFound}
				}
				client := api.NewSAMClient(cfg.SAMURL, cfg.SAMAPIKey)
				notice, err := client.GetNotice(id)
				if err != nil {
					return fmt.Errorf("SAM.gov API: %w", err)
				}
				if notice == nil {
					output.Error("notice %q not found", id)
					return &exitCodeErr{output.ExitNotFound}
				}
				s = &db.SAMOpportunity{
					NoticeID:           notice.NoticeID,
					Title:              notice.Title,
					Agency:             api.AgencyFromSAM(notice),
					SubTier:            notice.SubTier,
					NAICSCode:          notice.NAICSCode,
					SetAside:           notice.TypeOfSetAsideCode,
					ResponseDeadline:   notice.ResponseDeadLine,
					PostedDate:         notice.PostedDate,
					Description:        notice.Description,
					SolicitationNumber: notice.SolicitationNumber,
				}
			}

			if output.EffectiveFormat(opts) == output.FormatJSON {
				output.PrintJSON(output.FilterFields(samToMap(s), opts.Select))
				return nil
			}

			printSAMDetail(s)
			return nil
		},
	}
}

func printSAMDetail(s *db.SAMOpportunity) {
	fmt.Printf("Notice ID            : %s\n", s.NoticeID)
	fmt.Printf("Title                : %s\n", s.Title)
	fmt.Printf("Agency               : %s\n", s.Agency)
	fmt.Printf("Sub-Tier             : %s\n", s.SubTier)
	fmt.Printf("NAICS Code           : %s\n", s.NAICSCode)
	fmt.Printf("Set-Aside            : %s\n", s.SetAside)
	fmt.Printf("Solicitation Number  : %s\n", s.SolicitationNumber)
	fmt.Printf("Posted Date          : %s\n", s.PostedDate)
	fmt.Printf("Response Deadline    : %s\n", s.ResponseDeadline)
	fmt.Println()
	if s.Description != "" {
		fmt.Printf("Description:\n%s\n", s.Description)
	}
}

type exitCodeErr struct{ code int }

func (e *exitCodeErr) Error() string { return fmt.Sprintf("exit code %d", e.code) }
