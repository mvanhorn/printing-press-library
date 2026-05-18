package cli

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/anylist/internal/anylist"
	"github.com/mvanhorn/printing-press-library/library/food-and-dining/anylist/internal/config"
	"github.com/mvanhorn/printing-press-library/library/food-and-dining/anylist/internal/store"

	"github.com/spf13/cobra"
)

func newSyncCmd(flags *rootFlags) *cobra.Command {
	var quietFlag bool

	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Sync all AnyList data to local SQLite cache",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			cfg, err := config.Load(flags.configPath)
			if err != nil {
				return configErr(err)
			}

			if err := anylist.EnsureClientIdentifier(cfg); err != nil {
				return fmt.Errorf("ensuring client identifier: %w", err)
			}

			if cfg.AccessToken == "" {
				return authErr(fmt.Errorf("not authenticated — run 'anylist-pp-cli auth login' first"))
			}

			alClient := anylist.New(cfg)
			userData, err := alClient.GetUserData(ctx)
			if err != nil {
				return fmt.Errorf("fetching user data: %w", err)
			}

			st, err := store.Open(cfg)
			if err != nil {
				return fmt.Errorf("opening store: %w", err)
			}
			defer st.Close()

			if err := st.SyncFromUserData(userData); err != nil {
				return fmt.Errorf("syncing data: %w", err)
			}
			// Explicitly stamp each entity with a typed upsert so partial-sync
			// callers can track per-entity freshness without a full re-scan.
			now := time.Now()
			for _, entity := range defaultSyncResources {
				_ = st.UpsertSyncTimestamp(entity, now)
			}

			if quietFlag || flags.quiet {
				return nil
			}

			// Count synced data
			lists, _ := st.GetLists()
			listCount := len(lists)
			itemCount := 0
			for _, l := range lists {
				items, _ := st.GetItems(l.ID, nil)
				itemCount += len(items)
			}
			recipes, _ := st.GetRecipes()
			recipeCount := len(recipes)

			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"synced":    true,
					"lists":     listCount,
					"items":     itemCount,
					"recipes":   recipeCount,
					"synced_at": time.Now().Format(time.RFC3339),
				}, flags)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Synced: %d lists, %d items, %d recipes\n",
				listCount, itemCount, recipeCount)
			return nil
		},
	}

	cmd.Flags().BoolVar(&quietFlag, "quiet", false, "Suppress output")

	cmd.AddCommand(newSyncStatusCmd(flags))

	return cmd
}

func newSyncStatusCmd(flags *rootFlags) *cobra.Command {
	var staleAfter time.Duration

	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show local cache freshness and exit 1 if stale",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(flags.configPath)
			if err != nil {
				return configErr(err)
			}

			st, err := store.Open(cfg)
			if err != nil {
				return fmt.Errorf("opening store: %w", err)
			}
			defer st.Close()

			meta, err := st.GetSyncMeta()
			if err != nil {
				return fmt.Errorf("reading sync meta: %w", err)
			}

			if len(meta) == 0 {
				if flags.asJSON {
					return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
						"synced": false,
						"hint":   "run 'anylist-pp-cli sync' first",
					}, flags)
				}
				fmt.Fprintln(cmd.OutOrStdout(), "never synced — run 'anylist-pp-cli sync' first")
				return &cliError{code: 1, err: fmt.Errorf("never synced")}
			}

			now := time.Now()
			type entityStatus struct {
				Entity   string `json:"entity"`
				LastSync string `json:"last_sync"`
				Age      string `json:"age"`
				Status   string `json:"status"`
			}

			var statuses []entityStatus
			anyStale := false

			for _, entity := range []string{"lists", "recipes", "meal", "stores"} {
				t, ok := meta[entity]
				status := "unknown"
				lastSync := "never"
				age := ""
				if ok {
					age = formatAge(now.Sub(t))
					lastSync = t.Format("2006-01-02 15:04:05")
					if now.Sub(t) > staleAfter {
						status = "stale"
						anyStale = true
					} else {
						status = "fresh"
					}
				} else {
					anyStale = true
				}
				statuses = append(statuses, entityStatus{
					Entity:   entity,
					LastSync: lastSync,
					Age:      age,
					Status:   status,
				})
			}

			if flags.asJSON {
				raw, _ := json.Marshal(map[string]any{
					"entities": statuses,
					"stale":    anyStale,
				})
				fmt.Fprintln(cmd.OutOrStdout(), string(raw))
				if anyStale {
					return &cliError{code: 1, err: fmt.Errorf("stale cache")}
				}
				return nil
			}

			// Print table
			tw := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintln(tw, "ENTITY\tLAST SYNC\tAGE\tSTATUS")
			for _, s := range statuses {
				fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", s.Entity, s.LastSync, s.Age, s.Status)
			}
			tw.Flush()

			if anyStale {
				fmt.Fprintln(cmd.OutOrStdout(), "\nCache is stale — run 'anylist-pp-cli sync' to refresh")
				return &cliError{code: 1, err: fmt.Errorf("stale cache")}
			}
			return nil
		},
	}

	cmd.Flags().DurationVar(&staleAfter, "stale-after", 24*time.Hour, "Duration after which cache is considered stale")
	return cmd
}

func formatAge(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%.1fh", d.Hours())
	}
	return fmt.Sprintf("%.1fd", d.Hours()/24)
}
