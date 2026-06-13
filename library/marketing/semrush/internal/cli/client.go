// client onboard — clone the KRAM template into a new client-named sheet and
// populate the data tabs the CLI can produce: "Keyword Research" (research
// workflow with PKD) and "SEMRush export" (raw domain organic keywords).
//
// Other tabs (Keyword Targeting, Keyword Gaps, GA export RAW, GSC export RAW)
// are populated by the /seo-kram skill at the orchestration layer — the CLI
// stays focused on raw data fetching.
package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"google.golang.org/api/drive/v3"
	"google.golang.org/api/sheets/v4"
)

// Default KRAM template ID. Override with --template-id or by setting
// SEMRUSH_KRAM_TEMPLATE_ID in the environment for a different default.
const defaultKRAMTemplateID = "1iUQjwqWyQy3tdxu5seV7HUde-2kCqFXe_cRgshsXnBc"

func newClientCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "client",
		Short: "Client-onboarding workflows (clone KRAM template, populate data tabs)",
	}
	cmd.AddCommand(newClientOnboardCmd(flags))
	return cmd
}

func newClientOnboardCmd(flags *rootFlags) *cobra.Command {
	var (
		templateID       string
		nameTemplate     string
		researchTab      string
		exportTab        string
		seedsFlag        string
		seedsFile        string
		database         string
		currency         string
		modeFlag         string
		limit            int
		minVolume        int
		maxKD            int
		excludeBrand     bool
		excludeList      string
		sleepMS          int
		organicLimit     int
		skipResearch     bool
		skipExport       bool
		researchColumns  string
		exportColumns    string
	)
	cmd := &cobra.Command{
		Use:   "onboard <client-domain>",
		Short: "Clone the KRAM template for a client domain and populate the data tabs",
		Long: "Creates a new Google Sheet by copying the KRAM template, names it after the client, " +
			"and populates two tabs:\n\n" +
			"  • Keyword Research — magic-recipe pipeline (seeds × modes × PKD-filtered)\n" +
			"  • SEMRush export   — raw domain_organic dump for the client domain\n\n" +
			"Other tabs (Keyword Targeting, Keyword Gaps, GA export RAW, GSC export RAW) " +
			"are left for the /seo-kram skill or manual curation.\n\n" +
			"Requires drive scope — re-run 'auth google' if you get a permissions error.",
		Example: strings.Trim(`
  semrush-pp-cli client onboard nationaltiles.com.au \
    --seeds "tiles,bathroom tiles,kitchen splashback" \
    --database au --currency AUD \
    --exclude "beaumont,bunnings,connetix"

  # Skip the research workflow (just clone + export tab)
  semrush-pp-cli client onboard nationaltiles.com.au --skip-research

  # Custom template
  semrush-pp-cli client onboard nationaltiles.com.au --template-id <other-sheet-id>
`, "\n"),
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientDomain := strings.TrimSpace(args[0])
			if clientDomain == "" {
				return fmt.Errorf("client-domain argument is required")
			}
			ctx := cmd.Context()

			// Step 1: Auth + clone the template
			driveSvc, err := loadGoogleDriveService(ctx)
			if err != nil {
				return err
			}
			sheetsSvc, err := loadGoogleSheetsService(ctx)
			if err != nil {
				return err
			}

			newName := strings.ReplaceAll(nameTemplate, "{client}", clientDomain)
			newName = strings.ReplaceAll(newName, "{date}", time.Now().Format("2006-01-02"))

			fmt.Fprintf(cmd.ErrOrStderr(), "client onboard: cloning template %q → %q …\n", templateID, newName)
			copied, err := driveSvc.Files.Copy(templateID, &drive.File{Name: newName}).Fields("id, name, webViewLink").Do()
			if err != nil {
				return fmt.Errorf("cloning template (does your OAuth have drive scope? re-run 'auth google'): %w", err)
			}
			newSheetID := copied.Id
			fmt.Fprintf(cmd.ErrOrStderr(), "client onboard: cloned → %s\n", copied.WebViewLink)

			// Step 2: Populate Keyword Research tab via the research pipeline.
			if !skipResearch {
				if seedsFlag == "" && seedsFile == "" {
					fmt.Fprintln(cmd.ErrOrStderr(), "client onboard: no --seeds or --seeds-file provided — skipping Keyword Research tab population")
				} else {
					seeds, err := loadSeeds(seedsFlag, seedsFile)
					if err != nil {
						return err
					}
					p := ResearchPipelineParams{
						Seeds:        seeds,
						Modes:        parseModes(modeFlag),
						Domain:       clientDomain,
						Database:     database,
						Currency:     currency,
						Limit:        limit,
						MinVolume:    minVolume,
						MaxKD:        maxKD,
						ExcludeBrand: excludeBrand,
						ExcludeList:  excludeList,
						Dedupe:       true,
						SleepMS:      sleepMS,
						SortField:    "volume",
						LogTo:        cmd.ErrOrStderr(),
					}
					rows, err := RunResearchPipeline(ctx, p)
					if err != nil {
						fmt.Fprintf(cmd.ErrOrStderr(), "client onboard: research pipeline error: %v (continuing)\n", err)
					} else if len(rows) > 0 {
						written, err := replaceRowsMatchingTemplate(sheetsSvc, newSheetID, researchTab, rows)
						if err != nil {
							fmt.Fprintf(cmd.ErrOrStderr(), "client onboard: failed to write research to %q: %v\n", researchTab, err)
						} else {
							fmt.Fprintf(cmd.ErrOrStderr(), "client onboard: wrote %d research rows → %q tab (matched to template columns; PKD/Topic/Category/etc. left blank)\n", written, researchTab)
						}
					} else {
						fmt.Fprintln(cmd.ErrOrStderr(), "client onboard: research returned 0 rows after filters — Keyword Research tab left empty")
					}
				}
			}

			// Step 3: Populate SEMRush export tab via public API domain_organic.
			if !skipExport {
				exportRows, err := fetchDomainOrganic(ctx, flags, clientDomain, database, organicLimit)
				if err != nil {
					hint := ""
					if strings.Contains(err.Error(), "WRONG KEY - ID PAIR") || strings.Contains(err.Error(), "403") {
						hint = "\n  → SEMrush's domain_organic report typically requires Business-tier API access. " +
							"Your keyword + KMT reports work fine; only this specific report is gated. " +
							"Re-run with --skip-export to suppress this message."
					}
					fmt.Fprintf(cmd.ErrOrStderr(), "client onboard: SEMRush export skipped: %v%s\n", err, hint)
				} else if len(exportRows) > 0 {
					written, err := replaceRowsMatchingTemplate(sheetsSvc, newSheetID, exportTab, exportRows)
					if err != nil {
						fmt.Fprintf(cmd.ErrOrStderr(), "client onboard: failed to write export to %q: %v\n", exportTab, err)
					} else {
						fmt.Fprintf(cmd.ErrOrStderr(), "client onboard: wrote %d organic-keyword rows → %q tab (matched to template columns)\n", written, exportTab)
					}
				} else {
					fmt.Fprintln(cmd.ErrOrStderr(), "client onboard: domain_organic returned 0 rows — SEMRush export tab left empty")
				}
			}

			fmt.Fprintln(cmd.OutOrStdout(), copied.WebViewLink)
			return nil
		},
	}
	cmd.Flags().StringVar(&templateID, "template-id", defaultKRAMTemplateID, "Source Google Sheet ID to clone")
	cmd.Flags().StringVar(&nameTemplate, "name", "KRAM - {client} - {date}", "Name for the cloned sheet ({client} and {date} are substituted)")
	cmd.Flags().StringVar(&researchTab, "research-tab", "Keyword Research", "Tab name for research workflow output")
	cmd.Flags().StringVar(&exportTab, "export-tab", "Client Keywords (semrush)", "Tab name for the client's organic-keywords export (renamed from 'SEMRush export' in the KRAM template)")
	cmd.Flags().StringVar(&seedsFlag, "seeds", "", "Comma-separated seed keywords (or use --seeds-file)")
	cmd.Flags().StringVar(&seedsFile, "seeds-file", "", "CSV/newline-separated seed file")
	cmd.Flags().StringVar(&database, "database", "us", "Country database (us, au, uk, ca, etc.)")
	cmd.Flags().StringVar(&currency, "currency", "USD", "Currency code")
	cmd.Flags().StringVar(&modeFlag, "mode", "broad,related,questions", "KMT modes (comma-separated)")
	cmd.Flags().IntVar(&limit, "limit", 100, "Max keywords per seed × mode")
	cmd.Flags().IntVar(&minVolume, "min-volume", 0, "Filter: minimum search volume")
	cmd.Flags().IntVar(&maxKD, "max-kd", 0, "Filter: maximum KD%")
	cmd.Flags().BoolVar(&excludeBrand, "exclude-self-brand", true, "Auto-derive and exclude the client's own brand terms")
	cmd.Flags().StringVar(&excludeList, "exclude", "", "Additional competitor brand terms to exclude")
	cmd.Flags().IntVar(&sleepMS, "sleep-ms", 250, "Pause between API calls")
	cmd.Flags().IntVar(&organicLimit, "organic-limit", 1000, "Max rows for SEMRush export tab (domain_organic display_limit)")
	cmd.Flags().BoolVar(&skipResearch, "skip-research", false, "Skip populating the Keyword Research tab")
	cmd.Flags().BoolVar(&skipExport, "skip-export", false, "Skip populating the SEMRush export tab")
	cmd.Flags().StringVar(&researchColumns, "research-columns", "phrase,volume,difficulty,domain_position,domain_traffic,cpc,_type", "Column order for Keyword Research tab (matches KRAM template: Keyword, Volume, KD, Position, Est Traffic, CPC, Type)")
	cmd.Flags().StringVar(&exportColumns, "export-columns", "", "Column order for SEMRush export tab (empty = auto-derive from SEMrush API response)")
	return cmd
}

// fetchDomainOrganic calls the SEMrush public-API domain_organic report and
// returns the parsed CSV rows. Uses the same client + auth as other API
// commands (SEMRUSH_API_KEY in query).
func fetchDomainOrganic(ctx context.Context, flags *rootFlags, domain, database string, limit int) ([]map[string]any, error) {
	c, err := flags.newClient()
	if err != nil {
		return nil, err
	}
	params := map[string]string{
		"type":          "domain_organic",
		"domain":        domain,
		"database":      database,
		"display_limit": fmt.Sprintf("%d", limit),
	}
	data, err := c.Get("/", params)
	if err != nil {
		return nil, err
	}
	var rows []map[string]any
	if err := json.Unmarshal(data, &rows); err != nil {
		// CSV converter may have left a non-array; try one more shape
		return nil, fmt.Errorf("decoding domain_organic response: %w", err)
	}
	return rows, nil
}

// splitColumns parses a comma-separated column list with trim.
func splitColumns(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if v := strings.TrimSpace(p); v != "" {
			out = append(out, v)
		}
	}
	return out
}

// _ keeps the sheets import used.
var _ = sheets.NewService
