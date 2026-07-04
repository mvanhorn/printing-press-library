package cli

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/productivity/safari-history/internal/output"
	safariSource "github.com/mvanhorn/printing-press-library/library/productivity/safari-history/internal/source/safari"
)

// Indirection seams so the command's RunE is testable without touching the
// user's real CloudTabs.db or foregrounding Safari. Tests override these to
// point at a synthetic fixture and to assert that --refresh is invoked only
// when requested. Production wiring is the real source-package functions.
var (
	locateCloudTabsDBFn  = safariSource.LocateCloudTabsDB
	readCloudTabsFn      = safariSource.ReadCloudTabs
	summarizeCloudTabsFn = safariSource.SummarizeCloudTabs
	refreshSafariFn      = safariSource.RefreshSafari
	safariRunningFn      = safariSource.IsSafariRunning
)

// mapCloudTabsDBErr maps the source-package CloudTabs sentinels to the CLI's
// typed exit vocabulary (ErrSourceDBMissing → exit 4). It is applied at EVERY
// CloudTabs access site — locate, read, and summarize — because Full Disk Access
// is typically denied at copy/snapshot time (inside read/summarize), not at the
// initial stat in locate. Without this, a read-time permission denial would fall
// through to the generic exit 1 instead of the documented exit 4.
func mapCloudTabsDBErr(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, safariSource.ErrCloudTabsDBMissing) {
		return fmt.Errorf("%w: iCloud Tabs not found — enable it on iPhone via Settings → Apps → Safari (iCloud Tabs), and grant this terminal Full Disk Access (System Settings → Privacy & Security → Full Disk Access). Detail: %v", ErrSourceDBMissing, err)
	}
	if errors.Is(err, safariSource.ErrCloudTabsDBPermission) {
		return fmt.Errorf("%w: CloudTabs.db exists but is not readable — grant this terminal Full Disk Access (System Settings → Privacy & Security → Full Disk Access). Detail: %v", ErrSourceDBMissing, err)
	}
	return err
}

// newICloudTabsCmd lists synced iCloud tabs — the open tabs from the user's
// OTHER Apple devices — read directly from Safari's CloudTabs.db. It does NOT
// use the history snapshot/sync path; CloudTabs is a separate, always-current
// (when Safari is running) datastore. A pure read can return stale tabs because
// CloudTabs.db only updates while Safari is open, so --refresh activates Safari
// and waits before reading.
func newICloudTabsCmd(opts *RootOptions) *cobra.Command {
	var (
		summary    bool
		deviceName string
		pinned     bool
		refresh    bool
		wait       int
	)
	cmd := &cobra.Command{
		Use:   "icloud-tabs",
		Short: "List synced iCloud tabs open on your other Apple devices (read from Safari's CloudTabs.db)",
		Long: strings.TrimSpace(`
List synced iCloud tabs — the open tabs from your OTHER Apple devices — read
directly from Safari's CloudTabs.db.

CloudTabs.db only updates while Safari is running, so a pure read can return
stale data. Use --refresh to activate Safari and wait --wait seconds (default 5)
for iCloud to sync the latest tabs before reading. Without --refresh the read is
a pure read with no app side effect.
`),
		Example:     strings.Trim("safari-history-pp-cli icloud-tabs --json\nsafari-history-pp-cli icloud-tabs --summary\nsafari-history-pp-cli icloud-tabs --device-name iPhone --pinned\nsafari-history-pp-cli icloud-tabs --refresh --wait 5", "\n"),
		Annotations: map[string]string{"pp:typed-exit-codes": "0,2,4", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			// --refresh is the only side-effecting path: activate Safari and
			// wait so iCloud syncs the freshest tabs before we read. An
			// osascript failure is non-fatal — fall through and read whatever
			// CloudTabs.db currently holds, with a stderr note.
			if refresh {
				if err := refreshSafariFn(time.Duration(wait) * time.Second); err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: --refresh failed (%v); reading current CloudTabs.db\n", err)
				}
			}

			dbPath, err := locateCloudTabsDBFn()
			if err != nil {
				return mapCloudTabsDBErr(err)
			}

			// The root --limit defaults to 20. icloud-tabs must NOT apply that
			// silently — default to unlimited and only cap when the user
			// explicitly passed --limit.
			limit := 0
			if cmd.Flags().Changed("limit") {
				limit = opts.Output.Limit
			}
			filter := safariSource.CloudTabsFilter{
				DeviceName: deviceName,
				PinnedOnly: pinned,
				Limit:      limit,
			}

			if summary {
				rows, err := summarizeCloudTabsFn(dbPath, filter)
				if err != nil {
					return mapCloudTabsDBErr(err)
				}
				maybeStaleTabsHint(cmd, cloudTabsSummaryCount(rows), refresh)
				out := make([]map[string]any, 0, len(rows))
				for _, r := range rows {
					out = append(out, map[string]any{
						"device_name": r.DeviceName,
						"device_type": r.DeviceType,
						"tab_count":   r.TabCount,
					})
				}
				output.DefaultToJSONIfNotTTY(&opts.Output)
				return output.Render(opts.Output, out)
			}

			tabs, err := readCloudTabsFn(dbPath, filter)
			if err != nil {
				return mapCloudTabsDBErr(err)
			}
			maybeStaleTabsHint(cmd, len(tabs), refresh)
			out := make([]map[string]any, 0, len(tabs))
			for _, t := range tabs {
				out = append(out, map[string]any{
					"device_name":       t.DeviceName,
					"device_type":       t.DeviceType,
					"title":             t.Title,
					"url":               t.URL,
					"last_viewed_time":  formatRFC3339(t.LastViewedTime),
					"is_pinned":         t.IsPinned,
					"is_showing_reader": t.IsShowingReader,
				})
			}
			output.DefaultToJSONIfNotTTY(&opts.Output)
			return output.Render(opts.Output, out)
		},
	}
	cmd.Flags().BoolVar(&summary, "summary", false, "per-device tab counts instead of per-tab rows (deterministic N-tabs-across-M-devices)")
	cmd.Flags().StringVar(&deviceName, "device-name", "", "filter to devices whose name contains this substring")
	cmd.Flags().BoolVar(&pinned, "pinned", false, "only pinned tabs")
	cmd.Flags().BoolVar(&refresh, "refresh", false, "activate Safari and wait --wait seconds before reading, so iCloud syncs the latest tabs (side effect: brings Safari to the foreground)")
	cmd.Flags().IntVar(&wait, "wait", 5, "seconds to wait after --refresh activates Safari")
	return cmd
}

// maybeStaleTabsHint writes a one-line stderr hint when a plain (non-refresh)
// read happens while Safari is not running. CloudTabs.db only updates while
// Safari is running, so a closed Safari can leave a non-empty but stale tab set.
// Written to stderr so it never pollutes stdout (which may be JSON); suppressed
// when --refresh was already used (it would have opened Safari).
func maybeStaleTabsHint(cmd *cobra.Command, n int, refresh bool) {
	if refresh {
		return
	}
	running, err := safariRunningFn()
	if err != nil || running {
		return
	}
	fmt.Fprintf(cmd.ErrOrStderr(), "Safari isn't running — these %d tab(s) are from Safari's last iCloud sync and may be stale; run --refresh to open Safari and read the current tabs\n", n)
}

func cloudTabsSummaryCount(rows []safariSource.CloudTabDeviceSummary) int {
	var n int64
	for _, r := range rows {
		n += r.TabCount
	}
	if n > int64(^uint(0)>>1) {
		return int(^uint(0) >> 1)
	}
	return int(n)
}

// formatRFC3339 renders a time as RFC3339, or "" for the zero value so a tab
// with no last_viewed_time is surfaced as empty rather than a 0001 date.
func formatRFC3339(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}
