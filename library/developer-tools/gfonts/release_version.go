package gfonts

import (
	_ "embed"
	"encoding/json"
)

//go:embed .printing-press-release.json
var releaseManifest []byte

// ReleaseVersion uses the catalog ledger that post-merge automation stamps.
// Source builds therefore report the current published version without a
// second hand-maintained version number.
func ReleaseVersion() string {
	var manifest struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(releaseManifest, &manifest); err != nil || manifest.Version == "" {
		return "0.0.0-dev"
	}
	return manifest.Version
}
