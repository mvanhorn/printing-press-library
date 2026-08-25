// Copyright 2026 Micah Baldwin and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored catalog listing commands (collections, keywords, cameras, lenses).
// Shared factories: each is registered top-level and mirrored under the hidden
// 'catalog' group for the typed endpoint surface.
package cli

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/lightroom-classic/internal/lrcat"
)

func runNamedCountListing(flags *rootFlags, fetch func(*lrcat.Catalog) ([]lrcat.NamedCount, error), what string) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		if dryRunOK(flags) {
			fmt.Fprintf(cmd.OutOrStdout(), "would list %s from the local catalog\n", what)
			return nil
		}
		ctx, cancel := boundCtx(cmd.Context(), flags)
		defer cancel()
		cat, err := openCatalog(ctx, flags)
		if err != nil {
			return err
		}
		defer cat.Close()
		items, err := fetch(cat)
		if err != nil {
			return err
		}
		return emitLrcat(cmd, flags, items, func(w io.Writer) {
			for _, it := range items {
				fmt.Fprintf(w, "%-40s %7d\n", it.Name, it.ImageCount)
			}
			fmt.Fprintf(w, "%d %s\n", len(items), what)
		})
	}
}

func runGearListing(flags *rootFlags, fetch func(*lrcat.Catalog) ([]lrcat.Gear, error), what string) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		if dryRunOK(flags) {
			fmt.Fprintf(cmd.OutOrStdout(), "would list %s from the local catalog\n", what)
			return nil
		}
		ctx, cancel := boundCtx(cmd.Context(), flags)
		defer cancel()
		cat, err := openCatalog(ctx, flags)
		if err != nil {
			return err
		}
		defer cat.Close()
		items, err := fetch(cat)
		if err != nil {
			return err
		}
		return emitLrcat(cmd, flags, items, func(w io.Writer) {
			for _, it := range items {
				fmt.Fprintf(w, "%-40s %7d   %s → %s\n", it.Name, it.ImageCount, it.FirstSeen, it.LastSeen)
			}
			fmt.Fprintf(w, "%d %s\n", len(items), what)
		})
	}
}

func makeCollectionsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "collections",
		Short:       "List collections with image counts",
		Example:     "  lightroom-classic-pp-cli collections --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
	}
	cmd.RunE = func(c *cobra.Command, a []string) error {
		return runNamedCountListing(flags, func(cat *lrcat.Catalog) ([]lrcat.NamedCount, error) {
			return cat.Collections(c.Context())
		}, "collections")(c, a)
	}
	return cmd
}

func makeKeywordsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "keywords",
		Short:       "List keywords with image counts",
		Example:     "  lightroom-classic-pp-cli keywords --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
	}
	cmd.RunE = func(c *cobra.Command, a []string) error {
		return runNamedCountListing(flags, func(cat *lrcat.Catalog) ([]lrcat.NamedCount, error) {
			return cat.Keywords(c.Context())
		}, "keywords")(c, a)
	}
	return cmd
}

func makeCamerasCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "cameras",
		Short:       "List camera bodies with counts and first/last-seen dates",
		Example:     "  lightroom-classic-pp-cli cameras --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
	}
	cmd.RunE = func(c *cobra.Command, a []string) error {
		return runGearListing(flags, func(cat *lrcat.Catalog) ([]lrcat.Gear, error) {
			return cat.Cameras(c.Context())
		}, "cameras")(c, a)
	}
	return cmd
}

func makeLensesCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "lenses",
		Short:       "List lenses with counts and first/last-seen dates",
		Example:     "  lightroom-classic-pp-cli lenses --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
	}
	cmd.RunE = func(c *cobra.Command, a []string) error {
		return runGearListing(flags, func(cat *lrcat.Catalog) ([]lrcat.Gear, error) {
			return cat.Lenses(c.Context())
		}, "lenses")(c, a)
	}
	return cmd
}
