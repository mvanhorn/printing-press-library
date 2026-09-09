// Copyright 2026 Paul Siola and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored: full job detail via the SEEK GraphQL jobDetails query.
// pp:data-source live

package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/productivity/seek/internal/cliutil"
)

type jobDetailView struct {
	ID               string          `json:"id"`
	Title            string          `json:"title"`
	Advertiser       string          `json:"advertiser"`
	AdvertiserID     string          `json:"advertiser_id,omitempty"`
	Location         string          `json:"location"`
	SalaryLabel      string          `json:"salary_label,omitempty"`
	Content          string          `json:"content"`
	ApplyURL         string          `json:"apply_url"`
	CompanyProfile   json.RawMessage `json:"company_profile,omitempty"`
	CompanySearchURL string          `json:"company_search_url,omitempty"`
}

func newNovelListingsGetCmd(flags *rootFlags) *cobra.Command {
	var flagSite string
	var flagText bool

	cmd := &cobra.Command{
		Use:   "get <job-id>",
		Short: "Fetch full details for one SEEK job (description, salary, apply link, company)",
		Long: "Use 'listings get' for the full job ad — description, salary text, apply URL, and\n" +
			"the company profile — via SEEK's own GraphQL endpoint (never the bot-gated HTML).\n" +
			"Pass --text to strip the description HTML to plain text.",
		Example: strings.Trim(`
  seek-pp-cli listings get 94483533
  seek-pp-cli listings get 94483533 --text --select title,salary_label,apply_url`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live", "pp:happy-args": "job-id=94483533"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "listings get")
			}
			if err := validateDataSourceStrategy(flags, "live"); err != nil {
				return err
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a SEEK job ID is required (from 'listings search')"))
			}
			id := strings.TrimSpace(args[0])
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			jd, err := fetchJobDetails(ctx, c, id, flagSite)
			if err != nil {
				return classifyAPIError(cmd.OutOrStdout(), err, flags)
			}
			if jd.ID == "" {
				return notFoundErr(fmt.Errorf("no SEEK job with ID %q", id))
			}
			content := jd.Content
			if flagText {
				content = cliutil.CleanText(stripHTMLTags(content))
			}
			view := jobDetailView{
				ID: jd.ID, Title: jd.Title, Advertiser: jd.Advertiser.Name,
				AdvertiserID: jd.Advertiser.ID, Location: jd.Location.Label,
				SalaryLabel: jd.SalaryLabel, Content: content,
				ApplyURL:         "https://au.seek.com/job/" + jd.ID,
				CompanySearchURL: jd.CompanySearch,
			}
			if len(jd.CompanyProfile) > 0 && string(jd.CompanyProfile) != "null" {
				view.CompanyProfile = jd.CompanyProfile
			}

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "%s — %s\n%s\n", view.Title, view.Advertiser, view.Location)
			if view.SalaryLabel != "" {
				fmt.Fprintf(w, "Salary: %s\n", view.SalaryLabel)
			}
			fmt.Fprintf(w, "Apply: %s\n\n", view.ApplyURL)
			fmt.Fprintln(w, cliutil.CleanText(stripHTMLTags(view.Content)))
			return nil
		},
	}
	cmd.Flags().StringVar(&flagSite, "site", "AU-Main", "Marketplace: AU-Main or NZ-Main")
	cmd.Flags().BoolVar(&flagText, "text", false, "Strip the description HTML to plain text in the output")
	return cmd
}

// stripHTMLTags is a minimal tag remover for SEEK job-ad HTML; cliutil.CleanText
// then handles entities and whitespace.
func stripHTMLTags(s string) string {
	var b strings.Builder
	inTag := false
	for _, r := range s {
		switch {
		case r == '<':
			inTag = true
		case r == '>':
			inTag = false
			b.WriteByte(' ')
		case !inTag:
			b.WriteRune(r)
		}
	}
	return b.String()
}
