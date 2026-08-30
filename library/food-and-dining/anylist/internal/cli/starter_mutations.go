package cli

import (
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/anylist/internal/anylist"
	"github.com/mvanhorn/printing-press-library/library/food-and-dining/anylist/internal/anylist/pb"
	"github.com/spf13/cobra"
)

type starterListKind uint8

const (
	starterListUser starterListKind = iota
	starterListFavorites
)

// The current PBListOperation starter-list protocol has been live-proven with
// fresh read-back and cleanup; callers still require --apply and verify again.
func starterListWritesEnabled() bool { return true }

func (k starterListKind) label() string {
	if k == starterListFavorites {
		return "favorites"
	}
	return "starters"
}

func (k starterListKind) lists(data *pb.PBUserDataResponse) []*pb.StarterList {
	if data == nil || data.GetStarterListsResponse() == nil {
		return nil
	}
	var batch *pb.StarterListBatchResponse
	if k == starterListFavorites {
		batch = data.GetStarterListsResponse().GetFavoriteItemListsResponse()
	} else {
		batch = data.GetStarterListsResponse().GetUserListsResponse()
	}
	if batch == nil {
		return nil
	}
	lists := make([]*pb.StarterList, 0, len(batch.GetListResponses()))
	for _, response := range batch.GetListResponses() {
		if response != nil && response.GetStarterList() != nil {
			lists = append(lists, response.GetStarterList())
		}
	}
	return lists
}

func starterListByID(data *pb.PBUserDataResponse, kind starterListKind, id string) (*pb.StarterList, bool) {
	for _, list := range kind.lists(data) {
		if list.GetIdentifier() == id {
			return list, true
		}
	}
	return nil, false
}

// resolveStarterList uses exact, case-insensitive selectors only. For
// favorites, --list may be the favorite-list ID, its linked shopping-list ID,
// or the linked shopping-list name. For user starters it is the starter-list
// ID or name. A selector that matches more than one stable ID is rejected.
func resolveStarterList(data *pb.PBUserDataResponse, kind starterListKind, selector string) (*pb.StarterList, error) {
	selector = strings.TrimSpace(selector)
	if selector == "" {
		return nil, fmt.Errorf("--list is required")
	}
	matches := map[string]*pb.StarterList{}
	for _, list := range kind.lists(data) {
		if list == nil || list.GetIdentifier() == "" {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(list.GetIdentifier()), selector) ||
			strings.EqualFold(strings.TrimSpace(list.GetName()), selector) {
			matches[list.GetIdentifier()] = list
			continue
		}
		if kind == starterListFavorites {
			if strings.EqualFold(strings.TrimSpace(list.GetListId()), selector) {
				matches[list.GetIdentifier()] = list
				continue
			}
			if shoppingList, found := findLiveShoppingListByID(data, list.GetListId()); found &&
				strings.EqualFold(strings.TrimSpace(shoppingList.GetName()), selector) {
				matches[list.GetIdentifier()] = list
			}
		}
	}
	if len(matches) == 0 {
		return nil, fmt.Errorf("%s list %q not found", kind.label(), selector)
	}
	if len(matches) > 1 {
		return nil, fmt.Errorf("%s list selector %q is ambiguous; use a stable ID", kind.label(), selector)
	}
	for _, list := range matches {
		return list, nil
	}
	return nil, fmt.Errorf("%s list %q not found", kind.label(), selector)
}

func starterItemBySelector(list *pb.StarterList, selector string) (*pb.ListItem, error) {
	selector = strings.TrimSpace(selector)
	if selector == "" {
		return nil, fmt.Errorf("--item is required")
	}
	var matches []*pb.ListItem
	for _, item := range list.GetItems() {
		if item == nil {
			continue
		}
		if item.GetIdentifier() == selector || strings.EqualFold(strings.TrimSpace(item.GetName()), selector) {
			matches = append(matches, item)
		}
	}
	if len(matches) == 0 {
		return nil, fmt.Errorf("starter item %q not found in %q", selector, list.GetName())
	}
	if len(matches) > 1 {
		return nil, fmt.Errorf("starter item selector %q is ambiguous; use its stable ID", selector)
	}
	return matches[0], nil
}

func starterListAddPreview(cmd *cobra.Command, flags *rootFlags, kind starterListKind, list, name, quantity, details, category string, apply bool) error {
	result := map[string]any{
		"status":   "preview",
		"action":   "add",
		"kind":     kind.label(),
		"list":     strings.TrimSpace(list),
		"name":     strings.TrimSpace(name),
		"quantity": strings.TrimSpace(quantity),
		"details":  strings.TrimSpace(details),
		"category": strings.TrimSpace(category),
		"apply":    apply,
	}
	if flags.asJSON {
		return printJSONFiltered(cmd.OutOrStdout(), result, flags)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Preview: would add %q to %s list %q (pass --apply to write)\n", strings.TrimSpace(name), kind.label(), strings.TrimSpace(list))
	return nil
}

func newStarterListAddCmd(flags *rootFlags, kind starterListKind) *cobra.Command {
	var listSelector, name, quantity, details, category string
	var apply bool
	cmd := &cobra.Command{
		Use:     "add",
		Short:   fmt.Sprintf("Add an item to a %s list (preview unless --apply)", kind.label()),
		Example: fmt.Sprintf("  anylist-pp-cli %s add --list Groceries --name \"Milk\"", kind.label()),
		Annotations: map[string]string{
			"pp:endpoint": "starter-lists.add",
			"pp:method":   "POST",
			"pp:path":     "/data/starter-lists/update",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			listSelector = strings.TrimSpace(listSelector)
			name = strings.TrimSpace(name)
			if listSelector == "" {
				return fmt.Errorf("required flag \"list\" not set")
			}
			if name == "" {
				return fmt.Errorf("required flag \"name\" not set")
			}
			if !apply || flags.dryRun {
				return starterListAddPreview(cmd, flags, kind, listSelector, name, quantity, details, category, apply)
			}
			if !starterListWritesEnabled() {
				return fmt.Errorf("%s mutation is disabled until AnyList's starter-list protobuf round-trip is verified", kind.label())
			}

			cfg, st, err := openAuthedLocalStore(flags)
			if err != nil {
				return err
			}
			defer st.Close()
			ctx := cmd.Context()
			client := anylist.New(cfg)
			liveData, err := client.GetUserData(ctx)
			if err != nil {
				return fmt.Errorf("reading live %s: %w", kind.label(), err)
			}
			list, err := resolveStarterList(liveData, kind, listSelector)
			if err != nil {
				return err
			}
			for _, existing := range list.GetItems() {
				if existing != nil && strings.EqualFold(strings.TrimSpace(existing.GetName()), name) {
					return fmt.Errorf("%s item %q already exists in %q; refusing to create a duplicate", kind.label(), name, list.GetName())
				}
			}
			item := &pb.ListItem{Name: name, Quantity: strings.TrimSpace(quantity), Details: strings.TrimSpace(details), Category: strings.TrimSpace(category), UserId: cfg.UserID}
			itemID, err := client.AddStarterListItem(ctx, list.GetIdentifier(), item)
			if err != nil {
				return fmt.Errorf("adding %s item %q: %w", kind.label(), name, err)
			}
			verifiedData, err := client.GetUserData(ctx)
			if err != nil {
				return fmt.Errorf("verifying %s item %q: %w", kind.label(), name, err)
			}
			verifiedList, found := starterListByID(verifiedData, kind, list.GetIdentifier())
			if !found {
				return fmt.Errorf("%s add verification failed: list %q was not read back", kind.label(), list.GetName())
			}
			verifiedItem, err := starterItemBySelector(verifiedList, itemID)
			if err != nil || !strings.EqualFold(verifiedItem.GetName(), name) {
				return fmt.Errorf("%s add verification failed: item %q was not read back", kind.label(), name)
			}
			if err := st.SyncFromUserData(verifiedData); err != nil {
				return fmt.Errorf("updating local cache after %s add: %w", kind.label(), err)
			}
			result := map[string]any{"added": true, "kind": kind.label(), "list_id": list.GetIdentifier(), "list": list.GetName(), "item_id": itemID, "name": verifiedItem.GetName(), "verified": true}
			if flags.quiet {
				return nil
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), result, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Added %q to %s list %q\n", verifiedItem.GetName(), kind.label(), list.GetName())
			return nil
		},
	}
	cmd.Flags().StringVar(&listSelector, "list", "", "Starter-list ID or name; for favorites, linked shopping-list ID or name")
	cmd.Flags().StringVar(&name, "name", "", "Item name")
	cmd.Flags().StringVar(&quantity, "quantity", "", "Optional item quantity")
	cmd.Flags().StringVar(&details, "details", "", "Optional item details")
	cmd.Flags().StringVar(&category, "category", "", "Optional item category")
	cmd.Flags().BoolVar(&apply, "apply", false, "Apply the mutation; preview is the default")
	return cmd
}

func newStarterListRemoveCmd(flags *rootFlags, kind starterListKind) *cobra.Command {
	var listSelector, itemSelector string
	var apply bool
	cmd := &cobra.Command{
		Use:     "remove",
		Short:   fmt.Sprintf("Remove an item from a %s list (preview unless --apply)", kind.label()),
		Example: fmt.Sprintf("  anylist-pp-cli %s remove --list Groceries --item \"Milk\" --apply", kind.label()),
		Annotations: map[string]string{
			"pp:endpoint": "starter-lists.remove",
			"pp:method":   "POST",
			"pp:path":     "/data/starter-lists/update",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			listSelector = strings.TrimSpace(listSelector)
			itemSelector = strings.TrimSpace(itemSelector)
			if listSelector == "" {
				return fmt.Errorf("required flag \"list\" not set")
			}
			if itemSelector == "" {
				return fmt.Errorf("required flag \"item\" not set")
			}
			if !apply || flags.dryRun {
				result := map[string]any{"status": "preview", "action": "remove", "kind": kind.label(), "list": listSelector, "item": itemSelector, "apply": apply}
				if flags.asJSON {
					return printJSONFiltered(cmd.OutOrStdout(), result, flags)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "Preview: would remove %q from %s list %q (pass --apply to write)\n", itemSelector, kind.label(), listSelector)
				return nil
			}
			if !starterListWritesEnabled() {
				return fmt.Errorf("%s mutation is disabled until AnyList's starter-list protobuf round-trip is verified", kind.label())
			}

			cfg, st, err := openAuthedLocalStore(flags)
			if err != nil {
				return err
			}
			defer st.Close()
			ctx := cmd.Context()
			client := anylist.New(cfg)
			liveData, err := client.GetUserData(ctx)
			if err != nil {
				return fmt.Errorf("reading live %s: %w", kind.label(), err)
			}
			list, err := resolveStarterList(liveData, kind, listSelector)
			if err != nil {
				return err
			}
			item, err := starterItemBySelector(list, itemSelector)
			if err != nil {
				return err
			}
			itemID, itemName := item.GetIdentifier(), item.GetName()
			if err := client.RemoveStarterListItem(ctx, list.GetIdentifier(), item); err != nil {
				return fmt.Errorf("removing %s item %q: %w", kind.label(), itemName, err)
			}
			verifiedData, err := client.GetUserData(ctx)
			if err != nil {
				return fmt.Errorf("verifying %s item %q: %w", kind.label(), itemName, err)
			}
			verifiedList, found := starterListByID(verifiedData, kind, list.GetIdentifier())
			if !found {
				return fmt.Errorf("%s remove verification failed: list %q was not read back", kind.label(), list.GetName())
			}
			if _, stillPresent := starterItemBySelector(verifiedList, itemID); stillPresent == nil {
				return fmt.Errorf("%s remove verification failed: item %q is still present", kind.label(), itemName)
			}
			if err := st.SyncFromUserData(verifiedData); err != nil {
				return fmt.Errorf("updating local cache after %s remove: %w", kind.label(), err)
			}
			result := map[string]any{"removed": true, "kind": kind.label(), "list_id": list.GetIdentifier(), "list": list.GetName(), "item_id": itemID, "name": itemName, "verified": true}
			if flags.quiet {
				return nil
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), result, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Removed %q from %s list %q\n", itemName, kind.label(), list.GetName())
			return nil
		},
	}
	cmd.Flags().StringVar(&listSelector, "list", "", "Starter-list ID or name; for favorites, linked shopping-list ID or name")
	cmd.Flags().StringVar(&itemSelector, "item", "", "Item name or stable item ID")
	cmd.Flags().BoolVar(&apply, "apply", false, "Apply the mutation; preview is the default")
	return cmd
}
