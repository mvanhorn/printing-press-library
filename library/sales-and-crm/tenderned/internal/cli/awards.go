package cli

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func newAwardsCmd(flags *rootFlags) *cobra.Command {
	var since, until, cpv, buyer, kind, nuts string
	var limit, size int
	cmd := &cobra.Command{
		Use:   "awards",
		Short: "List contract-award notices (Aankondiging Gegunde Opdracht). Sugar over notices --type AGO; mirrors eu-tenders awards.",
		Long: `List Dutch contract-award notices (AGO — Aankondiging Gegunde Opdracht).
Convenience wrapper over the notices list endpoint with publicatieType=AGO already set.
Mirrors the eu-tenders awards command so the two CLIs can be used in conjunction.`,
		Example: "  tenderned-pp-cli awards --since 2026-04-01 --cpv 45000000-7 --limit 50",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			params := url.Values{}
			params.Set("publicatieType", "AGO")
			if since != "" {
				params.Set("publicatieDatumVanaf", since)
			}
			if until != "" {
				params.Set("publicatieDatumTot", until)
			}
			if cpv != "" {
				params.Set("cpvCodes", cpv)
			}
			if buyer != "" {
				params.Set("aanbestedendeDienstId", buyer)
			}
			if kind != "" {
				params.Set("typeOpdracht", kind)
			}
			if nuts != "" {
				params.Set("nutsCode", nuts)
			}
			pageSize := size
			if pageSize <= 0 {
				pageSize = 50
			}
			params.Set("size", fmt.Sprintf("%d", pageSize))
			params.Set("page", "0")

			fullURL := tnBaseURL + "/publicaties?" + params.Encode()
			// PATCH: check error from http.NewRequestWithContext; discarding it left req=nil → panic on Header.Set
			req, err := http.NewRequestWithContext(cmd.Context(), http.MethodGet, fullURL, nil)
			if err != nil {
				return fmt.Errorf("building request: %w", err)
			}
			req.Header.Set("Accept", "application/json")
			hc := &http.Client{Timeout: 30 * time.Second}
			resp, err := hc.Do(req)
			if err != nil {
				return err
			}
			defer resp.Body.Close()
			if resp.StatusCode/100 != 2 {
				return fmt.Errorf("HTTP %d from TenderNed", resp.StatusCode)
			}
			var body struct {
				Content       []json.RawMessage `json:"content"`
				TotalElements int               `json:"totalElements"`
			}
			if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
				return err
			}
			items := body.Content
			if limit > 0 && len(items) > limit {
				items = items[:limit]
			}
			result := map[string]any{
				"awards":        items,
				"returned":      len(items),
				"totalElements": body.TotalElements,
			}
			if flags.asJSON || flags.quiet {
				out, _ := json.MarshalIndent(result, "", "  ")
				fmt.Fprintln(cmd.OutOrStdout(), string(out))
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Returned %d / %d awards\n", len(items), body.TotalElements)
			for _, it := range items {
				var view struct {
					ID    string `json:"publicatieId"`
					Title string `json:"aanbestedingNaam"`
					Buyer string `json:"opdrachtgeverNaam"`
					Date  string `json:"publicatieDatum"`
				}
				_ = json.Unmarshal(it, &view)
				fmt.Fprintf(cmd.OutOrStdout(), "  %s | %s | %s | %s\n", view.ID, view.Date[:strings.Index(view.Date+"T", "T")], view.Buyer, view.Title)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&since, "since", "", "Earliest publication date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&until, "until", "", "Latest publication date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&cpv, "cpv", "", "CPV code in 8-digit-plus-check-digit form (e.g. 45000000-7)")
	cmd.Flags().StringVar(&buyer, "buyer-id", "", "Contracting-authority UUID")
	cmd.Flags().StringVar(&kind, "kind", "", "Contract type: D=services, L=supplies, W=works")
	cmd.Flags().StringVar(&nuts, "nuts", "", "NUTS region code (e.g. NL33)")
	cmd.Flags().IntVar(&limit, "limit", 0, "Cap the number of results returned (0 = no cap)")
	cmd.Flags().IntVar(&size, "size", 50, "Page size for the underlying API call (max 100)")
	return cmd
}
