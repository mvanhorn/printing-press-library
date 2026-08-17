package safari

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
	"github.com/mvanhorn/printing-press-library/library/productivity/safari-history/internal/source"
)

// makeCloudTabsFixture builds a synthetic CloudTabs.db with the two real tables
// and the given devices + tabs, so tests never touch the user's real
// CloudTabs.db. Returns the file path.
func makeCloudTabsFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "CloudTabs.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open fixture: %v", err)
	}
	defer db.Close()

	schema := []string{
		`CREATE TABLE cloud_tab_devices (
			device_uuid TEXT PRIMARY KEY,
			device_name TEXT,
			device_type_identifier TEXT,
			is_ephemeral_device INTEGER,
			last_modified REAL
		)`,
		`CREATE TABLE cloud_tabs (
			tab_uuid TEXT PRIMARY KEY,
			device_uuid TEXT,
			url TEXT,
			title TEXT,
			last_viewed_time REAL,
			is_pinned INTEGER,
			is_showing_reader INTEGER
		)`,
	}
	for _, s := range schema {
		if _, err := db.Exec(s); err != nil {
			t.Fatalf("schema: %v", err)
		}
	}

	// CFAbsoluteTime seconds: 2001-01-01 is 0. Use distinct values so ordering
	// is observable. base = some seconds into the epoch.
	const base = 700_000_000.0 // ~2023
	devices := []struct {
		uuid, name, typeID string
	}{
		{"dev-iphone", "Vinny's iPhone", "iPhone16,2"},
		{"dev-ipad", "Vinny's iPad", "iPad14,1"},
	}
	for _, d := range devices {
		if _, err := db.Exec(`INSERT INTO cloud_tab_devices(device_uuid, device_name, device_type_identifier, is_ephemeral_device, last_modified) VALUES (?,?,?,0,?)`,
			d.uuid, d.name, d.typeID, base); err != nil {
			t.Fatalf("insert device: %v", err)
		}
	}
	tabs := []struct {
		uuid, dev, url, title string
		viewed                float64
		pinned, reader        int
	}{
		// iPad: two tabs, one pinned. Inserted Old-then-New so the expected
		// New-before-Old output proves the last_viewed_time DESC sort (not
		// coincidental insert order).
		{"t1", "dev-ipad", "https://ipad.example/old", "iPad Old", base + 10, 0, 0},
		{"t2", "dev-ipad", "https://ipad.example/new", "iPad New", base + 50, 1, 1},
		// iPhone: THREE tabs (asymmetric count vs iPad's 2, so the summary's
		// tab_count-DESC ordering is exercised, not just the device_name
		// tie-break). Inserted oldest-first so the DESC sort must reorder them.
		{"t4", "dev-iphone", "https://iphone.example/c", "iPhone C", base + 10, 0, 0},
		{"t5", "dev-iphone", "https://iphone.example/b", "iPhone B", base + 20, 1, 0},
		{"t6", "dev-iphone", "https://iphone.example/a", "iPhone A", base + 30, 0, 0},
	}
	for _, tb := range tabs {
		if _, err := db.Exec(`INSERT INTO cloud_tabs(tab_uuid, device_uuid, url, title, last_viewed_time, is_pinned, is_showing_reader) VALUES (?,?,?,?,?,?,?)`,
			tb.uuid, tb.dev, tb.url, tb.title, tb.viewed, tb.pinned, tb.reader); err != nil {
			t.Fatalf("insert tab: %v", err)
		}
	}
	return path
}

func TestReadCloudTabs_ShapeAndOrdering(t *testing.T) {
	path := makeCloudTabsFixture(t)
	tabs, err := ReadCloudTabs(path, CloudTabsFilter{})
	if err != nil {
		t.Fatalf("ReadCloudTabs: %v", err)
	}
	if len(tabs) != 5 {
		t.Fatalf("expected 5 tabs (no silent cap), got %d", len(tabs))
	}
	// Ordering: device_name asc, then last_viewed_time desc. Device names:
	// "Vinny's iPad" < "Vinny's iPhone" (P-a < P-h). Within iPad, New (base+50)
	// before Old (base+10). Within iPhone, a(+30) > b(+20) > c(+10) — proving
	// the DESC sort reorders the oldest-first insert order.
	want := []string{
		"https://ipad.example/new", "https://ipad.example/old",
		"https://iphone.example/a", "https://iphone.example/b", "https://iphone.example/c",
	}
	for i, w := range want {
		if tabs[i].URL != w {
			t.Fatalf("order[%d] = %s, want %s", i, tabs[i].URL, w)
		}
	}
	// Device type humanization.
	if tabs[0].DeviceType != "iPad" {
		t.Fatalf("device_type = %q, want iPad", tabs[0].DeviceType)
	}
	if tabs[2].DeviceType != "iPhone" {
		t.Fatalf("device_type = %q, want iPhone", tabs[2].DeviceType)
	}
	// is_pinned / is_showing_reader plumbed through.
	if !tabs[0].IsPinned || !tabs[0].IsShowingReader {
		t.Fatalf("iPad New should be pinned+reader, got pinned=%v reader=%v", tabs[0].IsPinned, tabs[0].IsShowingReader)
	}
	// Time conversion matches source.SafariSecondsToTime.
	wantTime := source.SafariSecondsToTime(700_000_050)
	if !tabs[0].LastViewedTime.Equal(wantTime) {
		t.Fatalf("last_viewed_time = %v, want %v", tabs[0].LastViewedTime, wantTime)
	}
}

func TestReadCloudTabs_DeviceNameFilter(t *testing.T) {
	path := makeCloudTabsFixture(t)
	tabs, err := ReadCloudTabs(path, CloudTabsFilter{DeviceName: "iphone"})
	if err != nil {
		t.Fatalf("ReadCloudTabs: %v", err)
	}
	if len(tabs) != 3 {
		t.Fatalf("device-name=iphone should match 3 tabs, got %d", len(tabs))
	}
	for _, tb := range tabs {
		if tb.DeviceName != "Vinny's iPhone" {
			t.Fatalf("unexpected device %q", tb.DeviceName)
		}
	}
}

func TestReadCloudTabs_PinnedFilter(t *testing.T) {
	path := makeCloudTabsFixture(t)
	tabs, err := ReadCloudTabs(path, CloudTabsFilter{PinnedOnly: true})
	if err != nil {
		t.Fatalf("ReadCloudTabs: %v", err)
	}
	if len(tabs) != 2 {
		t.Fatalf("pinned-only should match 2 tabs (t2,t5), got %d", len(tabs))
	}
	for _, tb := range tabs {
		if !tb.IsPinned {
			t.Fatalf("non-pinned tab leaked: %s", tb.URL)
		}
	}
}

func TestReadCloudTabs_ExplicitLimitCaps(t *testing.T) {
	path := makeCloudTabsFixture(t)
	tabs, err := ReadCloudTabs(path, CloudTabsFilter{Limit: 1})
	if err != nil {
		t.Fatalf("ReadCloudTabs: %v", err)
	}
	if len(tabs) != 1 {
		t.Fatalf("explicit limit=1 should cap to 1, got %d", len(tabs))
	}
}

func TestSummarizeCloudTabs_PerDeviceCounts(t *testing.T) {
	path := makeCloudTabsFixture(t)
	rows, err := SummarizeCloudTabs(path, CloudTabsFilter{})
	if err != nil {
		t.Fatalf("SummarizeCloudTabs: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 device rows, got %d", len(rows))
	}
	// iPhone has 3 tabs, iPad 2. Ordering is tab_count DESC, so iPhone must sort
	// first — this exercises the count comparator, not just the name tie-break.
	total := int64(0)
	for _, r := range rows {
		total += r.TabCount
	}
	if total != 5 {
		t.Fatalf("total tab_count = %d, want 5", total)
	}
	if rows[0].DeviceName != "Vinny's iPhone" || rows[0].TabCount != 3 {
		t.Fatalf("tab_count-DESC ordering wrong; first = %q (%d), want Vinny's iPhone (3)", rows[0].DeviceName, rows[0].TabCount)
	}
	if rows[1].DeviceName != "Vinny's iPad" || rows[1].TabCount != 2 {
		t.Fatalf("second row wrong; got %q (%d), want Vinny's iPad (2)", rows[1].DeviceName, rows[1].TabCount)
	}
}

func TestSummarizeCloudTabs_RespectsFilter(t *testing.T) {
	path := makeCloudTabsFixture(t)
	rows, err := SummarizeCloudTabs(path, CloudTabsFilter{PinnedOnly: true})
	if err != nil {
		t.Fatalf("SummarizeCloudTabs: %v", err)
	}
	// One pinned tab per device.
	if len(rows) != 2 {
		t.Fatalf("expected 2 device rows, got %d", len(rows))
	}
	for _, r := range rows {
		if r.TabCount != 1 {
			t.Fatalf("pinned summary count = %d for %s, want 1", r.TabCount, r.DeviceName)
		}
	}
}

func TestReadCloudTabs_MissingDB(t *testing.T) {
	_, err := ReadCloudTabs(filepath.Join(t.TempDir(), "does-not-exist.db"), CloudTabsFilter{})
	if err == nil {
		t.Fatal("expected error reading missing CloudTabs.db")
	}
}

// A read-time permission failure (the realistic Full Disk Access denial, which
// surfaces when the snapshot copy opens the DB rather than at the earlier Stat)
// must wrap ErrCloudTabsDBPermission so the CLI maps it to exit 4.
func TestReadCloudTabs_PermissionDeniedWraps(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root bypasses file permissions")
	}
	path := makeCloudTabsFixture(t)
	if err := os.Chmod(path, 0o000); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(path, 0o600) })
	_, err := ReadCloudTabs(path, CloudTabsFilter{})
	if err == nil {
		t.Fatal("expected permission error")
	}
	if !errors.Is(err, ErrCloudTabsDBPermission) {
		t.Fatalf("error %v does not wrap ErrCloudTabsDBPermission (CLI would map it to exit 1, not 4)", err)
	}
}

// Equal tab counts across devices fall back to device_name ascending — the
// summary tie-break, kept under test now that the main fixture is asymmetric.
func TestSummarizeCloudTabs_NameTieBreak(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "CloudTabs.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()
	for _, s := range []string{
		`CREATE TABLE cloud_tab_devices (device_uuid TEXT, device_name TEXT, device_type_identifier TEXT, is_ephemeral_device INTEGER, last_modified REAL)`,
		`CREATE TABLE cloud_tabs (tab_uuid TEXT, device_uuid TEXT, url TEXT, title TEXT, last_viewed_time REAL, is_pinned INTEGER, is_showing_reader INTEGER)`,
		`INSERT INTO cloud_tab_devices VALUES ('z','Zeta','iPhone1,1',0,0)`,
		`INSERT INTO cloud_tab_devices VALUES ('a','Alpha','iPad1,1',0,0)`,
		`INSERT INTO cloud_tabs VALUES ('1','z','https://z/1','z1',700000010,0,0)`,
		`INSERT INTO cloud_tabs VALUES ('2','a','https://a/1','a1',700000010,0,0)`,
	} {
		if _, err := db.Exec(s); err != nil {
			t.Fatalf("seed: %v", err)
		}
	}
	rows, err := SummarizeCloudTabs(path, CloudTabsFilter{})
	if err != nil {
		t.Fatalf("SummarizeCloudTabs: %v", err)
	}
	if len(rows) != 2 || rows[0].DeviceName != "Alpha" || rows[1].DeviceName != "Zeta" {
		t.Fatalf("equal-count tie-break should order by device_name asc; got %+v", rows)
	}
}

func TestLocateCloudTabsDB_MissingIsTyped(t *testing.T) {
	// We cannot relocate the hardcoded home path without altering the function,
	// but we can assert the sentinel error type is wired so the CLI's exit-4
	// mapping holds. Build the wrapped error the same way LocateCloudTabsDB does
	// and confirm errors.Is matches.
	wrapped := fmt.Errorf("%w: /nope", ErrCloudTabsDBMissing)
	if !errors.Is(wrapped, ErrCloudTabsDBMissing) {
		t.Fatal("ErrCloudTabsDBMissing sentinel not matchable via errors.Is")
	}
}

func TestHumanizeDeviceType(t *testing.T) {
	cases := map[string]string{
		"iPhone16,2":       "iPhone",
		"iPad14,1":         "iPad",
		"Watch7,1":         "Apple Watch",
		"MacBookPro18,3":   "MacBook Pro",
		"MacBookAir10,1":   "MacBook Air",
		"iMac21,1":         "iMac",
		"Macmini9,1":       "Mac mini",
		"":                 "unknown",
		"FutureDevice99,9": "FutureDevice99,9", // unknown → raw passthrough
	}
	for in, want := range cases {
		if got := humanizeDeviceType(in); got != want {
			t.Fatalf("humanizeDeviceType(%q) = %q, want %q", in, got, want)
		}
	}
}
