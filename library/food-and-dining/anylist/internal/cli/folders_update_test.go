package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/anylist/internal/anylist/pb"
)

func TestFoldersUpdatePreviewsByDefault(t *testing.T) {
	t.Parallel()
	cmd := newFoldersUpdateCmd(&rootFlags{})
	cmd.SetArgs([]string{"--name", "Dinners", "--new-name", "Weeknight Dinners"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("preview returned error: %v", err)
	}
}

func TestFoldersUpdateMetadataPreview(t *testing.T) {
	t.Parallel()
	cmd := newFoldersUpdateCmd(&rootFlags{})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--name", "Dinners", "--color", "#123456", "--sort-position", "FolderSortPositionBeforeLists"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("preview returned error: %v", err)
	}
	var preview map[string]any
	if err := json.Unmarshal(out.Bytes(), &preview); err != nil {
		t.Fatalf("preview output is not JSON: %v", err)
	}
	if preview["color"] != "#123456" || preview["color_handler"] != "set-folder-hex-color" {
		t.Fatalf("color preview = %#v", preview)
	}
	if preview["sort_position"] != "FolderSortPositionBeforeLists" || preview["sort_position_handler"] != "set-folder-sort-position" {
		t.Fatalf("sort-position preview = %#v", preview)
	}
}

func TestFoldersUpdateMetadataValidation(t *testing.T) {
	t.Parallel()
	for _, bad := range []string{"#12345", "123456", "#GGGGGG", "red"} {
		cmd := newFoldersUpdateCmd(&rootFlags{})
		cmd.SetArgs([]string{"--name", "Dinners", "--color", bad})
		if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "#RRGGBB") {
			t.Fatalf("color %q error = %v", bad, err)
		}
	}
	for _, bad := range []string{"clear", "BeforeLists", "2"} {
		cmd := newFoldersUpdateCmd(&rootFlags{})
		cmd.SetArgs([]string{"--name", "Dinners", "--sort-position", bad})
		if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "not supported") {
			t.Fatalf("sort position %q error = %v", bad, err)
		}
	}
	cmd := newFoldersUpdateCmd(&rootFlags{})
	cmd.SetArgs([]string{"--name", "Dinners"})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "nothing to update") {
		t.Fatalf("empty update error = %v", err)
	}
}

func TestFoldersUpdateMetadataApplyRequiresAuthentication(t *testing.T) {
	t.Parallel()
	flags := &rootFlags{configPath: t.TempDir() + "/config.toml"}
	cmd := newFoldersUpdateCmd(flags)
	cmd.SetArgs([]string{"--name", "Dinners", "--color", "#123456", "--apply"})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "not authenticated") {
		t.Fatalf("apply error = %v", err)
	}
}

func TestValidFolderHexColor(t *testing.T) {
	t.Parallel()
	for _, value := range []string{"#123456", "#ABCDEF", "#000000"} {
		if !validFolderHexColor(value) {
			t.Fatalf("validFolderHexColor(%q) = false", value)
		}
	}
	for _, value := range []string{"", "#12345", "123456", "#GGGGGG"} {
		if validFolderHexColor(value) {
			t.Fatalf("validFolderHexColor(%q) = true", value)
		}
	}
}

func TestFolderSortPositionValue(t *testing.T) {
	t.Parallel()
	for _, value := range []string{"FolderSortPositionAfterLists", "FolderSortPositionBeforeLists", "FolderSortPositionWithLists"} {
		if _, ok := folderSortPositionValue(value); !ok {
			t.Fatalf("folderSortPositionValue(%q) rejected", value)
		}
	}
	if _, ok := folderSortPositionValue("clear"); ok {
		t.Fatal("clear must not be accepted")
	}
}

func TestFolderIsDescendantRejectsCycles(t *testing.T) {
	t.Parallel()
	parents := map[string]string{"child": "parent", "grandchild": "child", "parent": "root"}
	if !folderIsDescendant(parents, "grandchild", "parent") {
		t.Fatal("grandchild should be a descendant of parent")
	}
	if folderIsDescendant(parents, "parent", "grandchild") {
		t.Fatal("parent must not be treated as a descendant of grandchild")
	}
}

func TestReorderFolderItemsPreservesOmittedChildren(t *testing.T) {
	t.Parallel()
	folder := &pb.PBListFolder{Identifier: "folder", Name: "Dinners", Items: []*pb.PBListFolderItem{
		{Identifier: "list-a", ItemType: int32(pb.PBListFolderItem_ListType)},
		{Identifier: "folder-b", ItemType: int32(pb.PBListFolderItem_FolderType)},
		{Identifier: "list-c", ItemType: int32(pb.PBListFolderItem_ListType)},
	}}
	ordered, err := reorderFolderItems(folder, "list-c,folder-b", []*pb.PBListFolder{{Identifier: "folder-b", Name: "Sides"}}, []*pb.ShoppingList{{Identifier: "list-a", Name: "Main"}, {Identifier: "list-c", Name: "Dessert"}})
	if err != nil {
		t.Fatalf("reorderFolderItems returned error: %v", err)
	}
	if got := []string{ordered[0].GetIdentifier(), ordered[1].GetIdentifier(), ordered[2].GetIdentifier()}; strings.Join(got, ",") != "list-c,folder-b,list-a" {
		t.Fatalf("order = %v", got)
	}
}

func TestReorderFolderItemsRejectsUnknownChild(t *testing.T) {
	t.Parallel()
	folder := &pb.PBListFolder{Identifier: "folder", Items: []*pb.PBListFolderItem{{Identifier: "list-a"}}}
	if _, err := reorderFolderItems(folder, "missing", nil, nil); err == nil {
		t.Fatal("unknown child was accepted")
	}
}
