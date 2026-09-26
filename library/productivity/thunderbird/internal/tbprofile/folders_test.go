package tbprofile

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/productivity/thunderbird/internal/tbprofile/tbtest"
)

func TestDiscoverFolders(t *testing.T) {
	_, profile := tbtest.Fixture(t)
	tests := []struct {
		name    string
		dir     string
		account string
		want    map[string]bool
	}{
		{
			name:    "imap server",
			dir:     filepath.Join(profile, "ImapMail", "imap.example.com"),
			account: "account1",
			want:    map[string]bool{"Archives": true, "Archives/2025": true, "INBOX": true, "Posta inviata": true, "Spam": false},
		},
		{
			name:    "local folders",
			dir:     filepath.Join(profile, "Mail", "Local Folders"),
			account: "account2",
			want:    map[string]bool{"Trash": false, "Unsent Messages": true},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, skipped, err := DiscoverFolders(tt.dir, tt.account, nil)
			if err != nil || len(skipped) != 0 {
				t.Fatal(err, skipped)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("got %d folders: %+v", len(got), got)
			}
			for _, f := range got {
				offline, ok := tt.want[f.Path]
				if !ok {
					t.Errorf("unexpected folder %q", f.Path)
					continue
				}
				if f.Offline != offline || f.Account != tt.account {
					t.Errorf("%s offline=%v account=%s", f.Path, f.Offline, f.Account)
				}
				if !offline && (f.MboxPath != "" || f.SizeBytes != 0) {
					t.Errorf("msf-only %s has mbox %q size %d", f.Path, f.MboxPath, f.SizeBytes)
				}
				if f.Path == "Archives/2025" && (f.Name != "2025" || f.MboxPath != filepath.Join(tt.dir, "Archives.sbd", "2025")) {
					t.Errorf("subfolder = %+v", f)
				}
				if f.Path == "INBOX" && f.SizeBytes == 0 {
					t.Errorf("INBOX size 0")
				}
			}
		})
	}
	none, _, err := DiscoverFolders(filepath.Join(profile, "nope"), "x", nil)
	if err != nil || len(none) != 0 {
		t.Fatalf("missing dir: %v %v", none, err)
	}
}

func TestDiscoverFoldersUnreadableSubtree(t *testing.T) {
	_, profile := tbtest.Fixture(t)
	dir := filepath.Join(profile, "ImapMail", "imap.example.com")
	deny := func(name string) ([]os.DirEntry, error) {
		if filepath.Base(name) == "Archives.sbd" {
			return nil, errors.New("access denied")
		}
		return os.ReadDir(name)
	}
	got, skipped, err := DiscoverFolders(dir, "account1", deny)
	if err != nil {
		t.Fatal(err)
	}
	if len(skipped) != 1 || skipped[0].Prefix != "Archives" {
		t.Fatalf("skipped = %+v", skipped)
	}
	paths := map[string]bool{}
	for _, f := range got {
		paths[f.Path] = true
	}
	if !paths["INBOX"] || !paths["Archives"] || paths["Archives/2025"] {
		t.Fatalf("folders = %v", paths)
	}
}
