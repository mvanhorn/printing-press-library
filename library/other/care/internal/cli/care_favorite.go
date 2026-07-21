// Copyright 2026 beetz12. Licensed under Apache-2.0.
// `favorite` / `unfavorite` - toggle a caregiver as a favorite on your care.com
// account (FavoriteProvider mutation). Hand-authored; safe across regen.
// Draft-and-confirm: dry-run by default, requires --confirm to write.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

func careFavorite(cmd *cobra.Command, flags *rootFlags, caregiverID string, on, confirm bool) error {
	// Every write is draft-and-confirm: dry-run unless --confirm (and --dry-run
	// always forces a preview). Consistent with outreach send / messages reply.
	if !confirm || dryRunOK(flags) {
		verb := "favorite"
		if !on {
			verb = "unfavorite"
		}
		fmt.Fprintf(cmd.OutOrStdout(), "DRY RUN - would %s caregiver %s. Re-run with --confirm to apply.\n", verb, caregiverID)
		return nil
	}
	to := flags.timeout
	if to <= 0 {
		to = 30 * time.Second
	}
	ctx, cancel := context.WithTimeout(cmd.Context(), to)
	defer cancel()

	name, legacyID, err := careResolveCaregiver(ctx, flags, caregiverID)
	if err != nil {
		return err
	}
	if legacyID == "" {
		return fmt.Errorf("could not resolve a member id for caregiver %s", caregiverID)
	}
	input := map[string]any{"favoriteStatus": on, "memberId": legacyID, "serviceType": "CHILD_CARE"}
	data, err := careGraphQL(ctx, flags, careMFavoriteProviderOp, careMFavoriteProvider, map[string]any{"input": input})
	if err != nil {
		return err
	}
	var res struct {
		Fav struct {
			Typename string `json:"__typename"`
			Message  string `json:"message"`
			Favorite struct {
				FavoriteStatus bool `json:"favoriteStatus"`
			} `json:"favorite"`
		} `json:"favoriteProvider"`
	}
	if err := json.Unmarshal(data, &res); err != nil {
		return fmt.Errorf("parsing favorite result: %w", err)
	}
	if res.Fav.Typename != "FavoriteCaregiverPayload" {
		return fmt.Errorf("care.com rejected the favorite: %s %s", res.Fav.Typename, res.Fav.Message)
	}
	verb := "Favorited"
	if !res.Fav.Favorite.FavoriteStatus {
		verb = "Unfavorited"
	}
	fmt.Fprintf(cmd.OutOrStdout(), "%s %s.\n", verb, name)
	return nil
}

func newCareFavoriteCmd(flags *rootFlags) *cobra.Command {
	var confirm bool
	cmd := &cobra.Command{
		Use:     "favorite <caregiverId>",
		Short:   "Mark a caregiver as a favorite on your care.com account (dry-run unless --confirm)",
		Example: "  care-pp-cli favorite 44444444-4444-4444-4444-444444444444",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return careFavorite(cmd, flags, args[0], true, confirm)
		},
	}
	cmd.Flags().BoolVar(&confirm, "confirm", false, "actually apply the change (default is dry-run)")
	return cmd
}

func newCareUnfavoriteCmd(flags *rootFlags) *cobra.Command {
	var confirm bool
	cmd := &cobra.Command{
		Use:     "unfavorite <caregiverId>",
		Short:   "Remove a caregiver from your care.com favorites (dry-run unless --confirm)",
		Example: "  care-pp-cli unfavorite 44444444-4444-4444-4444-444444444444",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return careFavorite(cmd, flags, args[0], false, confirm)
		},
	}
	cmd.Flags().BoolVar(&confirm, "confirm", false, "actually apply the change (default is dry-run)")
	return cmd
}
