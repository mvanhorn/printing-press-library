package cli

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

type storeSnapshot struct {
	Count    int       `json:"recording_count"`
	Latest   float64   `json:"-"`
	LatestAt time.Time `json:"latest_recording_at,omitempty"`
	DBMod    time.Time `json:"db_modified_at,omitempty"`
	WALMod   time.Time `json:"wal_modified_at,omitempty"`
}

type syncOptions struct {
	DaemonWait   time.Duration
	AppWait      time.Duration
	PollInterval time.Duration
	SettleWait   time.Duration
}

type syncHooks struct {
	Snapshot        func() (storeSnapshot, error)
	KickDaemon      func() error
	AppRunning      func() (bool, error)
	LaunchHidden    func() error
	QuitLaunchedApp func() error
	Sleep           func(time.Duration)
}

type syncResult struct {
	SchemaVersion      int           `json:"schema_version"`
	Refreshed          bool          `json:"refreshed"`
	Changed            bool          `json:"changed"`
	FreshnessConfirmed bool          `json:"freshness_confirmed"`
	Method             string        `json:"method"`
	AppLaunched        bool          `json:"app_launched"`
	AppQuit            bool          `json:"app_quit"`
	Before             storeSnapshot `json:"before"`
	After              storeSnapshot `json:"after"`
	ElapsedMS          int64         `json:"elapsed_ms"`
	Warning            string        `json:"warning,omitempty"`
}

func syncStore(ctx context.Context, opts syncOptions, hooks syncHooks) (syncResult, error) {
	started := time.Now()
	before, err := hooks.Snapshot()
	if err != nil {
		return syncResult{}, err
	}
	result := syncResult{SchemaVersion: 1, Before: before, After: before, Method: "voicememod"}
	if err := hooks.KickDaemon(); err != nil {
		result.Warning = "could not kick voicememod: " + err.Error()
	}
	if after, changed, err := pollForChange(ctx, before, opts.DaemonWait, opts.PollInterval, hooks); err != nil {
		return result, err
	} else if changed {
		result.Refreshed, result.Changed, result.FreshnessConfirmed, result.After = true, true, true, after
		result.ElapsedMS = time.Since(started).Milliseconds()
		return result, nil
	}

	running, err := hooks.AppRunning()
	if err != nil {
		return result, err
	}
	if !running {
		if err := hooks.LaunchHidden(); err != nil {
			return result, fmt.Errorf("launch Voice Memos hidden: %w", err)
		}
		result.AppLaunched = true
		result.Method = "hidden-app"
	}

	after, changed, pollErr := pollForChange(ctx, before, opts.AppWait, opts.PollInterval, hooks)
	if pollErr == nil && changed && opts.SettleWait > 0 {
		after, pollErr = waitForSettle(ctx, after, opts.SettleWait, opts.PollInterval, hooks)
	}
	if result.AppLaunched {
		if err := hooks.QuitLaunchedApp(); err == nil {
			result.AppQuit = true
		} else if result.Warning == "" {
			result.Warning = "could not quit CLI-launched Voice Memos: " + err.Error()
		}
	}
	if pollErr != nil {
		return result, pollErr
	}
	result.After = after
	result.Refreshed = true
	result.Changed = changed
	result.FreshnessConfirmed = changed
	if !changed && result.Warning == "" {
		result.Warning = "store did not change during the sync window; it may already be current"
	}
	result.ElapsedMS = time.Since(started).Milliseconds()
	return result, nil
}

func pollForChange(ctx context.Context, before storeSnapshot, wait, interval time.Duration, hooks syncHooks) (storeSnapshot, bool, error) {
	if interval <= 0 {
		interval = 250 * time.Millisecond
	}
	attempts := int(math.Ceil(float64(wait)/float64(interval))) + 1
	if attempts < 1 {
		attempts = 1
	}
	last := before
	for i := 0; i < attempts; i++ {
		select {
		case <-ctx.Done():
			return last, false, ctx.Err()
		default:
		}
		current, err := hooks.Snapshot()
		if err != nil {
			return last, false, err
		}
		last = current
		if snapshotsDiffer(before, current) {
			return current, true, nil
		}
		if i+1 < attempts {
			hooks.Sleep(interval)
		}
	}
	return last, false, nil
}

func waitForSettle(ctx context.Context, current storeSnapshot, wait, interval time.Duration, hooks syncHooks) (storeSnapshot, error) {
	if interval <= 0 {
		interval = 250 * time.Millisecond
	}
	attempts := int(math.Ceil(float64(wait) / float64(interval)))
	if attempts < 1 {
		attempts = 1
	}
	last := current
	for i := 0; i < attempts; i++ {
		select {
		case <-ctx.Done():
			return last, ctx.Err()
		default:
		}
		hooks.Sleep(interval)
		next, err := hooks.Snapshot()
		if err != nil {
			return last, err
		}
		last = next
	}
	return last, nil
}

func snapshotsDiffer(a, b storeSnapshot) bool {
	return a.Count != b.Count || a.Latest != b.Latest || !a.DBMod.Equal(b.DBMod) || !a.WALMod.Equal(b.WALMod)
}

func snapshotStore(dbPath string) (storeSnapshot, error) {
	db, err := openDB(dbPath)
	if err != nil {
		return storeSnapshot{}, err
	}
	defer func() { _ = db.Close() }()
	if err := validateVoiceMemosSchema(db); err != nil {
		return storeSnapshot{}, err
	}
	var count int
	var latest sql.NullFloat64
	if err := db.QueryRow("SELECT count(*), max(ZDATE) FROM ZCLOUDRECORDING").Scan(&count, &latest); err != nil {
		return storeSnapshot{}, err
	}
	s := storeSnapshot{Count: count}
	if latest.Valid {
		s.Latest = latest.Float64
		s.LatestAt = coreDataToTime(latest.Float64)
	}
	if info, err := os.Stat(dbPath); err == nil {
		s.DBMod = info.ModTime()
	}
	if info, err := os.Stat(dbPath + "-wal"); err == nil {
		s.WALMod = info.ModTime()
	}
	return s, nil
}

func processIDs(ctx context.Context, name string) ([]int, error) {
	out, err := exec.CommandContext(ctx, "pgrep", "-x", name).Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
			return nil, nil
		}
		return nil, err
	}
	var ids []int
	for _, line := range strings.Fields(string(out)) {
		id, err := strconv.Atoi(line)
		if err == nil {
			ids = append(ids, id)
		}
	}
	return ids, nil
}

func processIdentity(ctx context.Context, id int) (string, error) {
	out, err := exec.CommandContext(ctx, "ps", "-p", strconv.Itoa(id), "-o", "lstart=", "-o", "comm=").Output()
	if err != nil {
		return "", err
	}
	identity := strings.TrimSpace(string(out))
	if identity == "" || !strings.Contains(identity, "VoiceMemos") {
		return "", fmt.Errorf("process %d is not VoiceMemos", id)
	}
	return identity, nil
}

func realSyncHooks(ctx context.Context, dbPath string) syncHooks {
	var beforePIDs = map[int]bool{}
	var ownedPIDs = map[int]string{}
	return syncHooks{
		Snapshot: func() (storeSnapshot, error) { return snapshotStore(dbPath) },
		KickDaemon: func() error {
			target := fmt.Sprintf("gui/%d/com.apple.voicememod", os.Getuid())
			return exec.CommandContext(ctx, "launchctl", "kickstart", target).Run()
		},
		AppRunning: func() (bool, error) {
			ids, err := processIDs(ctx, "VoiceMemos")
			if err != nil {
				return false, err
			}
			for _, id := range ids {
				beforePIDs[id] = true
			}
			return len(ids) > 0, nil
		},
		LaunchHidden: func() error {
			app := "/System/Applications/VoiceMemos.app"
			if _, err := os.Stat(app); err != nil {
				return err
			}
			if err := exec.CommandContext(ctx, "open", "-gj", "-n", app).Run(); err != nil {
				return err
			}
			for attempt := 0; attempt < 20; attempt++ {
				ids, err := processIDs(ctx, "VoiceMemos")
				if err != nil {
					return err
				}
				var candidates []int
				for _, id := range ids {
					if !beforePIDs[id] {
						candidates = append(candidates, id)
					}
				}
				if len(candidates) > 1 {
					return errors.New("multiple new Voice Memos processes appeared; refusing automatic cleanup")
				}
				if len(candidates) == 1 {
					identity, err := processIdentity(ctx, candidates[0])
					if err != nil {
						return err
					}
					ownedPIDs[candidates[0]] = identity
					return nil
				}
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(250 * time.Millisecond):
				}
			}
			return errors.New("voice memos launched but no owned process appeared")
		},
		QuitLaunchedApp: func() error {
			var errs []string
			for id, expectedIdentity := range ownedPIDs {
				identity, identityErr := processIdentity(ctx, id)
				if identityErr != nil || identity != expectedIdentity {
					errs = append(errs, fmt.Sprintf("process %d identity changed; refusing to signal", id))
					continue
				}
				p, err := os.FindProcess(id)
				if err != nil {
					errs = append(errs, err.Error())
					continue
				}
				if err := p.Signal(syscall.SIGTERM); err != nil && !errors.Is(err, os.ErrProcessDone) {
					errs = append(errs, err.Error())
				}
			}
			if len(errs) > 0 {
				return errors.New(strings.Join(errs, "; "))
			}
			for attempt := 0; attempt < 20; attempt++ {
				ids, err := processIDs(ctx, "VoiceMemos")
				if err != nil {
					return err
				}
				remaining := false
				for _, runningID := range ids {
					for ownedID := range ownedPIDs {
						if runningID == ownedID {
							remaining = true
						}
					}
				}
				if !remaining {
					return nil
				}
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(100 * time.Millisecond):
				}
			}
			return errors.New("timed out waiting for CLI-launched Voice Memos to quit")
		},
		Sleep: time.Sleep,
	}
}

func defaultSyncOptions() syncOptions {
	return syncOptions{DaemonWait: 3 * time.Second, AppWait: 12 * time.Second, PollInterval: 500 * time.Millisecond, SettleWait: 2 * time.Second}
}

func configuredSyncOptions() syncOptions {
	opts := defaultSyncOptions()
	if cfg.DaemonWait >= 0 {
		opts.DaemonWait = cfg.DaemonWait
	}
	if cfg.AppWait >= 0 {
		opts.AppWait = cfg.AppWait
	}
	if cfg.PollInterval > 0 {
		opts.PollInterval = cfg.PollInterval
	}
	if cfg.SettleWait >= 0 {
		opts.SettleWait = cfg.SettleWait
	}
	return opts
}

func runSync(ctx context.Context) (syncResult, error) {
	dbPath, _ := resolvePaths()
	if _, err := os.Stat(dbPath); err != nil {
		return syncResult{}, err
	}
	opts := configuredSyncOptions()
	total := opts.DaemonWait + opts.AppWait + opts.SettleWait + 10*time.Second
	bounded, cancel := context.WithTimeout(ctx, total)
	defer cancel()
	return syncStore(bounded, opts, realSyncHooks(bounded, filepath.Clean(dbPath)))
}
