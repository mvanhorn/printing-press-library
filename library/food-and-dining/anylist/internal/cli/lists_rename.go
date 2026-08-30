// Copyright 2026 Jeeves and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/anylist/internal/anylist"
	"github.com/mvanhorn/printing-press-library/library/food-and-dining/anylist/internal/anylist/pb"
	"github.com/spf13/cobra"
	"google.golang.org/protobuf/proto"
)

func newListsRenameCmd(flags *rootFlags) *cobra.Command {
	var oldName, newName string
	var stdinBody, apply bool

	cmd := &cobra.Command{
		Use:         "rename",
		Short:       "Rename a shopping list (preview unless --apply)",
		Example:     "  anylist-pp-cli lists rename --name Groceries --new-name \"Weekly Groceries\" --apply",
		Annotations: map[string]string{"pp:endpoint": "lists.rename", "pp:method": "POST", "pp:path": "/data/shopping-lists/update"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if stdinBody {
				body, err := readStdinJSONMap()
				if err != nil {
					return err
				}
				oldName = stringFromBody(body, "name")
				if oldName == "" {
					oldName = stringFromBody(body, "list")
				}
				newName = stringFromBody(body, "new_name")
				if newName == "" {
					newName = stringFromBody(body, "newName")
				}
				apply = boolFromBody(body, "apply")
			}
			oldName = strings.TrimSpace(oldName)
			newName = strings.TrimSpace(newName)
			if oldName == "" && !flags.dryRun {
				return fmt.Errorf("required flag \"name\" not set")
			}
			if newName == "" && !flags.dryRun {
				return fmt.Errorf("required flag \"new-name\" not set")
			}
			if oldName != "" && newName != "" && strings.EqualFold(oldName, newName) {
				return fmt.Errorf("new list name must differ from the current name")
			}
			if !apply || flags.dryRun {
				preview := map[string]any{"dry_run": true, "name": oldName, "new_name": newName, "apply": apply}
				if flags.asJSON {
					return printJSONFiltered(cmd.OutOrStdout(), preview, flags)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "Dry run: would rename list %q to %q (pass --apply to write)\n", oldName, newName)
				return nil
			}

			ctx := cmd.Context()
			cfg, st, err := openAuthedLocalStore(flags)
			if err != nil {
				return err
			}
			defer st.Close()
			alClient := anylist.New(cfg)
			userData, err := alClient.GetUserData(ctx)
			if err != nil {
				return fmt.Errorf("reading live lists: %w", err)
			}
			list, err := exactLiveShoppingListByName(userData, oldName)
			if err != nil {
				return err
			}
			if existing := findLiveShoppingListNameConflict(userData, list.GetIdentifier(), newName); existing != nil {
				return fmt.Errorf("a different shopping list is already named %q", newName)
			}
			updated := proto.Clone(list).(*pb.ShoppingList)
			updated.Name = newName
			if err := alClient.RenameList(ctx, list.GetIdentifier(), list.GetName(), newName, updated); err != nil {
				return fmt.Errorf("renaming list %q: %w", list.GetName(), err)
			}
			verifiedData, err := alClient.GetUserData(ctx)
			if err != nil {
				return fmt.Errorf("verifying renamed list %q: %w", list.GetName(), err)
			}
			verified, found := findLiveShoppingListByID(verifiedData, list.GetIdentifier())
			if !found || !strings.EqualFold(verified.GetName(), newName) {
				return fmt.Errorf("rename verification failed: list ID %q did not read back as %q", list.GetIdentifier(), newName)
			}
			if err := st.SyncFromUserData(verifiedData); err != nil {
				return fmt.Errorf("updating local cache after renaming list: %w", err)
			}
			if flags.quiet {
				return nil
			}
			result := map[string]any{"renamed": true, "id": verified.GetIdentifier(), "old_name": list.GetName(), "name": verified.GetName(), "verified": true}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), result, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Renamed list %q to %q\n", list.GetName(), verified.GetName())
			return nil
		},
	}
	cmd.Flags().StringVar(&oldName, "name", "", "Current list name")
	cmd.Flags().StringVar(&newName, "new-name", "", "New list name")
	cmd.Flags().BoolVar(&stdinBody, "stdin", false, "Read request body as JSON from stdin")
	cmd.Flags().BoolVar(&apply, "apply", false, "Apply the rename; preview is the default")
	return cmd
}

func exactLiveShoppingListByName(userData *pb.PBUserDataResponse, name string) (*pb.ShoppingList, error) {
	name = strings.TrimSpace(name)
	matches := map[string]*pb.ShoppingList{}
	for _, list := range liveShoppingLists(userData) {
		if list == nil || !strings.EqualFold(strings.TrimSpace(list.GetName()), name) {
			continue
		}
		matches[list.GetIdentifier()] = list
	}
	if len(matches) == 0 {
		return nil, fmt.Errorf("shopping list %q not found", name)
	}
	if len(matches) > 1 {
		return nil, fmt.Errorf("shopping list name %q is ambiguous; use a stable ID", name)
	}
	for _, match := range matches {
		return match, nil
	}
	return nil, fmt.Errorf("shopping list %q not found", name)
}

func findLiveShoppingListNameConflict(userData *pb.PBUserDataResponse, excludeID, name string) *pb.ShoppingList {
	for _, list := range liveShoppingLists(userData) {
		if list != nil && list.GetIdentifier() != excludeID && strings.EqualFold(strings.TrimSpace(list.GetName()), strings.TrimSpace(name)) {
			return list
		}
	}
	return nil
}
