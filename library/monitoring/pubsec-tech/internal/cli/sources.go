// Sources command — list, enable, disable, and seed federal-tech RSS sources.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/monitoring/pubsec-tech/internal/news"
	"github.com/mvanhorn/printing-press-library/library/monitoring/pubsec-tech/internal/store"
)

func newSourcesCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sources",
		Short: "Manage RSS news sources (list, enable, disable, seed defaults)",
		Long: "Sources are the federal-tech RSS feeds the CLI pulls articles from. " +
			"Default set includes Nextgov/FCW, FedScoop, CyberScoop, MeriTalk, GovExec Technology, and Federal News Network. " +
			"State/local sources (StateScoop, Route Fifty, GovTech) are opt-in.",
		Example: "  pubsec-tech-pp-cli sources list --json\n  pubsec-tech-pp-cli sources enable govtech",
	}
	cmd.AddCommand(newSourcesListCmd(flags))
	cmd.AddCommand(newSourcesSeedCmd(flags))
	cmd.AddCommand(newSourcesEnableCmd(flags))
	cmd.AddCommand(newSourcesDisableCmd(flags))
	return cmd
}

func openExtrasStore(ctx context.Context) (*store.Store, error) {
	dbPath := defaultDBPath("pubsec-tech-pp-cli")
	s, err := store.OpenWithContext(ctx, dbPath)
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}
	if err := s.MigrateExtras(ctx); err != nil {
		_ = s.Close()
		return nil, fmt.Errorf("migrating extras schema: %w", err)
	}
	return s, nil
}

// ensureSourcesSeeded inserts the DefaultSources set into the store if no
// sources are present yet. Idempotent: skips when any source already exists.
func ensureSourcesSeeded(ctx context.Context, s *store.Store) (int, error) {
	existing, err := s.ListSources(ctx, false)
	if err != nil {
		return 0, err
	}
	if len(existing) > 0 {
		return 0, nil
	}
	seeded := 0
	for _, src := range news.DefaultSources {
		if err := s.UpsertSource(ctx, src); err != nil {
			return seeded, err
		}
		seeded++
	}
	return seeded, nil
}

func newSourcesListCmd(flags *rootFlags) *cobra.Command {
	var enabledOnly bool
	cmd := &cobra.Command{
		Use:         "list",
		Short:       "List configured RSS news sources",
		Example:     "  pubsec-tech-pp-cli sources list\n  pubsec-tech-pp-cli sources list --enabled --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			s, err := openExtrasStore(ctx)
			if err != nil {
				return err
			}
			defer s.Close()
			if _, err := ensureSourcesSeeded(ctx, s); err != nil {
				return err
			}
			sources, err := s.ListSources(ctx, enabledOnly)
			if err != nil {
				return err
			}
			type row struct {
				ID            string `json:"id"`
				Name          string `json:"name"`
				FeedURL       string `json:"feed_url"`
				Category      string `json:"category"`
				Tier          string `json:"tier"`
				Enabled       bool   `json:"enabled"`
				LastFetchedAt string `json:"last_fetched_at,omitempty"`
			}
			rows := make([]row, 0, len(sources))
			for _, src := range sources {
				r := row{ID: src.ID, Name: src.Name, FeedURL: src.FeedURL, Category: src.Category, Tier: src.Tier, Enabled: src.Enabled}
				if src.LastFetchedAt.Valid {
					r.LastFetchedAt = src.LastFetchedAt.Time.Format("2006-01-02T15:04:05Z07:00")
				}
				rows = append(rows, r)
			}
			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
			}
			tw := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintln(tw, "ID\tNAME\tTIER\tENABLED\tFEED URL")
			for _, r := range rows {
				en := "no"
				if r.Enabled {
					en = "yes"
				}
				fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", r.ID, r.Name, r.Tier, en, r.FeedURL)
			}
			return tw.Flush()
		},
	}
	cmd.Flags().BoolVar(&enabledOnly, "enabled", false, "Show only enabled sources")
	return cmd
}

func newSourcesSeedCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "seed",
		Short:   "Seed the default federal-tech RSS sources (idempotent)",
		Example: "  pubsec-tech-pp-cli sources seed",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			s, err := openExtrasStore(ctx)
			if err != nil {
				return err
			}
			defer s.Close()
			n, err := ensureSourcesSeeded(ctx, s)
			if err != nil {
				return err
			}
			if flags.asJSON {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(map[string]any{"seeded": n})
			}
			if n == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "Sources already seeded; no changes.")
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "Seeded %d default sources.\n", n)
			}
			return nil
		},
	}
	return cmd
}

func newSourcesEnableCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "enable <source-id>",
		Short:   "Enable a source so it will be fetched on `news sync`",
		Example: "  pubsec-tech-pp-cli sources enable govtech",
		RunE:    sourcesToggleRunE(flags, true),
	}
	return cmd
}

func newSourcesDisableCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "disable <source-id>",
		Short:   "Disable a source so it will be skipped on `news sync`",
		Example: "  pubsec-tech-pp-cli sources disable statescoop",
		RunE:    sourcesToggleRunE(flags, false),
	}
	return cmd
}

func sourcesToggleRunE(flags *rootFlags, enable bool) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return cmd.Help()
		}
		if dryRunOK(flags) {
			return nil
		}
		ctx := cmd.Context()
		s, err := openExtrasStore(ctx)
		if err != nil {
			return err
		}
		defer s.Close()
		if _, err := ensureSourcesSeeded(ctx, s); err != nil {
			return err
		}
		id := strings.TrimSpace(args[0])
		if err := s.SetSourceEnabled(ctx, id, enable); err != nil {
			return notFoundErr(err)
		}
		if flags.asJSON {
			return json.NewEncoder(cmd.OutOrStdout()).Encode(map[string]any{"id": id, "enabled": enable})
		}
		state := "disabled"
		if enable {
			state = "enabled"
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Source %s is now %s.\n", id, state)
		return nil
	}
}
