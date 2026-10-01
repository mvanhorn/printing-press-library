// Copyright 2026 Jon Gouveia and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/ai/fish-audio/internal/client"
	"github.com/mvanhorn/printing-press-library/library/ai/fish-audio/internal/config"
)

// Transcription is billed; a 503 may arrive after the server accepted the
// upload, so transcribeFile must send the audio exactly once.
func TestTranscribeFileDoesNotReplayAfter503(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		http.Error(w, "busy", http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	audio := filepath.Join(t.TempDir(), "clip.wav")
	if err := os.WriteFile(audio, []byte("RIFF0000WAVE"), 0o600); err != nil {
		t.Fatal(err)
	}
	c := client.New(&config.Config{BaseURL: srv.URL}, 5*time.Second, 0)
	c.NoCache = true
	if _, err := transcribeFile(context.Background(), c, audio, "", false); err == nil {
		t.Fatal("expected the 503 to be reported")
	}
	if got := hits.Load(); got != 1 {
		t.Fatalf("ASR request sent %d times, want exactly 1", got)
	}
}
