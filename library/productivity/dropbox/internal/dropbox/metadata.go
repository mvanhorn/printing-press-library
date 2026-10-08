package dropbox

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type Entry struct {
	Tag                  string    `json:".tag"`
	ID                   string    `json:"id,omitempty"`
	Name                 string    `json:"name,omitempty"`
	PathLower            string    `json:"path_lower,omitempty"`
	PathDisplay          string    `json:"path_display,omitempty"`
	ParentLower          string    `json:"-"`
	Rev                  string    `json:"rev,omitempty"`
	Size                 int64     `json:"size,omitempty"`
	ContentHash          string    `json:"content_hash,omitempty"`
	ClientModified       time.Time `json:"client_modified,omitempty"`
	ServerModified       time.Time `json:"server_modified,omitempty"`
	SharedFolderID       string    `json:"-"`
	ParentSharedFolderID string    `json:"-"`
	IsDownloadable       bool      `json:"is_downloadable,omitempty"`
}

func ParseEntries(raw json.RawMessage) ([]Entry, error) {
	var payload struct {
		Entries []struct {
			Entry
			SharingInfo struct {
				SharedFolderID       string `json:"shared_folder_id"`
				ParentSharedFolderID string `json:"parent_shared_folder_id"`
			} `json:"sharing_info"`
		} `json:"entries"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, fmt.Errorf("parse Dropbox entries: %w", err)
	}
	entries := make([]Entry, 0, len(payload.Entries))
	for _, item := range payload.Entries {
		e := item.Entry
		e.PathLower = strings.ToLower(e.PathLower)
		if i := strings.LastIndex(e.PathLower, "/"); i > 0 {
			e.ParentLower = e.PathLower[:i]
		}
		e.SharedFolderID = item.SharingInfo.SharedFolderID
		e.ParentSharedFolderID = item.SharingInfo.ParentSharedFolderID
		entries = append(entries, e)
	}
	return entries, nil
}
