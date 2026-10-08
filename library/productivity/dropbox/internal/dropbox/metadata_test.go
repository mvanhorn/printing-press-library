package dropbox

import (
	"encoding/json"
	"testing"
)

func TestParseEntries(t *testing.T) {
	raw := json.RawMessage(`{"entries":[{".tag":"file","path_lower":"/photos/2019/A.JPG","name":"A.JPG","size":7,"client_modified":"2020-01-02T03:04:05Z","sharing_info":{"parent_shared_folder_id":"sf"}},{".tag":"folder","path_lower":"/Photos","sharing_info":{"shared_folder_id":"folder"}},{".tag":"deleted","path_lower":"/old"}]}`)
	entries, err := ParseEntries(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 3 || entries[0].PathLower != "/photos/2019/a.jpg" || entries[0].ParentLower != "/photos/2019" || entries[0].ParentSharedFolderID != "sf" || entries[0].ClientModified.IsZero() {
		t.Fatalf("file = %+v", entries[0])
	}
	if entries[1].ParentLower != "" || entries[1].SharedFolderID != "folder" {
		t.Fatalf("root child = %+v", entries[1])
	}
}
