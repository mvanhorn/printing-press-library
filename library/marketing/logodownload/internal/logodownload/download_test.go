package logodownload

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestDownloadImagesWritesSelectedImageAndPath(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/logo.png" {
			t.Fatalf("unexpected request: %s", r.URL.String())
		}
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write([]byte("png-bytes"))
	}))
	defer server.Close()

	results := []LogoResult{{
		Title:    "Banco Inter Logo",
		URL:      server.URL + "/banco-inter-logo/",
		ImageURL: server.URL + "/logo.png",
	}}
	outputDir := t.TempDir()

	err := DownloadImages(context.Background(), server.Client(), results, Selection{Mode: SelectFirst}, outputDir)
	if err != nil {
		t.Fatalf("DownloadImages returned error: %v", err)
	}

	if results[0].DownloadPath == "" {
		t.Fatal("expected download_path to be populated")
	}
	if filepath.Dir(results[0].DownloadPath) != outputDir {
		t.Fatalf("unexpected output dir: %q", results[0].DownloadPath)
	}

	contents, err := os.ReadFile(results[0].DownloadPath)
	if err != nil {
		t.Fatalf("expected downloaded file to exist: %v", err)
	}
	if string(contents) != "png-bytes" {
		t.Fatalf("unexpected file contents: %q", string(contents))
	}
}

func TestSelectedIndexesRejectsOutOfRangeIndex(t *testing.T) {
	_, err := selectedIndexes([]LogoResult{{Title: "Nike"}}, Selection{Mode: SelectIndex, Index: 2})
	if err == nil {
		t.Fatal("expected out-of-range index error")
	}
}
