// Copyright 2026 drummerms and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command — implemented from the absorb manifest transcendence row
// "Offline full-text search".
// pp:data-source auto

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/devices/logitech-docs/internal/store"
	"github.com/spf13/cobra"
)

func newNovelFindCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var limit int
	var live bool

	cmd := &cobra.Command{
		Use:   "find <query>",
		Short: "Full-text search inside synced manuals and spec sheets from the local store.",
		Long: "Offline full-text search over locally synced articles (title and body where available). " +
			"When the local store has no matches, it automatically falls back to a live search so the command is never a dead end. " +
			"Run 'logitech-docs-pp-cli sync --resources articles' to build the local index; pass --live to skip the local store entirely.",
		Example: "  logitech-docs-pp-cli find \"camera dimensions\"\n" +
			"  logitech-docs-pp-cli find \"pairing\" --limit 10 --json\n" +
			"  logitech-docs-pp-cli find \"pairing\" --live",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "auto", "pp:happy-args": "query=pairing"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "find search")
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("find requires a search query"))
			}
			query := strings.Join(args, " ")

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			rows := make([]map[string]any, 0)
			source := "local"

			// Local search first (unless --live).
			if !live {
				localPath := dbPath
				if localPath == "" {
					localPath = defaultDBPath("logitech-docs-pp-cli")
				}
				if _, statErr := os.Stat(localPath); !os.IsNotExist(statErr) {
					db, err := store.OpenReadOnly(localPath)
					if err != nil {
						return fmt.Errorf("opening database: %w", err)
					}
					hintIfUnsynced(cmd, db, "articles")
					hintIfStale(cmd, db, "articles", flags.maxAge)
					raws, searchErr := db.SearchArticles(query, limit)
					_ = db.Close()
					if searchErr != nil {
						return fmt.Errorf("searching local store: %w", searchErr)
					}
					for _, raw := range raws {
						var m map[string]any
						if json.Unmarshal(raw, &m) == nil {
							rows = append(rows, m)
						}
					}
				} else if dbPath == "" {
					fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s; run 'logitech-docs-pp-cli sync --resources articles' to build one\n", localPath)
				}
			}

			// Fall back to live when local produced nothing.
			if len(rows) == 0 {
				source = "live"
				if !live {
					fmt.Fprintln(cmd.ErrOrStderr(), "no local matches; searching support.logi.com live")
				}
				perPage := limit
				if perPage <= 0 {
					perPage = 50
				}
				if perPage > 100 {
					perPage = 100
				}
				c, err := flags.newClient()
				if err != nil {
					return err
				}
				data, err := c.Get(ctx, "/api/v2/help_center/articles/search.json", map[string]string{
					"query":    query,
					"per_page": fmt.Sprintf("%d", perPage),
				})
				if err != nil {
					return apiErr(fmt.Errorf("live search: %w", err))
				}
				var env struct {
					Results []json.RawMessage `json:"results"`
				}
				if err := json.Unmarshal(data, &env); err != nil {
					return apiErr(fmt.Errorf("parsing search response: %w", err))
				}
				for _, raw := range env.Results {
					var m map[string]any
					if json.Unmarshal(raw, &m) == nil {
						rows = append(rows, m)
					}
				}
			}

			cleanSnippets(rows)

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printNovelJSON(cmd.OutOrStdout(), rows, flags, source)
			}
			if len(rows) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No matching documents found.")
				return nil
			}
			return printAutoTable(cmd.OutOrStdout(), rows)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Local SQLite database path")
	cmd.Flags().IntVar(&limit, "limit", 50, "Maximum results")
	cmd.Flags().BoolVar(&live, "live", false, "Skip the local store and search support.logi.com live")
	return cmd
}
