package cli

import (
	"context"
	"testing"
	"time"
)

func TestSyncStoreStopsAfterDaemonChangesStore(t *testing.T) {
	before := storeSnapshot{Count: 10, Latest: 100}
	after := storeSnapshot{Count: 11, Latest: 200}
	calls := []string{}
	snapshots := []storeSnapshot{before, before, after}

	result, err := syncStore(context.Background(), syncOptions{DaemonWait: time.Second, AppWait: time.Second, PollInterval: time.Millisecond}, syncHooks{
		Snapshot: func() (storeSnapshot, error) {
			s := snapshots[0]
			if len(snapshots) > 1 {
				snapshots = snapshots[1:]
			}
			return s, nil
		},
		KickDaemon:      func() error { calls = append(calls, "daemon"); return nil },
		AppRunning:      func() (bool, error) { calls = append(calls, "app-running"); return false, nil },
		LaunchHidden:    func() error { calls = append(calls, "launch"); return nil },
		QuitLaunchedApp: func() error { calls = append(calls, "quit"); return nil },
		Sleep:           func(time.Duration) {},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Changed || result.Method != "voicememod" {
		t.Fatalf("unexpected result: %+v", result)
	}
	if result.AppLaunched {
		t.Fatal("app should not launch when daemon refreshes the store")
	}
	if len(calls) != 1 || calls[0] != "daemon" {
		t.Fatalf("unexpected calls: %v", calls)
	}
}

func TestSyncStoreFallsBackToHiddenAppAndQuitsOnlyOwnedInstance(t *testing.T) {
	before := storeSnapshot{Count: 10, Latest: 100}
	after := storeSnapshot{Count: 12, Latest: 300}
	calls := []string{}
	reads := 0

	result, err := syncStore(context.Background(), syncOptions{DaemonWait: time.Nanosecond, AppWait: time.Second, PollInterval: time.Millisecond}, syncHooks{
		Snapshot: func() (storeSnapshot, error) {
			reads++
			if reads >= 4 {
				return after, nil
			}
			return before, nil
		},
		KickDaemon:      func() error { calls = append(calls, "daemon"); return nil },
		AppRunning:      func() (bool, error) { calls = append(calls, "app-running"); return false, nil },
		LaunchHidden:    func() error { calls = append(calls, "launch"); return nil },
		QuitLaunchedApp: func() error { calls = append(calls, "quit"); return nil },
		Sleep:           func(time.Duration) {},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Changed || result.Method != "hidden-app" || !result.AppLaunched || !result.AppQuit {
		t.Fatalf("unexpected result: %+v", result)
	}
	want := []string{"daemon", "app-running", "launch", "quit"}
	if len(calls) != len(want) {
		t.Fatalf("calls=%v want=%v", calls, want)
	}
	for i := range want {
		if calls[i] != want[i] {
			t.Fatalf("calls=%v want=%v", calls, want)
		}
	}
}

func TestSyncStoreSettlesAfterHiddenAppChange(t *testing.T) {
	before := storeSnapshot{Count: 10, Latest: 100}
	firstChange := storeSnapshot{Count: 11, Latest: 200}
	settled := storeSnapshot{Count: 12, Latest: 300}
	reads := 0

	result, err := syncStore(context.Background(), syncOptions{DaemonWait: time.Nanosecond, AppWait: time.Second, PollInterval: time.Millisecond, SettleWait: time.Millisecond}, syncHooks{
		Snapshot: func() (storeSnapshot, error) {
			reads++
			switch {
			case reads < 4:
				return before, nil
			case reads == 4:
				return firstChange, nil
			default:
				return settled, nil
			}
		},
		KickDaemon:      func() error { return nil },
		AppRunning:      func() (bool, error) { return false, nil },
		LaunchHidden:    func() error { return nil },
		QuitLaunchedApp: func() error { return nil },
		Sleep:           func(time.Duration) {},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.After.Count != settled.Count || result.After.Latest != settled.Latest {
		t.Fatalf("after=%+v want settled=%+v", result.After, settled)
	}
}

func TestSyncStoreDoesNotQuitAppThatWasAlreadyRunning(t *testing.T) {
	before := storeSnapshot{Count: 10, Latest: 100}
	calls := []string{}

	result, err := syncStore(context.Background(), syncOptions{DaemonWait: time.Nanosecond, AppWait: time.Nanosecond, PollInterval: time.Millisecond}, syncHooks{
		Snapshot:        func() (storeSnapshot, error) { return before, nil },
		KickDaemon:      func() error { calls = append(calls, "daemon"); return nil },
		AppRunning:      func() (bool, error) { calls = append(calls, "app-running"); return true, nil },
		LaunchHidden:    func() error { calls = append(calls, "launch"); return nil },
		QuitLaunchedApp: func() error { calls = append(calls, "quit"); return nil },
		Sleep:           func(time.Duration) {},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.AppLaunched || result.AppQuit {
		t.Fatalf("must not own or quit an existing app: %+v", result)
	}
	if !result.Refreshed || result.FreshnessConfirmed {
		t.Fatalf("unexpected freshness semantics: %+v", result)
	}
	for _, call := range calls {
		if call == "quit" || call == "launch" {
			t.Fatalf("unexpected %s call: %v", call, calls)
		}
	}
}
