// Article operation hash resolution.
//
// X Articles GraphQL endpoints carry rotating operation hashes in the URL path:
//   /i/api/graphql/<hash>/<OperationName>
//
// The hash changes when X redeploys their web app. Treating it as a compile-time
// constant means every printed CLI breaks the moment X redeploys. This file
// reads hashes from ~/.config/x-twitter-pp-cli/article-ops.json at runtime, so the user
// can re-sniff and update the file without rebuilding the CLI.
//
// Falls back to compile-time defaults captured at generation time, so the CLI
// works out of the box until the next X redeploy.

package client

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

type articleOpsFile struct {
	Operations map[string]string `json:"operations"`
	CapturedAt string            `json:"captured_at"`
}

// articleOpsDefaults are the hashes captured at generation time (2026-05-08).
// Used as fallback when ~/.config/x-twitter-pp-cli/article-ops.json is missing or
// missing a key. Re-capture via browser-sniff and overwrite the config file
// when these stop working.
var articleOpsDefaults = map[string]string{
	"ArticleEntitiesSlice":          "N1zzFzRPspT-sP9Q42n_bg",
	"ArticleEntityDraftCreate":      "g1l5N8BxGewYuCy5USe_bQ",
	"ArticleEntityUpdateTitle":      "x75E2ABzm8_mGTg1bz8hcA",
	"ArticleEntityUpdateContent":    "M7N2FrPrlOmu-YrVIBxFnQ",
	"ArticleEntityUpdateCoverMedia": "Es8InPh7mEkK9PxclxFAVQ",
	"ArticleEntityPublish":          "m4SHicYMoWO_qkLvjhDk7Q",
	"ArticleEntityUnpublish":        "WbeMAOZdMHilHrqhgpjObw",
	"ArticleEntityDelete":           "e4lWqB6m2TA8Fn_j9L9xEA",
}

var (
	articleOpsOnce sync.Once
	articleOpsMap  map[string]string
)

func loadArticleOps() {
	articleOpsMap = make(map[string]string, len(articleOpsDefaults))
	for k, v := range articleOpsDefaults {
		articleOpsMap[k] = v
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	path := filepath.Join(home, ".config", "x-twitter-pp-cli", "article-ops.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return // fall back to defaults
	}
	var f articleOpsFile
	if err := json.Unmarshal(data, &f); err != nil {
		return
	}
	for op, hash := range f.Operations {
		if hash != "" {
			articleOpsMap[op] = hash
		}
	}
}

// ArticleOpURL returns the absolute URL for a Draft.js Articles GraphQL
// operation using the latest known hash. Returns an absolute URL on
// https://x.com because Articles endpoints do NOT live on the api.x.com host
// the OAuth-based v2 surface uses; the client recognises absolute URLs and
// bypasses BaseURL. Reads ~/.config/x-twitter-pp-cli/article-ops.json on first call;
// falls back to defaults captured at generation time. If the operation is
// unknown, returns a URL with an empty hash, which X will reject with a clear
// "no such operation" error.
func ArticleOpURL(opName string) string {
	articleOpsOnce.Do(loadArticleOps)
	hash := articleOpsMap[opName]
	return fmt.Sprintf("https://x.com/i/api/graphql/%s/%s", hash, opName)
}

// PATCH: ArticleOpQueryID returns the bare operation hash for use as the
// GraphQL request body's queryId field. Same runtime resolution as
// ArticleOpURL (config-file override, then generation-time defaults), so
// command code never hardcodes hash literals that drift when X redeploys.
func ArticleOpQueryID(opName string) string {
	articleOpsOnce.Do(loadArticleOps)
	return articleOpsMap[opName]
}

// MediaUploadURL returns the absolute URL for the chunked media upload
// endpoint, which lives on upload.x.com (different host from both api.x.com
// and x.com). Used by articles_upload_media.go.
func MediaUploadURL() string {
	return "https://upload.x.com/i/media/upload.json"
}
