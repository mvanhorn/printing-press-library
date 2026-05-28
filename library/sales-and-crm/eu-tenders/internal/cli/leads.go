// Copyright 2026 Mathias Michel and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/eu-tenders/internal/store"

	"github.com/spf13/cobra"
)

// LeadRow represents a construction contract award winner as a B2B lead.
type LeadRow struct {
	WinnerName     string  `json:"winner_name"`
	WinnerCountry  string  `json:"winner_country"`
	Title          string  `json:"title"`
	CPVDescription string  `json:"cpv_description"`
	ContractValue  float64 `json:"contract_value"`
	Currency       string  `json:"currency"`
	Location       string  `json:"location"`
	PublishedDate  string  `json:"published_date"`
	CPVCode        string  `json:"cpv_code"`
	BuyerName      string  `json:"buyer_name"`
	BuyerCountry   string  `json:"buyer_country"`
	TEDURL         string  `json:"ted_url"`
}

func newLeadsCmd(flags *rootFlags) *cobra.Command {
	var (
		cpv      string
		country  string
		region   string
		keywords string
		minValue float64
		days     int
		dbPath   string
		limit    int
	)

	cmd := &cobra.Command{
		Use:   "leads",
		Short: "Find recent construction contract award winners for B2B outreach",
		Long: `Surface companies that recently won construction contracts — ideal for
outreach from suppliers of construction machinery, materials, or services.

The JSON output includes cpv_description as a human-readable project type when
the notice title is missing (the TED API omits titles for ~85% of award notices).

Use --cpv to filter by construction category. Use --keywords to further narrow
by project type in the notice title (OR-matched, case-insensitive).

NOTE: TED award notice titles describe what is built (Neubau, Rohbau, Brücke),
not what equipment is required. Keywords like "Kran" or "Container" will not
match. Use project-type keywords instead — they imply heavy machinery needs.

Keywords that imply crane use: Neubau, Rohbau, Hochbau, Stahlbeton, Brücke,
  Tunnel, Krankenhaus, Schulbau, Industriebau, Generalunternehmer

CPV codes by project type (useful for ICP targeting):
  45500000  Machinery hire with operator (direct buyers of construction equipment rental)
  45000000  All construction work (broadest net)
  45200000  Civil engineering work (roads, bridges, infrastructure)
  45310000  Electrical installation (covers PV/solar farm construction)
  45230000  Pipelines and power line construction (wind farm cables)
  45210000  Building construction (Hochbau, hospitals, schools)

For a weekly Slack digest of new machinery hire leads:
  eu-tenders-pp-cli leads --cpv 45 --country DEU --days 7 --json

For PV/solar construction company leads (companies that typically rent, not own, heavy machines):
  eu-tenders-pp-cli leads --cpv 45310000 --country DEU --days 30 --json

Examples:
  eu-tenders-pp-cli leads --country DEU --days 7 --json
  eu-tenders-pp-cli leads --cpv 45310000 --country DEU --days 30 --json
  eu-tenders-pp-cli leads --country DEU --keywords "Neubau,Rohbau,Hochbau" --days 90
  eu-tenders-pp-cli leads --country DEU --keywords "Brücke,Tunnel,Krankenhaus" --days 90
  eu-tenders-pp-cli leads --cpv 45500000 --country DEU --days 365`,
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}

			if flags.dataSource == "live" || flags.dataSource == "auto" {
				return leadsFromLive(cmd, flags, cpv, country, region, keywords, minValue, days, limit)
			}
			return leadsFromStore(cmd, flags, cpv, country, region, keywords, minValue, days, limit, dbPath)
		},
	}

	cmd.Flags().StringVar(&cpv, "cpv", "45", "CPV prefix for construction works (default: 45 = all construction)")
	cmd.Flags().StringVar(&country, "country", "", "Buyer country 3-letter ISO code (e.g. DEU)")
	cmd.Flags().StringVar(&region, "region", "", "NUTS region code filter (e.g. DE2)")
	cmd.Flags().StringVar(&keywords, "keywords", "", "Comma-separated keywords to match in notice title (e.g. \"Kran,Container\")")
	cmd.Flags().Float64Var(&minValue, "min-value", 0, "Minimum contract value in EUR (0 = include notices where value is not reported)")
	cmd.Flags().IntVar(&days, "days", 90, "Look back N days for recent awards")
	cmd.Flags().IntVar(&limit, "limit", 50, "Maximum results to return")
	cmd.Flags().StringVar(&dbPath, "db", defaultDBPath(), "SQLite database path")

	return cmd
}

func leadsFromLive(cmd *cobra.Command, flags *rootFlags, cpv, country, region, keywords string, minValue float64, days, limit int) error {
	c, err := flags.newClient()
	if err != nil {
		return err
	}

	cpvNorm := normalizeCPV(cpv)
	since := time.Now().AddDate(0, 0, -days).Format("20060102")

	var queryParts []string
	queryParts = append(queryParts, "notice-type=can-standard")
	queryParts = append(queryParts, fmt.Sprintf("classification-cpv IN (%s)", cpvNorm))
	queryParts = append(queryParts, fmt.Sprintf("PD>=%s", since))
	if country != "" {
		queryParts = append(queryParts, fmt.Sprintf("buyer-country=%s", strings.ToUpper(country)))
	}

	query := strings.Join(queryParts, " AND ")
	fields := []string{
		"publication-number", "notice-type", "publication-date",
		"winner-name", "winner-country", "result-value-notice", "result-value-cur-notice",
		"classification-cpv", "title-lot", "title-proc", "place-of-performance-post-code-part",
		"buyer-country", "buyer-name",
	}

	// When keyword filtering is active, fetch the API maximum (250) so the
	// post-filter has enough candidates. Without keywords, respect the limit directly.
	batchLimit := 250
	if keywords == "" && limit < 250 {
		batchLimit = limit
	}

	notices, err := postSearch(c, query, fields, batchLimit)
	if err != nil {
		return classifyAPIError(err, flags)
	}

	kwTerms := parseKeywords(keywords)
	// Warn when keyword post-filtering may have exhausted the API page before
	// reaching --limit matches; the user should narrow their date range instead.
	if len(kwTerms) > 0 && len(notices) == 250 {
		fmt.Fprintf(cmd.ErrOrStderr(), "note: fetched the API maximum of 250 notices before keyword filtering; results may be incomplete — use a shorter --days window or a more specific --cpv to reduce the candidate set\n")
	}
	now := time.Now().UTC().Format(time.RFC3339)
	var leads []LeadRow
	for _, raw := range notices {
		if limit > 0 && len(leads) >= limit {
			break
		}
		n := extractNotice(raw, now)
		if region != "" && !strings.HasPrefix(n.PlaceOfPerformance, region) {
			continue
		}
		if !matchesKeywords(n.Title, kwTerms) {
			continue
		}
		val := n.ContractValue
		if val == 0 {
			val = n.EstimatedValue
		}
		if minValue > 0 && val > 0 && val < minValue {
			continue
		}
		leads = append(leads, LeadRow{
			WinnerName:     n.WinnerName,
			WinnerCountry:  n.WinnerCountry,
			Title:          n.Title,
			CPVDescription: cpvDescription(n.CPVCode),
			ContractValue:  val,
			Currency:       n.Currency,
			Location:       n.PlaceOfPerformance,
			PublishedDate:  n.PublicationDate,
			CPVCode:        n.CPVCode,
			BuyerName:      n.BuyerName,
			BuyerCountry:   n.BuyerCountry,
			TEDURL:         n.NoticeURL,
		})
	}

	return printLeads(cmd, flags, leads)
}

func leadsFromStore(cmd *cobra.Command, flags *rootFlags, cpv, country, region, keywords string, minValue float64, days, limit int, dbPath string) error {
	st, err := store.Open(dbPath)
	if err != nil {
		return fmt.Errorf("open store: %w", err)
	}
	defer st.Close()

	count, _ := st.Count()
	if count == 0 {
		fmt.Fprintf(cmd.OutOrStdout(), "No notices synced yet. Run: eu-tenders-pp-cli sync --country %s --cpv %s --since %s\n",
			orDefault(country, "DEU"), normalizeCPV(cpv), time.Now().AddDate(-1, 0, 0).Format("2006-01-02"))
		return nil
	}

	since := time.Now().AddDate(0, 0, -days).Format("2006-01-02")
	q := `SELECT id, publication_date, buyer_name, buyer_country, cpv_code,
		winner_name, winner_country, contract_value, estimated_value, currency, title,
		place_of_performance, notice_url
		FROM notices
		WHERE notice_type='can-standard'
		  AND publication_date >= ?
		  AND cpv_code LIKE ?`
	params := []interface{}{since, normalizeCPVLike(cpv)}

	if country != "" {
		q += " AND buyer_country=?"
		params = append(params, strings.ToUpper(country))
	}
	// Region filter in SQL so LIMIT applies after filtering — otherwise
	// `--limit 50 --region DE2` could return 0–50 rows by silently
	// dropping the SQL window's non-DE2 results in Go.
	if region != "" {
		q += " AND place_of_performance LIKE ?"
		params = append(params, region+"%")
	}
	if minValue > 0 {
		// Include notices that meet the threshold OR have no reported value (TED often omits contract values).
		q += " AND (contract_value = 0 AND estimated_value = 0 OR contract_value >= ? OR estimated_value >= ?)"
		params = append(params, minValue, minValue)
	}
	// Keyword filter in SQL so LIMIT applies after filtering, not before.
	kwTerms := parseKeywords(keywords)
	if len(kwTerms) > 0 {
		q += " AND ("
		for i, t := range kwTerms {
			if i > 0 {
				q += " OR "
			}
			q += "LOWER(title) LIKE ?"
			params = append(params, "%"+t+"%")
		}
		q += ")"
	}
	q += " ORDER BY publication_date DESC LIMIT ?"
	params = append(params, limit)

	rows, err := st.DB().Query(q, params...)
	if err != nil {
		return fmt.Errorf("query: %w", err)
	}
	defer rows.Close()
	var leads []LeadRow
	for rows.Next() {
		var id, pubDate, buyerName, buyerCountry, cpvCode string
		var winnerName, winnerCountry, currency, title, location, url string
		var contractValue, estimatedValue float64
		if err := rows.Scan(&id, &pubDate, &buyerName, &buyerCountry, &cpvCode,
			&winnerName, &winnerCountry, &contractValue, &estimatedValue, &currency,
			&title, &location, &url); err != nil {
			continue
		}
		val := contractValue
		if val == 0 {
			val = estimatedValue
		}
		leads = append(leads, LeadRow{
			WinnerName:     winnerName,
			WinnerCountry:  winnerCountry,
			Title:          title,
			CPVDescription: cpvDescription(cpvCode),
			ContractValue:  val,
			Currency:       currency,
			Location:       location,
			PublishedDate:  pubDate,
			CPVCode:        cpvCode,
			BuyerName:      buyerName,
			BuyerCountry:   buyerCountry,
			TEDURL:         url,
		})
	}

	return printLeads(cmd, flags, leads)
}

func printLeads(cmd *cobra.Command, flags *rootFlags, leads []LeadRow) error {
	if flags.asJSON {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(leads)
	}

	if len(leads) == 0 {
		fmt.Fprintf(cmd.OutOrStdout(), "No leads found for the given filters\n")
		return nil
	}

	tw := newTabWriter(cmd.OutOrStdout())
	fmt.Fprintln(tw, "WINNER\tCOUNTRY\tPROJECT\tVALUE\tLOCATION\tPUBLISHED\tTED LINK")
	for _, l := range leads {
		val := ""
		if l.ContractValue > 0 {
			val = fmt.Sprintf("€%.0f", l.ContractValue)
		}
		project := l.Title
		if project == "" {
			project = l.CPVDescription
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			truncate(l.WinnerName, 30),
			l.WinnerCountry,
			truncate(project, 40),
			val,
			l.Location,
			l.PublishedDate,
			l.TEDURL,
		)
	}
	return tw.Flush()
}

// matchesKeywords returns true when text contains at least one of the terms
// (case-insensitive OR match). Always returns true when terms is nil.
func matchesKeywords(text string, terms []string) bool {
	if len(terms) == 0 {
		return true
	}
	lower := strings.ToLower(text)
	for _, t := range terms {
		if strings.Contains(lower, t) {
			return true
		}
	}
	return false
}
