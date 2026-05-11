package cli

import (
	"fmt"
	"math"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/cre-owner/internal/store"
	"github.com/spf13/cobra"
)

func newOutreachCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var flagMarket string
	var flagPropertyType string
	var flagMinBuildings int
	var flagMinSqft int
	var flagLimit int

	cmd := &cobra.Command{
		Use:   "outreach",
		Short: "Ranked cold-outreach list with contacts and mailing addresses",
		Long: `Build a ranked list of property owners for cold outreach in a target market.

Owners are ranked by number of buildings owned, total assessed value, and contact
confidence score. Output includes the best available contact (phone/email) and
mailing address (registered agent or assessor address).

Data comes from the local SQLite store. Run 'sync' first to populate.`,
		Example: strings.Trim(`
  cre-owner-pp-cli outreach --market "Dallas"
  cre-owner-pp-cli outreach --market "Austin" --type office --min-buildings 3
  cre-owner-pp-cli outreach --market "Houston" --min-sqft 50000 --limit 25 --json`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if !cmd.Flags().Changed("market") && !flags.dryRun {
				return fmt.Errorf("required flag \"market\" not set")
			}
			if dryRunOK(flags) {
				return nil
			}

			if dbPath == "" {
				dbPath = defaultDBPath("cre-owner-pp-cli")
			}
			s, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening store: %w", err)
			}
			defer s.Close()

			db := s.DB()

			// Build the query: join parcels with contacts and entities,
			// aggregate by owner, rank by building count + value + contact confidence.
			query := `
				SELECT
					p_owner,
					COUNT(DISTINCT p_id) AS building_count,
					COALESCE(SUM(p_value), 0) AS total_value,
					COALESCE(SUM(p_sqft), 0) AS total_sqft,
					MAX(c_name) AS contact_name,
					MAX(c_phone) AS contact_phone,
					MAX(c_email) AS contact_email,
					MAX(c_confidence) AS best_confidence,
					COALESCE(MAX(e_agent_addr), MAX(c_mailing)) AS mailing_address
				FROM (
					SELECT
						json_extract(r.data, '$.owner_name') AS p_owner,
						json_extract(r.data, '$.parcel_id') AS p_id,
						CAST(json_extract(r.data, '$.assessed_value') AS REAL) AS p_value,
						CAST(json_extract(r.data, '$.sqft') AS REAL) AS p_sqft,
						json_extract(r.data, '$.market') AS p_market,
						json_extract(r.data, '$.property_type') AS p_type
					FROM resources r
					WHERE r.resource_type = 'parcels'
				) parcels
				LEFT JOIN (
					SELECT
						json_extract(r.data, '$.owner_id') AS c_owner_id,
						json_extract(r.data, '$.name') AS c_name,
						json_extract(r.data, '$.phone') AS c_phone,
						json_extract(r.data, '$.email') AS c_email,
						json_extract(r.data, '$.mailing_address') AS c_mailing,
						CAST(json_extract(r.data, '$.confidence_score') AS REAL) AS c_confidence
					FROM resources r
					WHERE r.resource_type = 'contacts'
				) contacts ON contacts.c_owner_id = parcels.p_owner
				LEFT JOIN (
					SELECT
						json_extract(r.data, '$.name') AS e_name,
						json_extract(r.data, '$.registered_agent_address') AS e_agent_addr
					FROM resources r
					WHERE r.resource_type = 'entities'
				) entities ON entities.e_name = parcels.p_owner
				WHERE parcels.p_owner IS NOT NULL
				  AND LOWER(parcels.p_market) = LOWER(?)`

			queryArgs := []any{flagMarket}

			if flagPropertyType != "" {
				query += ` AND LOWER(parcels.p_type) = LOWER(?)`
				queryArgs = append(queryArgs, flagPropertyType)
			}

			query += ` GROUP BY p_owner`

			if flagMinBuildings > 1 {
				query += fmt.Sprintf(` HAVING building_count >= %d`, flagMinBuildings)
			}
			if flagMinSqft > 0 {
				if flagMinBuildings > 1 {
					query += fmt.Sprintf(` AND total_sqft >= %d`, flagMinSqft)
				} else {
					query += fmt.Sprintf(` HAVING total_sqft >= %d`, flagMinSqft)
				}
			}

			query += ` ORDER BY building_count DESC, total_value DESC, best_confidence DESC`
			query += fmt.Sprintf(` LIMIT %d`, flagLimit)

			rows, err := db.Query(query, queryArgs...)
			if err != nil {
				return fmt.Errorf("querying outreach data: %w", err)
			}
			defer rows.Close()

			type outreachRow struct {
				OwnerName      string  `json:"owner_name"`
				BuildingCount  int     `json:"building_count"`
				TotalValue     float64 `json:"total_assessed_value"`
				TotalSqft      float64 `json:"total_sqft"`
				ContactName    *string `json:"contact_name,omitempty"`
				ContactPhone   *string `json:"contact_phone,omitempty"`
				ContactEmail   *string `json:"contact_email,omitempty"`
				Confidence     float64 `json:"contact_confidence"`
				MailingAddress *string `json:"mailing_address,omitempty"`
				Rank           int     `json:"rank"`
			}

			var results []outreachRow
			rank := 0
			for rows.Next() {
				rank++
				var r outreachRow
				var contactName, contactPhone, contactEmail, mailingAddr *string
				if err := rows.Scan(
					&r.OwnerName,
					&r.BuildingCount,
					&r.TotalValue,
					&r.TotalSqft,
					&contactName,
					&contactPhone,
					&contactEmail,
					&r.Confidence,
					&mailingAddr,
				); err != nil {
					return fmt.Errorf("scanning row: %w", err)
				}
				r.ContactName = contactName
				r.ContactPhone = contactPhone
				r.ContactEmail = contactEmail
				r.MailingAddress = mailingAddr
				r.Rank = rank
				r.TotalValue = math.Round(r.TotalValue*100) / 100
				results = append(results, r)
			}
			if err := rows.Err(); err != nil {
				return fmt.Errorf("reading rows: %w", err)
			}

			if len(results) == 0 {
				if flags.asJSON {
					return printJSONFiltered(cmd.OutOrStdout(), []any{}, flags)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "No owners found in market %q matching criteria.\n", flagMarket)
				return nil
			}

			return printJSONFiltered(cmd.OutOrStdout(), results, flags)
		},
	}

	cmd.Flags().StringVar(&flagMarket, "market", "", "Target market/county (required)")
	cmd.Flags().StringVar(&flagPropertyType, "type", "", "Filter by property type")
	cmd.Flags().IntVar(&flagMinBuildings, "min-buildings", 1, "Minimum number of buildings owned")
	cmd.Flags().IntVar(&flagMinSqft, "min-sqft", 0, "Minimum total square footage")
	cmd.Flags().IntVar(&flagLimit, "limit", 50, "Maximum number of results")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/cre-owner-pp-cli/data.db)")

	return cmd
}
