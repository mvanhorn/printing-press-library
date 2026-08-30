package cli

import (
	"compress/gzip"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/anylist/internal/anylist/pb"
)

func TestMarshalAnyListExportUsesProtoFieldNames(t *testing.T) {
	data, err := marshalAnyListExport(&pb.PBUserDataResponse{
		ShoppingListsResponse: &pb.ShoppingListsResponse{
			NewLists: []*pb.ShoppingList{{Identifier: "list-1", Name: "Groceries"}},
		},
	})
	if err != nil {
		t.Fatalf("marshalAnyListExport returned error: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("export is not JSON: %v", err)
	}
	if !strings.Contains(string(data), "shoppingListsResponse") || !strings.Contains(string(data), "list-1") {
		t.Fatalf("export = %s", data)
	}
}

func TestWriteAnyListExportGzip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "backup.json.gz")
	if err := writeAnyListExport(path, []byte(`{"ok":true}`), true); err != nil {
		t.Fatalf("writeAnyListExport returned error: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat returned error: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("backup mode = %o, want 600", info.Mode().Perm())
	}
	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	reader, err := gzip.NewReader(file)
	if err != nil {
		file.Close()
		t.Fatalf("NewReader returned error: %v", err)
	}
	data, err := io.ReadAll(reader)
	reader.Close()
	file.Close()
	if err != nil {
		t.Fatalf("ReadAll returned error: %v", err)
	}
	if string(data) != `{"ok":true}` {
		t.Fatalf("decompressed export = %q", data)
	}
}
