package cli

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/devices/unifi/internal/client"
	"github.com/mvanhorn/printing-press-library/library/devices/unifi/internal/store"
)

// The API's two voucher deletes were generated with their names crossed, so the name
// that reads as "one voucher" ran the filter-based bulk delete. Pin each name to its
// shape: a regression here makes an agent delete more than it asked to.
func TestVoucherDeleteNamesMatchWhatTheyDelete(t *testing.T) {
	flags := &rootFlags{}
	single := newNetworkDeleteHotspotVoucherCmd(flags)
	if !strings.HasPrefix(single.Use, "delete-hotspot-voucher ") || !strings.Contains(single.Use, "<voucherId>") {
		t.Fatalf("singular command must take a voucher id, got Use %q", single.Use)
	}
	if !strings.Contains(single.Short, "specific") {
		t.Fatalf("singular command must be the single delete, got %q", single.Short)
	}
	bulk := newNetworkDeleteHotspotVouchersCmd(flags)
	if !strings.HasPrefix(bulk.Use, "delete-hotspot-vouchers ") || strings.Contains(bulk.Use, "<voucherId>") {
		t.Fatalf("plural command must be the bulk delete, got Use %q", bulk.Use)
	}
	if bulk.Flags().Lookup("filter") == nil {
		t.Fatal("bulk delete must select by --filter")
	}
}

func TestZoneFirewallNotConfiguredIsASyncWarning(t *testing.T) {
	err := &client.APIError{StatusCode: 400, Body: `{"statusCode":400,"code":"api.firewall.zone-based-firewall-not-configured","message":"Zone Based Firewall is not configured"}`}
	w, ok := isSyncAccessWarning(err)
	if !ok || w.Reason != "feature_not_configured" {
		t.Fatalf("want feature_not_configured warning, got %+v ok=%v", w, ok)
	}
	// Other 400s must stay errors, or a real broken request would be hidden as a warning.
	if _, ok := isSyncAccessWarning(&client.APIError{StatusCode: 400, Body: `{"code":"api.request.invalid"}`}); ok {
		t.Fatal("an unrelated 400 must not become a warning")
	}
}

func TestNoZonesReasonDistinguishesSyncedFromNeverSynced(t *testing.T) {
	ctx := context.Background()
	db, err := store.OpenWithContext(ctx, filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if got := noZonesReason(ctx, db, "C1"); !strings.Contains(got, "run: unifi-pp-cli sync") {
		t.Fatalf("never synced should point at sync, got %q", got)
	}
	if _, err := db.SaveSnapshot(ctx, "C1", time.Now(), nil, nil, nil); err != nil {
		t.Fatal(err)
	}
	got := noZonesReason(ctx, db, "C1")
	if strings.Contains(got, "run: unifi-pp-cli sync") || !strings.Contains(got, "legacy firewall") {
		t.Fatalf("after a sync the reason must name legacy firewall, got %q", got)
	}
}
