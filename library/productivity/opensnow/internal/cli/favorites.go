package cli

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/opensnow/internal/store"

	"github.com/spf13/cobra"
)

func newFavoritesCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "favorites",
		Short: "Manage favorite locations for dashboard, overnight, and powder-rank commands",
	}
	cmd.AddCommand(newFavoritesAddCmd(flags))
	cmd.AddCommand(newFavoritesRemoveCmd(flags))
	cmd.AddCommand(newFavoritesListCmd(flags))
	return cmd
}

func openStore(ctx context.Context) (*store.Store, error) {
	dbPath := defaultDBPath("opensnow-pp-cli")
	return store.OpenWithContext(ctx, dbPath)
}

func newFavoritesAddCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:     "add <slug> [slug...]",
		Short:   "Add locations to favorites",
		Example: "  opensnow-pp-cli favorites add vail aspen",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			db, err := openStore(cmd.Context())
			if err != nil {
				return fmt.Errorf("opening store: %w", err)
			}
			defer db.Close()

			added := make([]string, 0, len(args))
			for _, slug := range args {
				if err := db.AddFavorite(slug); err != nil {
					fmt.Fprintf(os.Stderr, "warn: %s: %v\n", slug, err)
				} else {
					added = append(added, slug)
				}
			}

			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"added": added,
					"count": len(added),
				}, flags)
			}
			for _, s := range added {
				fmt.Fprintf(cmd.OutOrStdout(), "Added %s to favorites\n", s)
			}
			return nil
		},
	}
}

func newFavoritesRemoveCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:     "remove <slug>",
		Short:   "Remove a location from favorites",
		Example: "  opensnow-pp-cli favorites remove vail",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			db, err := openStore(cmd.Context())
			if err != nil {
				return fmt.Errorf("opening store: %w", err)
			}
			defer db.Close()

			slug := args[0]
			if err := db.RemoveFavorite(slug); err != nil {
				return fmt.Errorf("removing favorite: %w", err)
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"removed": slug,
				}, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Removed %s from favorites\n", slug)
			return nil
		},
	}
}

func newFavoritesListCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:     "list",
		Short:   "List all favorite locations",
		Example: "  opensnow-pp-cli favorites list",
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			db, err := openStore(cmd.Context())
			if err != nil {
				return fmt.Errorf("opening store: %w", err)
			}
			defer db.Close()

			slugs, err := db.ListFavorites()
			if err != nil {
				return fmt.Errorf("listing favorites: %w", err)
			}
			if len(slugs) == 0 {
				if flags.asJSON {
					return printJSONFiltered(cmd.OutOrStdout(), []string{}, flags)
				}
				fmt.Fprintln(cmd.OutOrStdout(), "No favorites. Add some with: opensnow-pp-cli favorites add <slug>")
				return nil
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), slugs, flags)
			}
			for _, s := range slugs {
				fmt.Fprintln(cmd.OutOrStdout(), s)
			}
			return nil
		},
	}
}

// loadFavorites opens the store and returns the list of favorite slugs.
// Returns a helpful error when no favorites are configured.
func loadFavorites(ctx context.Context) (*store.Store, []string, error) {
	db, err := openStore(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("opening store: %w", err)
	}
	slugs, err := db.ListFavorites()
	if err != nil {
		db.Close()
		return nil, nil, fmt.Errorf("listing favorites: %w", err)
	}
	return db, slugs, nil
}

// Ensure time import is used
var _ = time.Now
