package cli

import (
	"bytes"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/devices/walkingpad/internal/history"
	"github.com/mvanhorn/printing-press-library/library/devices/walkingpad/internal/wpble"
	"github.com/spf13/cobra"
)

func TestCaptureAndSaveSynchronizesInFlightStatusCallbacks(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CACHE_HOME", home)

	var out bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&out)
	flags := &rootFlags{asJSON: true}

	stop := make(chan struct{})
	done := make(chan struct{})
	var started sync.WaitGroup
	started.Add(1)

	err := captureAndSave(cmd, flags, func(onStatus func(wpble.Status) error) error {
		if err := onStatus(wpble.Status{SpeedKmh: 1.2, DistanceM: 10, Steps: 20, BeltState: 1}); err != nil {
			return err
		}
		go func() {
			defer close(done)
			started.Done()
			for {
				select {
				case <-stop:
					return
				default:
					_ = onStatus(wpble.Status{SpeedKmh: 1.5, DistanceM: 20, Steps: 30, BeltState: 1})
					time.Sleep(time.Millisecond)
				}
			}
		}()
		started.Wait()
		return nil
	})
	close(stop)
	<-done
	if err != nil {
		t.Fatalf("captureAndSave() error = %v", err)
	}

	historyDir, err := history.DefaultDir(appName)
	if err != nil {
		t.Fatalf("history default dir: %v", err)
	}
	store, err := history.Open(historyDir)
	if err != nil {
		t.Fatalf("open history: %v", err)
	}
	sessions, err := store.Sessions()
	if err != nil {
		t.Fatalf("sessions: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("sessions len = %d, want 1", len(sessions))
	}
	samples, err := os.ReadFile(historyDir + "/samples.jsonl")
	if err != nil {
		t.Fatalf("read samples: %v", err)
	}
	if len(samples) == 0 {
		t.Fatal("expected persisted samples")
	}
}
