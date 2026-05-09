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

func newSearchCmd(opts *output.Options) *cobra.Command {
	var limit int
	var live bool
	var setAside string

	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Search SAM.gov opportunities (local FTS5 or live API)",
		Args:  cobra.MinimumNArgs(1),
		Example: `  govprocure-pp-cli sam search "software development"
  govprocure-pp-cli sam search "IT consulting" --live --set-aside SDVOSB
  govprocure-pp-cli sam search "cybersecurity" --agent`,
		RunE: func(cmd *cobra.Command, args []string) error {
			query := args[0]
			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}

			if opts.DryRun {
				output.DryRunMsg(fmt.Sprintf("sam search %q (live=%v)", query, live))
				return nil
			}

			if !live {
				return searchLocalSAM(query, limit, cfg, opts)
			}
			return searchLiveSAM(query, setAside, limit, cfg, opts)
		},
	}

	cmd.Flags().IntVar(&limit, "limit", 25, "max results")
	cmd.Flags().BoolVar(&live, "live", false, "query SAM.gov API directly")
	cmd.Flags().StringVar(&setAside, "set-aside", "", "filter by set-aside code: sdvosb|wosb|8a|hubzone|sba")
	return cmd
}

func searchLocalSAM(query string, limit int, cfg *config.Config, opts *output.Options) error {
	database, err := db.Open(cfg.DBPath)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer database.Close()

	opps, err := database.SearchSAM(query, limit)
	if err != nil {
		output.Warn("local FTS search failed (%v), falling back to live", err)
		return searchLiveSAM(query, "", limit, cfg, opts)
	}

	if len(opps) == 0 {
		output.Warn("no local results for %q — run 'govprocure-pp-cli sam sync' or use --live", query)
		return nil
	}

	renderSAM(opps, opts)
	return nil
}

func searchLiveSAM(query, setAside string, limit int, cfg *config.Config, opts *output.Options) error {
	if cfg.SAMAPIKey == "" {
		return fmt.Errorf("SAM API key not configured — run: govprocure-pp-cli auth set-key --sam KEY")
	}

	// Normalize set-aside code
	var saCode string
	if setAside != "" {
		if code, ok := api.SetAsideCodes[setAside]; ok {
			saCode = code
		} else {
			saCode = setAside // pass through raw if not in map
		}
	}

	client := api.NewSAMClient(cfg.SAMURL, cfg.SAMAPIKey)
	resp, err := client.Search(query, saCode, limit, 0)
	if err != nil {
		return fmt.Errorf("SAM.gov search: %w", err)
	}

	var opps []*db.SAMOpportunity
	for _, o := range resp.OpportunitiesData {
		raw, _ := json.Marshal(o)
		opps = append(opps, &db.SAMOpportunity{
			NoticeID:           o.NoticeID,
			Title:              o.Title,
			Agency:             api.AgencyFromSAM(&o),
			SubTier:            o.SubTier,
			NAICSCode:          o.NAICSCode,
			SetAside:           o.TypeOfSetAsideCode,
			ResponseDeadline:   o.ResponseDeadLine,
			PostedDate:         o.PostedDate,
			Description:        o.Description,
			SolicitationNumber: o.SolicitationNumber,
			RawJSON:            string(raw),
		})
	}

	if len(opps) == 0 {
		fmt.Println("No results found.")
		return nil
	}

	renderSAM(opps, opts)
	return nil
}

func renderSAM(opps []*db.SAMOpportunity, opts *output.Options) {
	switch output.EffectiveFormat(opts) {
	case output.FormatJSON:
		var out []map[string]interface{}
		for _, s := range opps {
			m := samToMap(s)
			out = append(out, output.FilterFields(m, opts.Select))
		}
		output.PrintJSON(out)

	case output.FormatCSV:
		headers := []string{"notice_id", "title", "agency", "set_aside", "deadline", "naics"}
		var rows [][]string
		for _, s := range opps {
			rows = append(rows, []string{s.NoticeID, s.Title, s.Agency, s.SetAside, s.ResponseDeadline, s.NAICSCode})
		}
		output.PrintCSV(headers, rows)

	case output.FormatCompact:
		for _, s := range opps {
			fmt.Printf("%s | %s | %s | %s\n",
				s.NoticeID, output.Truncate(s.Title, 45), s.SetAside, s.ResponseDeadline)
		}

	default:
		headers := []string{"NOTICE ID", "TITLE", "AGENCY", "SET-ASIDE", "DEADLINE"}
		var rows [][]string
		for _, s := range opps {
			rows = append(rows, []string{
				s.NoticeID,
				output.Truncate(s.Title, 40),
				output.Truncate(s.Agency, 28),
				s.SetAside,
				s.ResponseDeadline,
			})
		}
		output.PrintTable(headers, rows)
		fmt.Printf("\n%d results\n", len(opps))
	}
}

func samToMap(s *db.SAMOpportunity) map[string]interface{} {
	return map[string]interface{}{
		"notice_id":            s.NoticeID,
		"title":                s.Title,
		"agency":               s.Agency,
		"sub_tier":             s.SubTier,
		"naics_code":           s.NAICSCode,
		"set_aside":            s.SetAside,
		"response_deadline":    s.ResponseDeadline,
		"posted_date":          s.PostedDate,
		"description":          s.Description,
		"solicitation_number":  s.SolicitationNumber,
	}
}
