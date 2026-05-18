// PATCH: novel cross-audience contact lookup — every audience/segment/topic a contact belongs to, in one query.
package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/productivity/resend/internal/store"
)

func newContactsWhereCmd(flags *rootFlags) *cobra.Command {
	var dbPath string

	cmd := &cobra.Command{
		Use:   "where <email-or-name>",
		Short: "Find every audience, segment, and topic a contact belongs to (cross-audience lookup)",
		Long: `Cross-audience contact lookup. Resend's API requires scanning every
audience (N requests) and merging client-side to answer "which audiences is
bob in". This command does it as one local query joined across the contacts,
contacts_segments, and contacts_topics tables.`,
		Example: strings.Trim(`
  # By email
  resend-pp-cli contacts where bob@example.invalid --json

  # By name (matches first_name or last_name)
  resend-pp-cli contacts where Bob --json --select email,audience_id,subscribed
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			needle := strings.TrimSpace(args[0])
			if needle == "" {
				return usageErr(fmt.Errorf("email-or-name argument is required"))
			}

			if dbPath == "" {
				dbPath = defaultDBPath("resend-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w\nRun 'resend-pp-cli sync' first.", err)
			}
			defer db.Close()

			// Escape LIKE metacharacters so a needle containing '%' or '_'
			// (e.g., alice_b or first_name) doesn't widen the substring match.
			escaped := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(needle)
			like := "%" + escaped + "%"
			// Join contacts with contacts_segments and contacts_topics so the
			// command actually delivers what its Long description promises:
			// every audience, segment, and topic a contact belongs to in one
			// query. GROUP_CONCAT collapses per-contact rows into one membership
			// row with comma-separated segment_ids and topic_ids.
			rows, err := db.Query(`
				SELECT
					c.id,
					COALESCE(c.email, '') AS email,
					COALESCE(c.first_name, '') AS first_name,
					COALESCE(c.last_name, '') AS last_name,
					COALESCE(c.unsubscribed, 0) AS unsubscribed,
					COALESCE(json_extract(c.data, '$.audience_id'), '') AS audience_id,
					COALESCE(c.created_at, '') AS created_at,
					COALESCE(GROUP_CONCAT(DISTINCT cs.parent_id), '') AS segment_ids,
					COALESCE(GROUP_CONCAT(DISTINCT ct.parent_id), '') AS topic_ids
				FROM contacts c
				LEFT JOIN contacts_segments cs ON cs.contacts_id = c.id
				LEFT JOIN contacts_topics   ct ON ct.contacts_id = c.id
				WHERE c.email = ?
					OR c.email LIKE ? ESCAPE '\'
					OR c.first_name LIKE ? ESCAPE '\'
					OR c.last_name LIKE ? ESCAPE '\'
				GROUP BY c.id
				ORDER BY c.email
			`, needle, like, like, like)
			if err != nil {
				return fmt.Errorf("querying contacts: %w", err)
			}
			defer rows.Close()

			type membership struct {
				ID         string   `json:"id"`
				Email      string   `json:"email"`
				FirstName  string   `json:"first_name"`
				LastName   string   `json:"last_name"`
				Subscribed bool     `json:"subscribed"`
				AudienceID string   `json:"audience_id"`
				SegmentIDs []string `json:"segment_ids"`
				TopicIDs   []string `json:"topic_ids"`
				CreatedAt  string   `json:"created_at"`
			}
			splitIDs := func(s string) []string {
				if s == "" {
					return []string{}
				}
				parts := strings.Split(s, ",")
				out := parts[:0]
				for _, p := range parts {
					if p = strings.TrimSpace(p); p != "" {
						out = append(out, p)
					}
				}
				return out
			}
			results := []membership{}
			for rows.Next() {
				var m membership
				var unsubInt int
				var segCSV, topCSV string
				if err := rows.Scan(&m.ID, &m.Email, &m.FirstName, &m.LastName, &unsubInt, &m.AudienceID, &m.CreatedAt, &segCSV, &topCSV); err != nil {
					continue
				}
				m.Subscribed = unsubInt == 0
				m.SegmentIDs = splitIDs(segCSV)
				m.TopicIDs = splitIDs(topCSV)
				results = append(results, m)
			}
			if err := rows.Err(); err != nil {
				return fmt.Errorf("iterating contacts: %w", err)
			}

			out := cmd.OutOrStdout()
			if flags.asJSON {
				return printJSONFiltered(out, map[string]any{
					"needle":      needle,
					"match_count": len(results),
					"memberships": results,
				}, flags)
			}
			if len(results) == 0 {
				fmt.Fprintf(out, "No contact memberships found matching %q.\n", needle)
				fmt.Fprintln(out, "(Run 'resend-pp-cli sync --full' to refresh.)")
				return nil
			}
			fmt.Fprintf(out, "%d membership row(s) matching %q:\n\n", len(results), needle)
			fmt.Fprintf(out, "%-32s %-22s %-12s %-9s %-7s %s\n", "EMAIL", "AUDIENCE_ID", "NAME", "SEGMENTS", "TOPICS", "SUBSCRIBED")
			fmt.Fprintf(out, "%-32s %-22s %-12s %-9s %-7s %s\n", "-----", "-----------", "----", "--------", "------", "----------")
			for _, r := range results {
				name := strings.TrimSpace(r.FirstName + " " + r.LastName)
				sub := "yes"
				if !r.Subscribed {
					sub = "no"
				}
				fmt.Fprintf(out, "%-32s %-22s %-12s %-9d %-7d %s\n", truncate(r.Email, 30), truncate(r.AudienceID, 20), truncate(name, 10), len(r.SegmentIDs), len(r.TopicIDs), sub)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/resend-pp-cli/data.db)")
	return cmd
}
