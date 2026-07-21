package cli

import (
	"archive/zip"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCollectFolderFiltersJunkHiddenAndNonMedia(t *testing.T) {
	dir := t.TempDir()
	for name := range map[string]string{"photo.jpg": "x", "note.txt": "x", ".hidden.jpg": "x", "._photo.jpg": "x", "skip.png": "x"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	assets, skipped, err := collectFolder(dir, importOptions{recursive: true, ignores: []string{"skip.*"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(assets) != 1 || assets[0].Name != "photo.jpg" {
		t.Fatalf("assets = %#v, want only photo.jpg", assets)
	}
	if skipped != 4 {
		t.Fatalf("skipped = %d, want 4", skipped)
	}
}

func TestWatchReadyAssetsAreExactlyOnceUntilStampChanges(t *testing.T) {
	path := filepath.Join(t.TempDir(), "watch.jpg")
	if err := os.WriteFile(path, []byte("one"), 0600); err != nil {
		t.Fatal(err)
	}
	start := time.Date(2026, 7, 21, 12, 0, 0, 0, time.UTC)
	asset := importAsset{Path: path, Name: "watch.jpg", Modified: start}
	seen := map[string]fileStamp{}
	if ready := readyWatchAssets([]importAsset{asset}, seen, start, 5*time.Second); len(ready) != 0 {
		t.Fatalf("initial observation ready = %#v", ready)
	}
	ready := readyWatchAssets([]importAsset{asset}, seen, start.Add(5*time.Second), 5*time.Second)
	if len(ready) != 1 || ready[0].Path != path {
		t.Fatalf("stable asset ready = %#v", ready)
	}
	markWatchAssetsUploaded(ready, seen)
	if ready := readyWatchAssets([]importAsset{asset}, seen, start.Add(20*time.Second), 5*time.Second); len(ready) != 0 {
		t.Fatalf("unchanged uploaded asset repeated = %#v", ready)
	}
	if err := os.WriteFile(path, []byte("changed-size"), 0600); err != nil {
		t.Fatal(err)
	}
	changed := asset
	changed.Modified = start.Add(21 * time.Second)
	if ready := readyWatchAssets([]importAsset{changed}, seen, changed.Modified, 5*time.Second); len(ready) != 0 {
		t.Fatalf("changed stamp bypassed stability window = %#v", ready)
	}
	ready = readyWatchAssets([]importAsset{changed}, seen, changed.Modified.Add(5*time.Second), 5*time.Second)
	if len(ready) != 1 || ready[0].Path != path {
		t.Fatalf("changed asset was not reprocessed once = %#v", ready)
	}
	markWatchAssetsUploaded(ready, seen)
	if ready := readyWatchAssets([]importAsset{changed}, seen, changed.Modified.Add(20*time.Second), 5*time.Second); len(ready) != 0 {
		t.Fatalf("changed uploaded asset repeated = %#v", ready)
	}
}

func TestPeopleJulyResolvesAndQueriesEachJuly(t *testing.T) {
	withTempLearnHome(t)
	calls := map[string]int{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/api")
		calls[path]++
		if path == "/search/person" {
			if r.Method != http.MethodGet || r.URL.Query().Get("name") != "Alice" || len(r.URL.Query()) != 1 {
				t.Fatalf("person search = %s %s", r.Method, r.URL.RawQuery)
			}
			_, _ = w.Write([]byte(`[{"id":"person-1"}]`))
			return
		}
		if path == "/search/metadata" {
			_, _ = w.Write([]byte(`{"assets":{"items":[{"id":"asset-1","originalFileName":"x.jpg"}]}}`))
			return
		}
		w.WriteHeader(404)
	}))
	defer server.Close()
	t.Setenv("IMMICH_BASE_URL", server.URL)
	t.Setenv("IMMICH_BASE_PATH", "")
	t.Setenv("IMMICH_API_KEY", "key")
	_, _, err := runRootArgs(t, "people", "july", "--person", "Alice", "--years", "2", "--json", "--no-learn")
	if err != nil {
		t.Fatal(err)
	}
	if calls["/search/person"] != 1 || calls["/search/metadata"] != 2 {
		t.Fatalf("calls=%v", calls)
	}
}

func TestAlbumEventPreviewDoesNotMutateAndApplyUsesAssetItemsOnly(t *testing.T) {
	withTempLearnHome(t)
	var posts []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/api")
		if r.Method == http.MethodPost {
			posts = append(posts, path)
		}
		if path == "/search/metadata" {
			_, _ = w.Write([]byte(`{"assets":{"items":[{"id":"asset-1","originalFileName":"x.jpg"}]},"owner":{"id":"not-an-asset"}}`))
			return
		}
		if path == "/albums" {
			_, _ = w.Write([]byte(`{"id":"album-1"}`))
			return
		}
		w.WriteHeader(404)
	}))
	defer server.Close()
	t.Setenv("IMMICH_BASE_URL", server.URL)
	t.Setenv("IMMICH_BASE_PATH", "")
	t.Setenv("IMMICH_API_KEY", "key")
	out, _, err := runRootArgs(t, "album", "event", "beach", "--from", "2025-07-01", "--to", "2025-07-07", "--json", "--no-learn")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(strings.Join(posts, ","), "/albums") {
		t.Fatalf("preview mutated: %v", posts)
	}
	if strings.Contains(out, "not-an-asset") {
		t.Fatalf("non-asset id leaked into preview: %s", out)
	}
	_, _, err = runRootArgs(t, "album", "event", "beach", "--from", "2025-07-01", "--to", "2025-07-07", "--apply", "--json", "--no-learn")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.Join(posts, ","), "/albums") {
		t.Fatalf("apply did not create album: %v", posts)
	}
}

func TestDuplicateApplyRefetchesAndRejectsStalePlan(t *testing.T) {
	withTempLearnHome(t)
	posts := 0
	stale := true
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/api")
		if path == "/duplicates" && r.Method == http.MethodGet {
			if r.URL.RawQuery != "" {
				t.Fatalf("duplicates sent unsupported query %q", r.URL.RawQuery)
			}
			if stale {
				_, _ = w.Write([]byte(`[{"duplicateId":"g","assets":[{"id":"a"},{"id":"c"}],"suggestedKeepAssetIds":["a"]}]`))
			} else {
				_, _ = w.Write([]byte(`[{"duplicateId":"g","assets":[{"id":"a"},{"id":"b"}],"suggestedKeepAssetIds":["a"]}]`))
			}
			return
		}
		if path == "/duplicates/resolve" && r.Method == http.MethodPost {
			posts++
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			groups := body["groups"].([]any)
			if len(groups) != 1 {
				t.Fatal("missing resolved group")
			}
			group := groups[0].(map[string]any)
			if group["duplicateId"] != "g" || len(group) != 3 || !sameStrings(stringValues(group["keepAssetIds"]), []string{"a"}) || !sameStrings(stringValues(group["trashAssetIds"]), []string{"b"}) {
				t.Fatalf("resolve payload = %#v", group)
			}
			_, _ = w.Write([]byte(`{}`))
			return
		}
		w.WriteHeader(404)
	}))
	defer server.Close()
	t.Setenv("IMMICH_BASE_URL", server.URL)
	t.Setenv("IMMICH_API_KEY", "key")
	args := []string{"duplicates", "apply", "--groups", `[{"group_id":"g","keeper":"a","trash":["b"]}]`, "--apply", "--json", "--no-learn", "--no-cache"}
	if _, _, err := runRootArgs(t, args...); err == nil {
		t.Fatal("stale plan accepted")
	}
	if posts != 0 {
		t.Fatalf("stale plan posted %d mutations", posts)
	}
	stale = false
	if _, _, err := runRootArgs(t, args...); err != nil {
		t.Fatal(err)
	}
	if posts != 1 {
		t.Fatalf("valid plan posts=%d", posts)
	}
}

func TestDuplicatePlansPreserveAllSuggestedKeepAssets(t *testing.T) {
	plans, err := duplicatePlans([]byte(`[{"duplicateId":"g","assets":[{"id":"a"},{"id":"b"},{"id":"c"}],"suggestedKeepAssetIds":["b","a"]}]`))
	if err != nil || len(plans) != 1 {
		t.Fatalf("duplicatePlans = %#v, %v", plans, err)
	}
	if !sameStrings(plans[0].Keep, []string{"b", "a"}) || plans[0].Keeper != "b" || !sameStrings(plans[0].Trash, []string{"c"}) {
		t.Fatalf("plan = %#v", plans[0])
	}
}

func TestDuplicatePlansRequireExplicitKeeperWhenServerDoesNotSuggestOne(t *testing.T) {
	plans, err := duplicatePlans([]byte(`[{"duplicateId":"g","assets":[{"id":"a"},{"id":"b"}]}]`))
	if err != nil || len(plans) != 1 {
		t.Fatalf("duplicatePlans = %#v, %v", plans, err)
	}
	if !plans[0].KeeperRequired || plans[0].Keeper != "" || len(plans[0].Trash) != 0 || !sameStrings(plans[0].Evidence, []string{"a", "b"}) {
		t.Fatalf("unsuggested group must require an explicit keeper: %#v", plans[0])
	}
}

func TestDuplicateApplyRequiresExplicitKeeperWhenServerDoesNotSuggestOne(t *testing.T) {
	withTempLearnHome(t)
	resolves := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/api")
		switch path {
		case "/duplicates":
			_, _ = w.Write([]byte(`[{"duplicateId":"g","assets":[{"id":"a"},{"id":"b"}]}]`))
		case "/duplicates/resolve":
			resolves++
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			groups := body["groups"].([]any)
			group := groups[0].(map[string]any)
			if !sameStrings(stringValues(group["keepAssetIds"]), []string{"b"}) || !sameStrings(stringValues(group["trashAssetIds"]), []string{"a"}) {
				t.Fatalf("explicit keeper payload = %#v", group)
			}
			_, _ = w.Write([]byte(`{}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	t.Setenv("IMMICH_BASE_URL", server.URL)
	t.Setenv("IMMICH_API_KEY", "key")
	if _, _, err := runRootArgs(t, "duplicates", "apply", "--groups", `[{"group_id":"g"}]`, "--apply", "--json", "--no-learn", "--no-cache"); err == nil {
		t.Fatal("apply accepted a group without an explicit keeper")
	}
	if resolves != 0 {
		t.Fatalf("unexpected resolve without keeper: %d", resolves)
	}
	if _, _, err := runRootArgs(t, "duplicates", "apply", "--groups", `[{"group_id":"g","keeper":"b"}]`, "--apply", "--json", "--no-learn", "--no-cache"); err != nil {
		t.Fatal(err)
	}
	if resolves != 1 {
		t.Fatalf("resolves=%d, want 1", resolves)
	}
}

func TestStacksReviewFetchesDetailsAndClassifies(t *testing.T) {
	withTempLearnHome(t)
	gets := map[string]int{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/api")
		gets[path]++
		if path == "/stacks" {
			if r.URL.RawQuery != "" {
				t.Fatalf("stacks sent unsupported query %q", r.URL.RawQuery)
			}
			_, _ = w.Write([]byte(`[{"id":"empty"},{"id":"single"},{"id":"large"}]`))
			return
		}
		if path == "/stacks/empty" {
			_, _ = w.Write([]byte(`{"assets":[]}`))
			return
		}
		if path == "/stacks/single" {
			_, _ = w.Write([]byte(`{"assets":[{}]}`))
			return
		}
		if path == "/stacks/large" {
			_, _ = w.Write([]byte(`{"assets":[{},{},{},{},{},{},{},{},{},{},{},{},{},{},{},{},{},{},{},{}]}`))
			return
		}
		w.WriteHeader(404)
	}))
	defer server.Close()
	t.Setenv("IMMICH_BASE_URL", server.URL)
	t.Setenv("IMMICH_API_KEY", "key")
	out, _, err := runRootArgs(t, "stacks", "review", "--limit", "3", "--json", "--no-learn")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"empty", "singleton", "large"} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %s in %s", want, out)
		}
	}
	for _, path := range []string{"/stacks/empty", "/stacks/single", "/stacks/large"} {
		if gets[path] != 1 {
			t.Fatalf("%s calls=%d", path, gets[path])
		}
	}
}

func TestNovelRoutesUseArchivedOpenAPIWireShapes(t *testing.T) {
	withTempLearnHome(t)
	var memoryQuery url.Values
	var favoriteBody, archivedBody, shareBody map[string]any
	partnerDirections := []string{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/api")
		switch path {
		case "/memories":
			memoryQuery = r.URL.Query()
			_, _ = w.Write([]byte(`[]`))
		case "/memories/statistics":
			_, _ = w.Write([]byte(`{}`))
		case "/search/metadata":
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			if body["isFavorite"] == true {
				favoriteBody = body
			} else {
				archivedBody = body
			}
			_, _ = w.Write([]byte(`{"assets":{"items":[]}}`))
		case "/albums":
			_, _ = w.Write([]byte(`{"id":"album-1"}`))
		case "/albums/album-1/users":
			if r.Method != http.MethodPut {
				t.Fatalf("album users method = %s", r.Method)
			}
			_ = json.NewDecoder(r.Body).Decode(&shareBody)
			_, _ = w.Write([]byte(`{}`))
		case "/partners":
			partnerDirections = append(partnerDirections, r.URL.Query().Get("direction"))
			_, _ = w.Write([]byte(`[]`))
		case "/jobs", "/queues":
			_, _ = w.Write([]byte(`[]`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	t.Setenv("IMMICH_BASE_URL", server.URL)
	t.Setenv("IMMICH_BASE_PATH", "")
	t.Setenv("IMMICH_API_KEY", "key")
	if _, _, err := runRootArgs(t, "memories", "review", "--days", "7", "--limit", "3", "--json", "--no-learn"); err != nil {
		t.Fatal(err)
	}
	if memoryQuery.Get("for") == "" || memoryQuery.Get("size") != "3" || len(memoryQuery) != 2 {
		t.Fatalf("memories query = %v", memoryQuery)
	}
	if _, _, err := runRootArgs(t, "library", "review", "--favorites", "--limit", "2", "--json", "--no-learn"); err != nil {
		t.Fatal(err)
	}
	if favoriteBody["isFavorite"] != true || favoriteBody["isArchived"] != nil || favoriteBody["visibility"] != nil || favoriteBody["size"] != float64(2) || len(favoriteBody) != 2 {
		t.Fatalf("favorite search body = %#v", favoriteBody)
	}
	if _, _, err := runRootArgs(t, "library", "review", "--archived", "--limit", "2", "--json", "--no-learn"); err != nil {
		t.Fatal(err)
	}
	if archivedBody["visibility"] != "archive" || archivedBody["isArchived"] != nil || archivedBody["isFavorite"] != nil || archivedBody["size"] != float64(2) || len(archivedBody) != 2 {
		t.Fatalf("archived search body = %#v", archivedBody)
	}
	if _, _, err := runRootArgs(t, "album", "event", "beach", "--from", "2025-07-01", "--to", "2025-07-07", "--share-with", "user-1", "--apply", "--json", "--no-learn"); err != nil {
		t.Fatal(err)
	}
	users := shareBody["albumUsers"].([]any)
	if len(shareBody) != 1 || len(users) != 1 || users[0].(map[string]any)["userId"] != "user-1" {
		t.Fatalf("album users body = %#v", shareBody)
	}
	if _, _, err := runRootArgs(t, "library", "health", "--json", "--no-learn"); err != nil {
		t.Fatal(err)
	}
	if !sameStrings(partnerDirections, []string{"shared-by", "shared-with"}) {
		t.Fatalf("partner directions = %v", partnerDirections)
	}
}

func TestBulkUploadCheckDecodesActions(t *testing.T) {
	for raw, want := range map[string]bool{`{"results":[{"id":"a","action":"accept"}]}`: false, `{"results":[{"id":"a","action":"duplicate"}]}`: true} {
		got, err := duplicateCheck([]byte(raw))
		if err != nil || got != want {
			t.Fatalf("duplicateCheck(%s) = %v,%v", raw, got, err)
		}
	}
	if _, err := duplicateCheck([]byte(`{"results":[{"id":"a","action":"mystery"}]}`)); err == nil {
		t.Fatal("unknown action should fail")
	}
}

func TestUploadOneUsesChecksumAndExactMultipartFields(t *testing.T) {
	withTempLearnHome(t)
	file := filepath.Join(t.TempDir(), "photo.jpg")
	if err := os.WriteFile(file, []byte("photo-bytes"), 0600); err != nil {
		t.Fatal(err)
	}
	var check map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/api")
		switch path {
		case "/assets/bulk-upload-check":
			_ = json.NewDecoder(r.Body).Decode(&check)
			_, _ = w.Write([]byte(`{"results":[{"id":"ignored","action":"accept"}]}`))
		case "/assets":
			if err := r.ParseMultipartForm(1 << 20); err != nil {
				t.Fatal(err)
			}
			if r.FormValue("filename") != "photo.jpg" || r.FormValue("fileCreatedAt") == "" || r.FormValue("fileModifiedAt") == "" || r.FormValue("deviceId") != "" || r.FormValue("deviceAssetId") != "" || r.FormValue("fileName") != "" {
				t.Fatalf("multipart form = %#v", r.MultipartForm.Value)
			}
			if _, _, err := r.FormFile("assetData"); err != nil {
				t.Fatalf("assetData missing: %v", err)
			}
			_, _ = w.Write([]byte(`{"id":"destination-asset","status":"created"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	t.Setenv("IMMICH_BASE_URL", server.URL)
	t.Setenv("IMMICH_BASE_PATH", "")
	t.Setenv("IMMICH_API_KEY", "key")
	c, err := (&rootFlags{rateLimit: 0}).newClient()
	if err != nil {
		t.Fatal(err)
	}
	created := time.Date(2025, 7, 1, 1, 2, 3, 0, time.UTC)
	duplicate, id, _, _, err := uploadOne(context.Background(), c, importAsset{Path: file, Name: "photo.jpg", Created: created, Modified: created})
	if err != nil || duplicate || id != "destination-asset" {
		t.Fatalf("uploadOne = duplicate:%v id:%q err:%v", duplicate, id, err)
	}
	assets := check["assets"].([]any)
	if len(assets) != 1 {
		t.Fatalf("checksum check = %#v", check)
	}
	item := assets[0].(map[string]any)
	if len(item) != 2 || item["id"] != item["checksum"] || item["checksum"] != "bb1f3308adcc035cb700962e4004e5e85c3cd006" {
		t.Fatalf("checksum check item = %#v", item)
	}
}

func TestUploadAssetsRetainsCommittedIDWhenMetadataUpdateFails(t *testing.T) {
	withTempLearnHome(t)
	file := filepath.Join(t.TempDir(), "photo.jpg")
	if err := os.WriteFile(file, []byte("photo-bytes"), 0600); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch strings.TrimPrefix(r.URL.Path, "/api") {
		case "/assets/bulk-upload-check":
			_, _ = w.Write([]byte(`{"results":[{"id":"ignored","action":"accept"}]}`))
		case "/assets":
			if r.Method == http.MethodPost {
				_, _ = w.Write([]byte(`{"id":"destination-asset"}`))
				return
			}
			w.WriteHeader(http.StatusInternalServerError)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	t.Setenv("IMMICH_BASE_URL", server.URL)
	t.Setenv("IMMICH_BASE_PATH", "")
	t.Setenv("IMMICH_API_KEY", "key")
	sum, err := uploadAssets(context.Background(), &rootFlags{rateLimit: 0}, []importAsset{{Path: file, Name: "photo.jpg", Created: time.Now(), Modified: time.Now(), Metadata: map[string]any{"source_asset_id": "source-asset", "description": "keep me"}}}, importOptions{concurrency: 1})
	if err != nil {
		t.Fatal(err)
	}
	if sum.Uploaded != 1 || sum.Failed != 0 || sum.AssetMapping["source-asset"] != "destination-asset" || len(sum.Warnings) != 1 {
		t.Fatalf("committed upload was not retained: %#v", sum)
	}
}

func TestArchiveRejectsTraversal(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.zip")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	w := zip.NewWriter(f)
	e, err := w.Create("../escape.jpg")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = io.WriteString(e, "x")
	_ = w.Close()
	_ = f.Close()
	if _, err := unzipToTemp(path); err == nil {
		t.Fatal("zip traversal accepted")
	}
}

func TestTakeoutSidecarsUseRelativePath(t *testing.T) {
	dir := t.TempDir()
	for _, rel := range []string{"one/same.jpg", "two/same.jpg"} {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(dir, rel)), 0750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, rel), []byte("x"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "one/same.jpg.json"), []byte(`{"description":"one"}`), 0600); err != nil {
		t.Fatal(err)
	}
	assets, _, err := collectFolder(dir, importOptions{recursive: true})
	if err != nil {
		t.Fatal(err)
	}
	applyTakeoutSidecars(dir, assets)
	found := 0
	for _, a := range assets {
		if a.Metadata["description"] == "one" {
			found++
			if a.Metadata["source_relative_path"] != "one/same.jpg" {
				t.Fatal("sidecar bound wrong duplicate basename")
			}
		}
	}
	if found != 1 {
		t.Fatalf("found %d mapped assets", found)
	}
}

func TestMapSourceCollectionsCreatesTagsWithName(t *testing.T) {
	withTempLearnHome(t)
	var createdTag map[string]any
	destination := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/api")
		switch path {
		case "/tags":
			if r.Method != http.MethodPost {
				t.Fatalf("tag method = %s", r.Method)
			}
			_ = json.NewDecoder(r.Body).Decode(&createdTag)
			_, _ = w.Write([]byte(`{"id":"tag-1"}`))
		case "/tags/tag-1/assets":
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			if !sameStrings(stringValues(body["ids"]), []string{"destination-asset"}) {
				t.Fatalf("tag assets body = %#v", body)
			}
			_, _ = w.Write([]byte(`[]`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer destination.Close()
	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/albums":
			_, _ = w.Write([]byte(`[]`))
		case "/tags":
			_, _ = w.Write([]byte(`[{"name":"vacation","assets":[{"id":"source-asset"}]}]`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer source.Close()
	t.Setenv("IMMICH_BASE_URL", destination.URL)
	t.Setenv("IMMICH_BASE_PATH", "")
	t.Setenv("IMMICH_API_KEY", "key")
	if err := mapSourceCollections(context.Background(), &rootFlags{}, source.URL, "source-key", map[string]string{"source-asset": "destination-asset"}); err != nil {
		t.Fatal(err)
	}
	if len(createdTag) != 1 || createdTag["name"] != "vacation" {
		t.Fatalf("tag create body = %#v", createdTag)
	}
}

func TestMapSourceCollectionsRejectsMalformedAlbumsAndTags(t *testing.T) {
	for name, body := range map[string]string{"albums": `{`, "tags": `[`} {
		t.Run(name, func(t *testing.T) {
			source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/albums":
					if name == "albums" {
						_, _ = w.Write([]byte(body))
						return
					}
					_, _ = w.Write([]byte(`[]`))
				case "/tags":
					_, _ = w.Write([]byte(body))
				default:
					w.WriteHeader(http.StatusNotFound)
				}
			}))
			defer source.Close()
			err := mapSourceCollections(context.Background(), &rootFlags{}, source.URL, "source-key", map[string]string{"source": "destination"})
			if err == nil || !strings.Contains(err.Error(), "decode source "+name) {
				t.Fatalf("malformed %s error = %v", name, err)
			}
		})
	}
}

func TestSourceImmichPaginatesAndUsesCollisionSafeTempFiles(t *testing.T) {
	var pages []int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && r.URL.Path == "/search/metadata" {
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			pages = append(pages, int(body["page"].(float64)))
			if body["page"].(float64) == 1 {
				_, _ = w.Write([]byte(`{"assets":{"items":[{"id":"a","originalFileName":"same.jpg"},{"id":"b","originalFileName":"same.jpg"}]}}`))
			} else {
				_, _ = w.Write([]byte(`{"assets":{"items":[]}}`))
			}
			return
		}
		if r.Method == http.MethodGet {
			_, _ = w.Write([]byte("image"))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()
	assets, _, err := fetchSourceImmich(context.Background(), server.URL, "key", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(assets) != 2 || assets[0].Path == assets[1].Path {
		t.Fatalf("assets paths not collision-safe: %#v", assets)
	}
	if len(pages) != 2 {
		t.Fatalf("pages=%v", pages)
	}
	dir := assets[0].Metadata["source_temp_dir"].(string)
	_ = os.RemoveAll(dir)
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatal("temp cleanup failed")
	}
}

func TestImportImmichCleansTemporaryDownloadDirectory(t *testing.T) {
	withTempLearnHome(t)
	tmpRoot := t.TempDir()
	t.Setenv("TMPDIR", tmpRoot)
	sourceRequests := 0
	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sourceRequests++
		switch r.URL.Path {
		case "/search/metadata":
			if r.Method != http.MethodPost || r.Header.Get("x-api-key") != "source-key" {
				t.Fatalf("source search request = %s api-key=%q", r.Method, r.Header.Get("x-api-key"))
			}
			_, _ = w.Write([]byte(`{"assets":{"items":[{"id":"source-asset","originalFileName":"source.jpg"}]}}`))
		case "/assets/source-asset/original":
			if r.Method != http.MethodGet || r.Header.Get("x-api-key") != "source-key" {
				t.Fatalf("source original request = %s api-key=%q", r.Method, r.Header.Get("x-api-key"))
			}
			_, _ = w.Write([]byte("source-image"))
		case "/albums", "/tags":
			_, _ = w.Write([]byte(`[]`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer source.Close()
	destination := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/api")
		switch path {
		case "/assets/bulk-upload-check":
			_, _ = w.Write([]byte(`{"results":[{"id":"source-asset","action":"accept"}]}`))
		case "/assets":
			if err := r.ParseMultipartForm(1 << 20); err != nil {
				t.Fatal(err)
			}
			if _, _, err := r.FormFile("assetData"); err != nil {
				t.Fatalf("destination asset upload missing file: %v", err)
			}
			_, _ = w.Write([]byte(`{"id":"destination-asset","status":"created"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer destination.Close()
	t.Setenv("IMMICH_BASE_URL", destination.URL)
	t.Setenv("IMMICH_BASE_PATH", "")
	t.Setenv("IMMICH_API_KEY", "destination-key")
	t.Setenv("SOURCE_API_KEY", "source-key")
	if _, _, err := runRootArgs(t, "import", "immich", "--source-url", source.URL, "--source-api-key-env", "SOURCE_API_KEY", "--max-files", "1", "--json", "--no-learn", "--no-cache"); err != nil {
		t.Fatal(err)
	}
	if sourceRequests < 4 {
		t.Fatalf("source requests = %d, want metadata, original, albums, and tags", sourceRequests)
	}
	entries, err := os.ReadDir(tmpRoot)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("import immich left temporary downloads in %s: %#v", tmpRoot, entries)
	}
}

func TestPersonalRitualCommandPathsExist(t *testing.T) {
	root := RootCmd()
	for _, path := range [][]string{{"import", "folder"}, {"import", "watch"}, {"import", "archive"}, {"import", "takeout"}, {"import", "icloud"}, {"import", "immich"}, {"album", "event"}, {"duplicates", "plan"}, {"duplicates", "apply"}, {"people", "july"}, {"memories", "review"}, {"library", "review"}, {"stacks", "review"}, {"library", "health"}} {
		if cmd, _, err := root.Find(path); err != nil || cmd == nil {
			t.Fatalf("missing command %v: %v", path, err)
		}
	}
}

func TestLibraryNovelCommandsHaveExactTopLevelHelpPaths(t *testing.T) {
	for _, check := range []struct {
		args  []string
		usage string
		short string
	}{
		{args: []string{"library", "review", "--help"}, usage: "immich-pp-cli library review", short: "Review favorites or archived assets without mutation"},
		{args: []string{"library", "health", "--help"}, usage: "immich-pp-cli library health", short: "Report partner sharing and worker-pressure facts"},
	} {
		out, stderr, err := runRootArgs(t, check.args...)
		if err != nil {
			t.Fatalf("%s: help error: %v (stderr=%q)", strings.Join(check.args[:2], " "), err, stderr)
		}
		if !strings.Contains(out, check.usage) || !strings.Contains(out, check.short) {
			t.Fatalf("%s: help did not expose exact novel command path and short:\n%s", strings.Join(check.args[:2], " "), out)
		}
	}
}
