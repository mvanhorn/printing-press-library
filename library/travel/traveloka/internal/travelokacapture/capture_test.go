package travelokacapture

import (
	"context"
	"strings"
	"testing"
)

func TestSourceURLsPreserveActualContext(t *testing.T) {
	tests := []struct {
		name string
		o    Options
		want string
		bad  bool
	}{{"two nights", Options{Market: "SG", Locale: "en-SG", Currency: "SGD", Depart: "2026-11-20", ReturnDate: "2026-11-27", CheckIn: "2027-01-06", CheckOut: "2027-01-08"}, ".2.1.HOTEL", false}, {"invalid calendar", Options{Market: "SG", Locale: "en-SG", Currency: "SGD", Depart: "2026-02-30", ReturnDate: "2026-11-27", CheckIn: "2027-01-06", CheckOut: "2027-01-08"}, "", true}, {"wrong locale region", Options{Market: "SG", Locale: "en-ID", Currency: "SGD", Depart: "2026-11-20", ReturnDate: "2026-11-27", CheckIn: "2027-01-06", CheckOut: "2027-01-08"}, "", true}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u, e := SourceURLs(tt.o)
			if tt.bad {
				if e == nil {
					t.Fatal("expected invalid source context error")
				}
				return
			}
			if e != nil || len(u) != 4 || !strings.Contains(u[3], tt.want) {
				t.Fatalf("source URLs lost context: %v %v", u, e)
			}
		})
	}
}
func TestBackendPathMissingDoesNotInstall(t *testing.T) {
	if _, e := BackendPath("/SIMULATED/not-installed-browser-use"); e == nil {
		t.Fatal("missing backend should require explicit installed dependency")
	}
}
func TestCaptureRejectsInputsBeforeOpeningBrowser(t *testing.T) {
	_, e := Capture(context.Background(), Options{Market: "SG", Locale: "en-SG", Currency: "SGD", Depart: "2026-11-20", ReturnDate: "2026-11-19", CheckIn: "2027-01-06", CheckOut: "2027-01-08"})
	if e == nil {
		t.Fatal("reversed travel dates accepted")
	}
}
