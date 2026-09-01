package cli

import (
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/anylist/internal/anylist"
	"github.com/mvanhorn/printing-press-library/library/food-and-dining/anylist/internal/anylist/pb"
	"github.com/spf13/cobra"
)

// recentChunk pairs a recent-items starter-list chunk with the entry inside
// it that matched a selector.
type recentChunk struct {
	list *pb.StarterList
	item *pb.ListItem
}

// recentChunks returns every chunk of the Recent Items store. The store is
// sharded across many starter lists, so every reader and writer must scan
// them all; a scan of a single chunk can never be exhaustive.
func recentChunks(data *pb.PBUserDataResponse) []*pb.StarterList {
	if data == nil || data.GetStarterListsResponse() == nil {
		return nil
	}
	batch := data.GetStarterListsResponse().GetRecentItemListsResponse()
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

// resolveRecentEntry scans every recent-items chunk for the selector. The
// selector is an exact item ID or an exact name (case-insensitive). An
// optional chunkSelector narrows the scan to one chunk by ID or name.
// Multiple matches are rejected with the full candidate set returned so the
// caller can report how to disambiguate; verification after a removal must
// check absence across all chunks, never just the written one.
func resolveRecentEntry(data *pb.PBUserDataResponse, itemSelector, chunkSelector string) (*recentChunk, []*recentChunk, error) {
	itemSelector = strings.TrimSpace(itemSelector)
	chunkSelector = strings.TrimSpace(chunkSelector)
	if itemSelector == "" {
		return nil, nil, fmt.Errorf("--item is required")
	}
	var matches []*recentChunk
	for _, list := range recentChunks(data) {
		if list == nil || list.GetIdentifier() == "" {
			continue
		}
		if chunkSelector != "" &&
			!strings.EqualFold(strings.TrimSpace(list.GetIdentifier()), chunkSelector) &&
			!strings.EqualFold(strings.TrimSpace(list.GetName()), chunkSelector) {
			continue
		}
		for _, item := range list.GetItems() {
			if item == nil {
				continue
			}
			if item.GetIdentifier() == itemSelector || strings.EqualFold(strings.TrimSpace(item.GetName()), itemSelector) {
				matches = append(matches, &recentChunk{list: list, item: item})
			}
		}
	}
	if len(matches) == 0 {
		return nil, nil, fmt.Errorf("no recent-items entry matches %q", itemSelector)
	}
	if len(matches) > 1 {
		return nil, matches, fmt.Errorf("recent-items selector %q matches %d entries; pass the exact item ID or --chunk to disambiguate", itemSelector, len(matches))
	}
	return matches[0], matches, nil
}

// verifyRecentAbsence positively establishes that itemID is absent from every
// chunk of the read-back. It deliberately does not reuse resolveRecentEntry:
// that resolver's error path conflates "no match" with "ambiguous match", so
// an entry still present in several chunks would look like verified absence.
// A read-back without any recent chunks is likewise inconclusive and fails
// closed.
func verifyRecentAbsence(data *pb.PBUserDataResponse, itemID, itemName string) error {
	chunks := recentChunks(data)
	if len(chunks) == 0 {
		return fmt.Errorf("recent remove verification failed: read-back contained no recent-items chunks")
	}
	var present []string
	for _, list := range chunks {
		if list == nil || list.GetIdentifier() == "" {
			continue
		}
		for _, item := range list.GetItems() {
			if item != nil && item.GetIdentifier() == itemID {
				present = append(present, list.GetIdentifier())
				break
			}
		}
	}
	if len(present) > 0 {
		return fmt.Errorf("recent remove verification failed: entry %q (id %s) is still present in chunk(s) %s", itemName, itemID, strings.Join(present, ", "))
	}
	return nil
}

func newItemsRecentRemoveCmd(flags *rootFlags) *cobra.Command {
	var itemSelector, chunkSelector string
	var apply bool
	cmd := &cobra.Command{
		Use:   "remove",
		Short: "Remove an entry from the Recent Items store (preview unless --apply)",
		Long: `Removes one entry from the Recent Items store — the autocomplete
backing store sharded across many starter-list chunks. The selector is an
exact item ID or an exact item name (case-insensitive); a name present in
several chunks is rejected until disambiguated with the exact item ID or
--chunk. The removal is verified by a fresh read-back of every chunk before
the local cache is updated.`,
		Example: `  anylist-pp-cli items recent remove --item "bell peppers"
  anylist-pp-cli items recent remove --item <item-id> --chunk <chunk-list-id> --apply`,
		Annotations: map[string]string{
			"pp:endpoint": "starter-lists.remove",
			"pp:method":   "POST",
			"pp:path":     "/data/starter-lists/update",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			itemSelector = strings.TrimSpace(itemSelector)
			chunkSelector = strings.TrimSpace(chunkSelector)
			if itemSelector == "" {
				return fmt.Errorf("required flag \"item\" not set")
			}
			if flags.dryRun {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"dry_run": true, "status": "preview", "action": "remove", "kind": "recent",
					"item": itemSelector, "chunk": chunkSelector, "apply": apply,
				}, flags)
			}
			if !apply {
				cfg, st, err := openAuthedLocalStore(flags)
				if err != nil {
					return err
				}
				defer st.Close()
				client := anylist.New(cfg)
				liveData, err := client.GetUserData(cmd.Context())
				if err != nil {
					return fmt.Errorf("reading live recent items: %w", err)
				}
				chunk, _, err := resolveRecentEntry(liveData, itemSelector, chunkSelector)
				if err != nil {
					return err
				}
				result := map[string]any{
					"status":   "preview",
					"action":   "remove",
					"kind":     "recent",
					"item":     chunk.item.GetName(),
					"item_id":  chunk.item.GetIdentifier(),
					"chunk":    chunk.list.GetName(),
					"chunk_id": chunk.list.GetIdentifier(),
					"apply":    false,
				}
				if flags.asJSON {
					return printJSONFiltered(cmd.OutOrStdout(), result, flags)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "Preview: would remove %q (id %s) from recent chunk %s (id %s); pass --apply to write\n",
					chunk.item.GetName(), chunk.item.GetIdentifier(), chunk.list.GetName(), chunk.list.GetIdentifier())
				return nil
			}
			if !starterListWritesEnabled() {
				return fmt.Errorf("recent-items mutation is disabled until AnyList's starter-list protobuf round-trip is verified")
			}
			cfg, st, err := openAuthedLocalStore(flags)
			if err != nil {
				return err
			}
			defer st.Close()
			client := anylist.New(cfg)
			liveData, err := client.GetUserData(cmd.Context())
			if err != nil {
				return fmt.Errorf("reading live recent items: %w", err)
			}
			chunk, _, err := resolveRecentEntry(liveData, itemSelector, chunkSelector)
			if err != nil {
				return err
			}
			chunkID, itemName, itemID := chunk.list.GetIdentifier(), chunk.item.GetName(), chunk.item.GetIdentifier()
			if err := client.RemoveStarterListItem(cmd.Context(), chunkID, chunk.item); err != nil {
				return fmt.Errorf("removing recent entry %q: %w", itemName, err)
			}
			verifiedData, err := client.GetUserData(cmd.Context())
			if err != nil {
				return fmt.Errorf("verifying removal of recent entry %q: %w", itemName, err)
			}
			if err := verifyRecentAbsence(verifiedData, itemID, itemName); err != nil {
				return err
			}
			if err := st.SyncFromUserData(verifiedData); err != nil {
				return fmt.Errorf("updating local cache after recent remove: %w", err)
			}
			if flags.quiet {
				return nil
			}
			result := map[string]any{
				"removed": true, "kind": "recent", "item_id": itemID, "name": itemName,
				"chunk_id": chunkID, "chunk": chunk.list.GetName(), "verified": true,
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), result, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Removed %q from recent chunk %s (id %s)\n", itemName, chunk.list.GetName(), chunkID)
			return nil
		},
	}
	cmd.Flags().StringVar(&itemSelector, "item", "", "Item name or stable item ID")
	cmd.Flags().StringVar(&chunkSelector, "chunk", "", "Optional chunk list ID or name to narrow the search")
	cmd.Flags().BoolVar(&apply, "apply", false, "Apply the removal; preview is the default")
	return cmd
}
