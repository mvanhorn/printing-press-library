package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/anylist/internal/anylist"
	"github.com/mvanhorn/printing-press-library/library/food-and-dining/anylist/internal/anylist/pb"
	"github.com/spf13/cobra"
	"google.golang.org/protobuf/proto"
)

func newFoldersUpdateCmd(flags *rootFlags) *cobra.Command {
	var name, newName, parent, order, color, sortPosition string
	var stdinBody, apply bool
	cmd := &cobra.Command{
		Use:         "update",
		Short:       "Rename or reorganize a list folder (preview unless --apply)",
		Example:     "  anylist-pp-cli folders update --name Dinners --color \"#123456\"\n  anylist-pp-cli folders update --name Lists --order \"Pantry,Garden,Inbox\"",
		Annotations: map[string]string{"pp:endpoint": "folders.update", "pp:method": "POST", "pp:path": "/data/list-folders/update"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if stdinBody {
				body, err := readStdinJSONMap()
				if err != nil {
					return err
				}
				name = stringFromBody(body, "name")
				newName = stringFromBody(body, "new_name")
				parent = stringFromBody(body, "parent")
				order = stringFromBody(body, "order")
				color = stringFromBody(body, "color")
				if color == "" {
					color = stringFromBody(body, "hex_color")
				}
				sortPosition = stringFromBody(body, "sort-position")
				if sortPosition == "" {
					sortPosition = stringFromBody(body, "sort_position")
				}
				apply = boolFromBody(body, "apply")
			}
			name, newName, parent, order = strings.TrimSpace(name), strings.TrimSpace(newName), strings.TrimSpace(parent), strings.TrimSpace(order)
			color, sortPosition = strings.TrimSpace(color), strings.TrimSpace(sortPosition)
			if color != "" && !validFolderHexColor(color) {
				return fmt.Errorf("color %q is not a valid #RRGGBB hex color", color)
			}
			sortValue, sortOK := folderSortPositionValue(sortPosition)
			if sortPosition != "" && !sortOK {
				return fmt.Errorf("sort position %q is not supported; use FolderSortPositionAfterLists, FolderSortPositionBeforeLists, or FolderSortPositionWithLists (there is no clear/default encoding)", sortPosition)
			}
			if name == "" && !flags.dryRun {
				return fmt.Errorf("required flag \"name\" not set")
			}
			if newName == "" && parent == "" && order == "" && color == "" && sortPosition == "" && !flags.dryRun {
				return fmt.Errorf("nothing to update; set --new-name, --parent, --order, --color, and/or --sort-position")
			}
			if order != "" && (newName != "" || parent != "" || color != "" || sortPosition != "") && !flags.dryRun {
				return fmt.Errorf("--order must be used by itself; apply folder rename, parent, color, or sort-position changes separately")
			}
			if !apply || flags.dryRun {
				preview := map[string]any{"status": "preview", "action": "update", "name": name, "new_name": newName, "parent": parent, "order": order, "apply": apply}
				if color != "" {
					preview["color"] = color
					preview["color_handler"] = "set-folder-hex-color"
				}
				if sortPosition != "" {
					preview["sort_position"] = sortPosition
					preview["sort_position_handler"] = "set-folder-sort-position"
				}
				return printJSONFiltered(cmd.OutOrStdout(), preview, flags)
			}
			metadataOnly := (color != "" || sortPosition != "") && newName == "" && parent == "" && order == ""
			if metadataOnly && !folderMetadataLiveWritesEnabled() {
				return fmt.Errorf("folder metadata live mutation is disabled until AnyList's folder protobuf round-trip is verified")
			}
			renameOnly := newName != "" && parent == "" && order == ""
			moveOnly := newName == "" && parent != "" && order == ""
			if !metadataOnly {
				if renameOnly {
					if !folderRenameLiveWritesEnabled() {
						return fmt.Errorf("folder rename live mutation is disabled until AnyList's folder protobuf round-trip is verified")
					}
				} else if moveOnly {
					if !folderMoveLiveWritesEnabled() {
						return fmt.Errorf("folder parent move is disabled until AnyList's folder protobuf round-trip is verified")
					}
				} else if order != "" && !folderChildOrderingLiveWritesEnabled() {
					return fmt.Errorf("folder child ordering is disabled until set-ordered-folder-items has a live fresh-read proof; rerun without --apply for a preview")
				} else if !folderLiveWritesEnabled() {
					return fmt.Errorf("folder live mutation is disabled until AnyList's folder protobuf round-trip is verified")
				}
			}
			ctx := cmd.Context()
			cfg, st, err := openAuthedLocalStore(flags)
			if err != nil {
				return err
			}
			defer st.Close()
			al := anylist.New(cfg)
			userData, listDataID, rootID, err := currentListFolderData(ctx, cfg)
			if err != nil {
				return err
			}
			folder, err := findLiveListFolderForMutation(userData, name)
			if err != nil {
				return err
			}
			if folder == nil {
				return fmt.Errorf("list folder %q not found", name)
			}
			if metadataOnly {
				metadataFolder := proto.Clone(folder).(*pb.PBListFolder)
				if color != "" {
					if err := al.SetListFolderHexColor(ctx, listDataID, metadataFolder, color); err != nil {
						return fmt.Errorf("updating hex color for folder %q: %w", name, err)
					}
					if metadataFolder.FolderSettings == nil {
						metadataFolder.FolderSettings = &pb.PBListFolderSettings{}
					}
					metadataFolder.FolderSettings.FolderHexColor = color
				}
				if sortPosition != "" {
					if err := al.SetListFolderSortPosition(ctx, listDataID, metadataFolder, sortValue); err != nil {
						return fmt.Errorf("updating sort position for folder %q: %w", name, err)
					}
					if metadataFolder.FolderSettings == nil {
						metadataFolder.FolderSettings = &pb.PBListFolderSettings{}
					}
					metadataFolder.FolderSettings.FolderSortPosition = int32(sortValue)
				}
				verifiedData, err := al.GetUserData(ctx)
				if err != nil {
					return fmt.Errorf("folder metadata write sent but fresh read-back failed: %w", err)
				}
				verified, found := findLiveListFolderByID(verifiedData, folder.GetIdentifier())
				if !found {
					return fmt.Errorf("folder metadata verification failed: folder disappeared")
				}
				if color != "" && verified.GetFolderSettings().GetFolderHexColor() != color {
					return fmt.Errorf("folder metadata verification failed: color read back as %q, want %q", verified.GetFolderSettings().GetFolderHexColor(), color)
				}
				if sortPosition != "" && verified.GetFolderSettings().GetFolderSortPosition() != int32(sortValue) {
					return fmt.Errorf("folder metadata verification failed: sort position read back as %d, want %d", verified.GetFolderSettings().GetFolderSortPosition(), int32(sortValue))
				}
				if err := st.SyncFromUserData(verifiedData); err != nil {
					return fmt.Errorf("updating local cache after folder metadata update: %w", err)
				}
				if flags.quiet {
					return nil
				}
				updated := map[string]any{"updated": true, "folder": verified.GetName(), "id": verified.GetIdentifier(), "verified": true}
				if color != "" {
					updated["color"] = color
				}
				if sortPosition != "" {
					updated["sort_position"] = sortPosition
				}
				return printJSONFiltered(cmd.OutOrStdout(), updated, flags)
			}
			folders := userData.GetListFoldersResponse().GetListFolders()
			parentMap := folderParentMap(folders, rootID)
			oldParent := parentMap[folder.GetIdentifier()]
			if oldParent == "" {
				oldParent = rootID
			}
			newParent := oldParent
			if parent != "" {
				if parent == rootID {
					newParent = rootID
				} else {
					p, resolveErr := resolveFolderSelector(userData, parent)
					if resolveErr != nil {
						return resolveErr
					}
					newParent = p.GetIdentifier()
				}
			}
			if newParent == folder.GetIdentifier() || folderIsDescendant(parentMap, newParent, folder.GetIdentifier()) {
				return fmt.Errorf("cannot move folder %q inside itself or one of its descendants", folder.GetName())
			}
			updated := proto.Clone(folder).(*pb.PBListFolder)
			if newName != "" {
				updated.Name = newName
			}
			if order != "" {
				ordered, err := reorderFolderItems(folder, order, folders, liveShoppingLists(userData))
				if err != nil {
					return err
				}
				updated.Items = ordered
			}
			if renameOnly {
				if err := al.RenameListFolder(ctx, listDataID, updated, newName); err != nil {
					return fmt.Errorf("renaming folder %q: %w", folder.GetName(), err)
				}
			} else if moveOnly {
				if err := al.MoveListFolderItems(ctx, listDataID, folder.GetIdentifier(), oldParent, newParent); err != nil {
					return fmt.Errorf("moving folder %q: %w", folder.GetName(), err)
				}
			} else if order != "" {
				if err := al.SetOrderedFolderItems(ctx, listDataID, folder, updated.GetItems()); err != nil {
					return fmt.Errorf("ordering folder %q: %w", folder.GetName(), err)
				}
			} else {
				if err := al.SaveListFolderWithParents(ctx, listDataID, updated, oldParent, newParent); err != nil {
					return fmt.Errorf("updating folder %q: %w", folder.GetName(), err)
				}
				if oldParent != newParent || order != "" {
					if err := saveParentMembership(ctx, al, listDataID, folders, oldParent, newParent, folder.GetIdentifier(), rootID); err != nil {
						return err
					}
				}
			}
			verifiedData, err := al.GetUserData(ctx)
			if err != nil {
				return fmt.Errorf("verifying folder update: %w", err)
			}
			verified, found := findLiveListFolderByID(verifiedData, folder.GetIdentifier())
			if !found {
				return fmt.Errorf("folder update verification failed: folder disappeared")
			}
			if newName != "" && verified.GetName() != newName {
				return fmt.Errorf("folder update verification failed: name read back as %q", verified.GetName())
			}
			if order != "" && !sameFolderItems(verified.GetItems(), updated.GetItems()) {
				return fmt.Errorf("folder update verification failed: child order did not read back")
			}
			verifiedMap := folderParentMap(verifiedData.GetListFoldersResponse().GetListFolders(), verifiedData.GetListFoldersResponse().GetRootFolderId())
			if oldParent != newParent && verifiedMap[folder.GetIdentifier()] != newParent {
				return fmt.Errorf("folder update verification failed: parent did not read back")
			}
			if err := st.SyncFromUserData(verifiedData); err != nil {
				return fmt.Errorf("updating local cache after folder update: %w", err)
			}
			if flags.quiet {
				return nil
			}
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"updated": true, "folder": verified.GetName(), "id": verified.GetIdentifier(), "parent": verifiedMap[folder.GetIdentifier()], "verified": true}, flags)
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "Current folder name")
	cmd.Flags().StringVar(&newName, "new-name", "", "New folder name")
	cmd.Flags().StringVar(&parent, "parent", "", "Parent folder name or ID; empty keeps the current parent")
	cmd.Flags().StringVar(&order, "order", "", "Comma-separated child folder/list names or IDs in desired order")
	cmd.Flags().StringVar(&color, "color", "", "Folder hex color (#RRGGBB)")
	cmd.Flags().StringVar(&sortPosition, "sort-position", "", "Folder sort position: FolderSortPositionAfterLists, FolderSortPositionBeforeLists, or FolderSortPositionWithLists")
	cmd.Flags().BoolVar(&stdinBody, "stdin", false, "Read request body as JSON from stdin")
	cmd.Flags().BoolVar(&apply, "apply", false, "Apply the change; preview is the default")
	return cmd
}

// folderMetadataLiveWritesEnabled is enabled only for the two folder metadata
// handlers with a disposable live round-trip recorded in
// .printing-press-patches/review-folder-metadata-handlers.json. Other folder
// mutations remain behind their own safety gates.
func folderMetadataLiveWritesEnabled() bool { return true }

func validFolderHexColor(value string) bool {
	if len(value) != 7 || value[0] != '#' {
		return false
	}
	for i := 1; i < 7; i++ {
		c := value[i]
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}

func folderSortPositionValue(value string) (pb.PBListFolderSettings_FolderSortPosition, bool) {
	switch value {
	case "FolderSortPositionAfterLists":
		return pb.PBListFolderSettings_FolderSortPositionAfterLists, true
	case "FolderSortPositionBeforeLists":
		return pb.PBListFolderSettings_FolderSortPositionBeforeLists, true
	case "FolderSortPositionWithLists":
		return pb.PBListFolderSettings_FolderSortPositionWithLists, true
	default:
		return 0, false
	}
}

func resolveFolderSelector(userData *pb.PBUserDataResponse, selector string) (*pb.PBListFolder, error) {
	selector = strings.TrimSpace(selector)
	if selector == "" {
		return nil, fmt.Errorf("parent folder must not be empty")
	}
	if folder, ok := findLiveListFolderByID(userData, selector); ok {
		return folder, nil
	}
	return findLiveListFolderForMutation(userData, selector)
}

func sameFolderItems(a, b []*pb.PBListFolderItem) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].GetIdentifier() != b[i].GetIdentifier() || a[i].GetItemType() != b[i].GetItemType() {
			return false
		}
	}
	return true
}

func folderParentMap(folders []*pb.PBListFolder, root string) map[string]string {
	m := map[string]string{}
	for _, f := range folders {
		for _, i := range f.GetItems() {
			if i.GetItemType() == int32(pb.PBListFolderItem_FolderType) {
				m[i.GetIdentifier()] = f.GetIdentifier()
			}
		}
	}
	for _, f := range folders {
		if _, ok := m[f.GetIdentifier()]; !ok {
			m[f.GetIdentifier()] = root
		}
	}
	return m
}
func folderIsDescendant(par map[string]string, candidate, ancestor string) bool {
	seen := map[string]bool{}
	for candidate != "" && !seen[candidate] {
		if candidate == ancestor {
			return true
		}
		seen[candidate] = true
		candidate = par[candidate]
	}
	return false
}

func reorderFolderItems(folder *pb.PBListFolder, raw string, folders []*pb.PBListFolder, lists []*pb.ShoppingList) ([]*pb.PBListFolderItem, error) {
	byName := map[string]string{}
	for _, f := range folders {
		byName[strings.ToLower(strings.TrimSpace(f.GetName()))] = f.GetIdentifier()
	}
	for _, l := range lists {
		byName[strings.ToLower(strings.TrimSpace(l.GetName()))] = l.GetIdentifier()
	}
	requested := strings.Split(raw, ",")
	seen := map[string]bool{}
	out := make([]*pb.PBListFolderItem, 0, len(folder.GetItems()))
	for _, token := range requested {
		token = strings.TrimSpace(token)
		if token == "" {
			continue
		}
		id := token
		if resolved, ok := byName[strings.ToLower(token)]; ok {
			id = resolved
		}
		found := false
		for _, item := range folder.GetItems() {
			if item.GetIdentifier() == id {
				if seen[id] {
					return nil, fmt.Errorf("order contains duplicate child %q", token)
				}
				seen[id] = true
				out = append(out, proto.Clone(item).(*pb.PBListFolderItem))
				found = true
				break
			}
		}
		if !found {
			return nil, fmt.Errorf("child %q is not in folder %q", token, folder.GetName())
		}
	}
	for _, item := range folder.GetItems() {
		if !seen[item.GetIdentifier()] {
			out = append(out, proto.Clone(item).(*pb.PBListFolderItem))
		}
	}
	return out, nil
}

func saveParentMembership(ctx context.Context, al *anylist.Client, listDataID string, folders []*pb.PBListFolder, oldParent, newParent, child, root string) error {
	if oldParent == newParent {
		return nil
	}
	parents := folderParentMap(folders, root)
	byID := map[string]*pb.PBListFolder{}
	for _, folder := range folders {
		byID[folder.GetIdentifier()] = folder
	}
	for _, parentID := range []string{oldParent, newParent} {
		if parentID == "" || parentID == root {
			continue
		}
		parent := byID[parentID]
		if parent == nil {
			return fmt.Errorf("parent folder %q not found while updating membership", parentID)
		}
		updated := proto.Clone(parent).(*pb.PBListFolder)
		items := make([]*pb.PBListFolderItem, 0, len(parent.GetItems())+1)
		for _, item := range parent.GetItems() {
			if item.GetIdentifier() != child {
				items = append(items, proto.Clone(item).(*pb.PBListFolderItem))
			}
		}
		if parentID == newParent {
			items = append(items, &pb.PBListFolderItem{Identifier: child, ItemType: int32(pb.PBListFolderItem_FolderType)})
		}
		updated.Items = items
		if err := al.SaveListFolderWithParents(ctx, listDataID, updated, parents[parentID], parentID); err != nil {
			return fmt.Errorf("updating parent folder %q: %w", parent.GetName(), err)
		}
	}
	return nil
}
