package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/ai/zylos/internal/store"
)

func newSearchCmd(flags *rootFlags) *cobra.Command {
	var contextN int
	var limit int
	var dbPath string

	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Full-text search conversation history with optional surrounding context",
		Example: strings.Trim(`
  zylos-pp-cli search "hello world"
  zylos-pp-cli search "error" --context 3 --limit 10
  zylos-pp-cli search "deploy" --json`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		Args:        cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}

			if dbPath == "" {
				dbPath = defaultDBPath("zylos-pp-cli")
			}
			db, err := store.OpenReadOnly(dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w\nRun 'zylos-pp-cli sync' first.", err)
			}
			defer db.Close()

			if limit <= 0 {
				limit = 50
			}

			rows, err := db.DB().QueryContext(cmd.Context(),
				`SELECT r.id, r.data FROM resources r
				 JOIN resources_fts f ON r.id = f.id
				 WHERE resources_fts MATCH ?
				 ORDER BY rank
				 LIMIT ?`,
				args[0], limit,
			)
			if err != nil {
				return fmt.Errorf("searching: %w", err)
			}
			defer rows.Close()

			type message struct {
				ID        int    `json:"id"`
				Direction string `json:"direction"`
				Content   string `json:"content"`
				Timestamp string `json:"timestamp"`
			}

			type searchResult struct {
				Match   message   `json:"match"`
				Context []message `json:"context,omitempty"`
			}

			var results []searchResult
			for rows.Next() {
				var id string
				var dataStr string
				if err := rows.Scan(&id, &dataStr); err != nil {
					continue
				}
				var msg message
				if err := json.Unmarshal([]byte(dataStr), &msg); err != nil {
					continue
				}

				sr := searchResult{Match: msg}

				if contextN > 0 {
					ctxRows, err := db.DB().QueryContext(cmd.Context(),
						`SELECT data FROM resources
						 WHERE resource_type = 'conversations'
						 AND id IN (
						   SELECT id FROM resources WHERE resource_type = 'conversations'
						   ORDER BY json_extract(data, '$.timestamp') ASC
						 )
						 AND json_extract(data, '$.timestamp') > (
						   SELECT json_extract(data, '$.timestamp') FROM resources WHERE id = ?
						 )
						 ORDER BY json_extract(data, '$.timestamp') ASC
						 LIMIT ?`,
						id, contextN,
					)
					if err == nil {
						var afterMsgs []message
						for ctxRows.Next() {
							var ctxData string
							if ctxRows.Scan(&ctxData) == nil {
								var m message
								if json.Unmarshal([]byte(ctxData), &m) == nil {
									afterMsgs = append(afterMsgs, m)
								}
							}
						}
						ctxRows.Close()

						beforeRows, err := db.DB().QueryContext(cmd.Context(),
							`SELECT data FROM resources
							 WHERE resource_type = 'conversations'
							 AND json_extract(data, '$.timestamp') < (
							   SELECT json_extract(data, '$.timestamp') FROM resources WHERE id = ?
							 )
							 ORDER BY json_extract(data, '$.timestamp') DESC
							 LIMIT ?`,
							id, contextN,
						)
						if err == nil {
							var beforeMsgs []message
							for beforeRows.Next() {
								var ctxData string
								if beforeRows.Scan(&ctxData) == nil {
									var m message
									if json.Unmarshal([]byte(ctxData), &m) == nil {
										beforeMsgs = append(beforeMsgs, m)
									}
								}
							}
							beforeRows.Close()

							// Reverse beforeMsgs so they are chronological
							for i, j := 0, len(beforeMsgs)-1; i < j; i, j = i+1, j-1 {
								beforeMsgs[i], beforeMsgs[j] = beforeMsgs[j], beforeMsgs[i]
							}
							sr.Context = append(beforeMsgs, msg)
							sr.Context = append(sr.Context, afterMsgs...)
						}
					}
				}

				results = append(results, sr)
			}

			return printJSONFiltered(cmd.OutOrStdout(), results, flags)
		},
	}

	cmd.Flags().IntVar(&contextN, "context", 0, "Number of surrounding messages to include")
	cmd.Flags().IntVar(&limit, "limit", 50, "Maximum number of search results")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")

	return cmd
}
