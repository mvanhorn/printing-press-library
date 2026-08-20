// Copyright 2026 kothari-nikunj and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

func newWishlistCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "wishlist",
		Short:       "Manage a local saved-property list (add/list/remove)",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newWishlistAddCmd(flags))
	cmd.AddCommand(newWishlistListCmd(flags))
	cmd.AddCommand(newWishlistRemoveCmd(flags))
	return cmd
}

func newWishlistAddCmd(flags *rootFlags) *cobra.Command {
	var name string
	cmd := &cobra.Command{
		Use:         "add <property-token>",
		Short:       "Save a property to the local wishlist",
		Example:     "  hotel-goat-pp-cli wishlist add ChcIyIDJ4cf-7J3eARoKL20vMDJycDRobBAB --name \"Westin SF\"",
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			s, err := openStoreEnsured(cmd.Context(), defaultDBPath("hotel-goat-pp-cli"))
			if err != nil {
				return configErr(err)
			}
			defer s.Close()
			if err := s.WishlistAdd(cmd.Context(), args[0], name, nil); err != nil {
				return apiErr(err)
			}
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"status": "added", "property_token": args[0], "name": name}, flags)
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "Friendly name for the saved property")
	return cmd
}

func newWishlistListCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:         "list",
		Short:       "List saved hotel properties with property token, name, location, and saved-at timestamp",
		Example:     "  hotel-goat-pp-cli wishlist list\n  hotel-goat-pp-cli wishlist list --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			s, err := openStoreEnsured(cmd.Context(), defaultDBPath("hotel-goat-pp-cli"))
			if err != nil {
				return configErr(err)
			}
			defer s.Close()
			entries, err := s.WishlistList(cmd.Context())
			if err != nil {
				return apiErr(err)
			}
			env := map[string]any{
				"meta":    map[string]any{"source": "local:wishlist", "count": len(entries)},
				"results": entries,
			}
			return printJSONFiltered(cmd.OutOrStdout(), env, flags)
		},
	}
}

func newWishlistRemoveCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:         "remove <property-token>",
		Short:       "Remove a property from the wishlist",
		Example:     "  hotel-goat-pp-cli wishlist remove ChgIxxxxxxxxxxxx",
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			s, err := openStoreEnsured(cmd.Context(), defaultDBPath("hotel-goat-pp-cli"))
			if err != nil {
				return configErr(err)
			}
			defer s.Close()
			n, err := s.WishlistRemove(cmd.Context(), args[0])
			if err != nil {
				return apiErr(err)
			}
			out, _ := json.Marshal(map[string]any{"status": "removed", "removed": n})
			return printJSONFiltered(cmd.OutOrStdout(), json.RawMessage(out), flags)
		},
	}
}

var _ = fmt.Sprintf // keep fmt import if otherwise unused
