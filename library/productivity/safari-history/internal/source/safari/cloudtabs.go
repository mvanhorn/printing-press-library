package safari

import (
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	_ "modernc.org/sqlite"
	"github.com/mvanhorn/printing-press-library/library/productivity/safari-history/internal/source"
)

// CloudTab is one synced iCloud tab open on one of the user's Apple devices.
// The last_viewed_time is converted from Safari's CFAbsoluteTime (seconds since
// 2001-01-01) to a Go time.Time via source.SafariSecondsToTime.
type CloudTab struct {
	DeviceName      string
	DeviceType      string
	Title           string
	URL             string
	LastViewedTime  time.Time
	IsPinned        bool
	IsShowingReader bool
}

// CloudTabDeviceSummary is one row of the --summary view: a per-device tab
// count, so a calling agent gets a deterministic "N tabs across M devices"
// total instead of fabricating one.
type CloudTabDeviceSummary struct {
	DeviceName string
	DeviceType string
	TabCount   int64
}

// CloudTabsFilter scopes a CloudTabs read. Empty fields mean "no filter".
type CloudTabsFilter struct {
	DeviceName string // substring match against device_name (case-insensitive)
	PinnedOnly bool   // only is_pinned tabs
	Limit      int    // <= 0 means unlimited (no silent cap)
}

// ErrCloudTabsDBMissing is returned when CloudTabs.db is absent — iCloud Tabs
// is not enabled, or Safari has never synced tabs on this Mac. The CLI maps it
// to ExitSourceDBMissing.
var ErrCloudTabsDBMissing = fmt.Errorf("safari cloudtabs db not found")

// ErrCloudTabsDBPermission is returned when CloudTabs.db exists but cannot be
// opened due to a permission error (typically the terminal lacks Full Disk
// Access for the Safari container). Distinct from ErrCloudTabsDBMissing so the
// CLI can point the user at the right fix.
var ErrCloudTabsDBPermission = fmt.Errorf("safari cloudtabs db permission denied")

// LocateCloudTabsDB returns the path to Safari's CloudTabs.db, or
// ErrCloudTabsDBMissing if it is absent / ErrCloudTabsDBPermission if it exists
// but is not readable.
func LocateCloudTabsDB() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	p := filepath.Join(home, "Library", "Containers", "com.apple.Safari", "Data", "Library", "Safari", "CloudTabs.db")
	if _, err := os.Stat(p); err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("%w: %s", ErrCloudTabsDBMissing, p)
		}
		if os.IsPermission(err) {
			return "", fmt.Errorf("%w: %s", ErrCloudTabsDBPermission, p)
		}
		return "", err
	}
	return p, nil
}

// RefreshSafari activates Safari (via osascript) and sleeps for wait so iCloud
// has time to sync the latest tabs into CloudTabs.db before a read. This is the
// only side-effecting operation in this file; the default read path never calls
// it. Errors from osascript are returned but are non-fatal at the call site —
// the caller may proceed with a (possibly staler) read.
func RefreshSafari(wait time.Duration) error {
	// Hardcoded AppleScript — no user input is interpolated, so there is no
	// script-injection surface here.
	cmd := exec.Command("osascript", "-e", `tell application "Safari" to activate`)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("osascript activate Safari: %w: %s", err, strings.TrimSpace(string(out)))
	}
	if wait > 0 {
		time.Sleep(wait)
	}
	return nil
}

// IsSafariRunning reports whether Safari is currently running without
// launching or foregrounding it. Errors mean the state could not be determined;
// callers should avoid presenting stale-data hints on an unknown state.
func IsSafariRunning() (bool, error) {
	cmd := exec.Command("osascript", "-e", `application "Safari" is running`)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return false, fmt.Errorf("osascript check Safari running: %w: %s", err, strings.TrimSpace(string(out)))
	}
	running, err := strconv.ParseBool(strings.TrimSpace(string(out)))
	if err != nil {
		return false, fmt.Errorf("parse Safari running state %q: %w", strings.TrimSpace(string(out)), err)
	}
	return running, nil
}

// snapshotCloudTabs copies CloudTabs.db to a temp file read-only (VACUUM INTO,
// with a byte-copy fallback when Safari holds an exclusive WAL lock), mirroring
// the History.db copySnapshot pattern. The caller must remove the returned temp
// path. The live DB is never modified.
//
// The snapshot holds the user's synced browsing URLs, so it is chmod'd 0600
// (VACUUM INTO / cp would otherwise create it 0644). On a permission failure —
// the realistic case is the terminal lacking Full Disk Access for the Safari
// container, which surfaces at copy time rather than at the earlier os.Stat —
// the error is wrapped as ErrCloudTabsDBPermission so the CLI maps it to exit 4
// and shows the Full Disk Access hint instead of a bare exit 1.
func snapshotCloudTabs(src string) (string, error) {
	tmp := filepath.Join(os.TempDir(), fmt.Sprintf("cloudtabs-snapshot-%d.db", time.Now().UnixNano()))
	if err := copySnapshot(src, tmp); err != nil {
		_ = os.Remove(tmp)
		if isPermissionErr(err) {
			return "", fmt.Errorf("%w: %s: %v", ErrCloudTabsDBPermission, src, err)
		}
		return "", err
	}
	// Tighten permissions on the sensitive snapshot (defense-in-depth; the
	// macOS per-user $TMPDIR is already 0700, but make the intent explicit and
	// robust to a non-default TMPDIR). Best-effort: a chmod failure does not
	// invalidate an otherwise-good snapshot.
	_ = os.Chmod(tmp, 0o600)
	return tmp, nil
}

// isPermissionErr reports whether err (or anything it wraps) is a
// permission-denied error. copySnapshot's cp fallback returns the permission
// failure as a formatted string rather than a wrapped fs.ErrPermission, so we
// also match on the message as a fallback.
func isPermissionErr(err error) bool {
	if errors.Is(err, fs.ErrPermission) {
		return true
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "permission denied") || strings.Contains(msg, "operation not permitted")
}

// ReadCloudTabs reads synced iCloud tabs from CloudTabs.db. It snapshots the DB
// read-only first (CloudTabs.db runs in WAL mode while Safari is open), reads
// the cloud_tab_devices + cloud_tabs tables, applies the filter, and returns
// tabs ordered by device_name then last_viewed_time desc (deterministic).
func ReadCloudTabs(dbPath string, f CloudTabsFilter) ([]CloudTab, error) {
	tmp, err := snapshotCloudTabs(dbPath)
	if err != nil {
		return nil, err
	}
	defer os.Remove(tmp)

	db, err := sql.Open("sqlite", "file:"+tmp+"?mode=ro")
	if err != nil {
		return nil, err
	}
	defer db.Close()

	devices, err := readDeviceMap(db)
	if err != nil {
		return nil, err
	}

	rows, err := db.Query(`SELECT
		COALESCE(t.device_uuid,''),
		COALESCE(t.url,''),
		COALESCE(t.title,''),
		COALESCE(t.last_viewed_time,0),
		COALESCE(t.is_pinned,0),
		COALESCE(t.is_showing_reader,0)
	FROM cloud_tabs t`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []CloudTab{}
	for rows.Next() {
		var deviceUUID, url, title string
		var lastViewed float64
		var pinned, reader int64
		if err := rows.Scan(&deviceUUID, &url, &title, &lastViewed, &pinned, &reader); err != nil {
			return nil, err
		}
		dev := devices[deviceUUID]
		tab := CloudTab{
			DeviceName:      dev.name,
			DeviceType:      humanizeDeviceType(dev.typeIdentifier),
			Title:           title,
			URL:             url,
			LastViewedTime:  source.SafariSecondsToTime(lastViewed),
			IsPinned:        pinned != 0,
			IsShowingReader: reader != 0,
		}
		if f.PinnedOnly && !tab.IsPinned {
			continue
		}
		if f.DeviceName != "" && !strings.Contains(strings.ToLower(tab.DeviceName), strings.ToLower(f.DeviceName)) {
			continue
		}
		out = append(out, tab)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	sortCloudTabs(out)
	if f.Limit > 0 && len(out) > f.Limit {
		out = out[:f.Limit]
	}
	return out, nil
}

// SummarizeCloudTabs returns a per-device tab count (the --summary view),
// ordered by tab_count desc then device_name. The same DeviceName/PinnedOnly
// filters apply; Limit caps the number of device rows only when explicitly set.
func SummarizeCloudTabs(dbPath string, f CloudTabsFilter) ([]CloudTabDeviceSummary, error) {
	// Reuse the full read (unlimited) so the summary counts every matching tab,
	// then aggregate. Limit on the summary caps device rows, not tabs.
	tabFilter := f
	tabFilter.Limit = 0
	tabs, err := ReadCloudTabs(dbPath, tabFilter)
	if err != nil {
		return nil, err
	}
	type agg struct {
		name  string
		dtype string
		count int64
	}
	counts := map[string]*agg{}
	order := []string{}
	for _, t := range tabs {
		key := t.DeviceName + "\x00" + t.DeviceType
		a, ok := counts[key]
		if !ok {
			a = &agg{name: t.DeviceName, dtype: t.DeviceType}
			counts[key] = a
			order = append(order, key)
		}
		a.count++
	}
	out := make([]CloudTabDeviceSummary, 0, len(order))
	for _, k := range order {
		a := counts[k]
		out = append(out, CloudTabDeviceSummary{DeviceName: a.name, DeviceType: a.dtype, TabCount: a.count})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].TabCount != out[j].TabCount {
			return out[i].TabCount > out[j].TabCount
		}
		return out[i].DeviceName < out[j].DeviceName
	})
	if f.Limit > 0 && len(out) > f.Limit {
		out = out[:f.Limit]
	}
	return out, nil
}

type deviceMeta struct {
	name           string
	typeIdentifier string
}

func readDeviceMap(db *sql.DB) (map[string]deviceMeta, error) {
	rows, err := db.Query(`SELECT
		COALESCE(device_uuid,''),
		COALESCE(device_name,''),
		COALESCE(device_type_identifier,'')
	FROM cloud_tab_devices`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]deviceMeta{}
	for rows.Next() {
		var uuid, name, typeID string
		if err := rows.Scan(&uuid, &name, &typeID); err != nil {
			return nil, err
		}
		out[uuid] = deviceMeta{name: name, typeIdentifier: typeID}
	}
	return out, rows.Err()
}

func sortCloudTabs(tabs []CloudTab) {
	sort.SliceStable(tabs, func(i, j int) bool {
		if tabs[i].DeviceName != tabs[j].DeviceName {
			return tabs[i].DeviceName < tabs[j].DeviceName
		}
		if !tabs[i].LastViewedTime.Equal(tabs[j].LastViewedTime) {
			return tabs[i].LastViewedTime.After(tabs[j].LastViewedTime)
		}
		// Final tie-break on URL keeps the order fully deterministic when two
		// tabs on one device share an identical last_viewed_time.
		return tabs[i].URL < tabs[j].URL
	})
}

// humanizeDeviceType maps Apple's device_type_identifier strings to a friendly
// label. The identifiers are dotted reverse-DNS-ish tokens such as
// "iPhone17,1" or family strings like "com.apple.iphone"; we match on the
// recognizable family substring and fall back to the raw identifier so an
// unknown future device type is still surfaced rather than blanked.
func humanizeDeviceType(id string) string {
	if strings.TrimSpace(id) == "" {
		return "unknown"
	}
	lower := strings.ToLower(id)
	switch {
	case strings.Contains(lower, "ipad"):
		return "iPad"
	case strings.Contains(lower, "iphone"):
		return "iPhone"
	case strings.Contains(lower, "ipod"):
		return "iPod"
	case strings.Contains(lower, "watch"):
		return "Apple Watch"
	case strings.Contains(lower, "macbookpro"):
		return "MacBook Pro"
	case strings.Contains(lower, "macbookair"):
		return "MacBook Air"
	case strings.Contains(lower, "macbook"):
		return "MacBook"
	case strings.Contains(lower, "imac"):
		return "iMac"
	case strings.Contains(lower, "macmini") || strings.Contains(lower, "mac mini"):
		return "Mac mini"
	case strings.Contains(lower, "macstudio"):
		return "Mac Studio"
	case strings.Contains(lower, "macpro"):
		return "Mac Pro"
	case strings.Contains(lower, "mac"):
		return "Mac"
	default:
		return id
	}
}
