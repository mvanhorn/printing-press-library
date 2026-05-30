// Hand-authored — NOT generated. Tests the GISIS Ship Particulars HTML parser
// against a real authenticated sample (IMO 9866641 SIDER ABIDJAN, captured
// 2026-05-27). The fixture lives at testdata/gisis-ship-9866641.html.
package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseShipParticularsHTML_RealSample(t *testing.T) {
	body, err := os.ReadFile(filepath.Join("testdata", "gisis-ship-9866641.html"))
	if err != nil {
		t.Fatalf("loading fixture: %v", err)
	}

	ship, err := parseShipParticularsHTML(body, "9866641", "https://gisis.imo.org/Public/SHIPS/ShipDetails.aspx?IMONumber=9866641")
	if err != nil {
		t.Fatalf("parseShipParticularsHTML: %v", err)
	}

	cases := []struct {
		name, got, want string
	}{
		{"IMONumber", ship.IMONumber, "9866641"},
		{"Name", ship.Name, "SIDER ABIDJAN"},
		{"Flag", ship.Flag, "MAR (Portugal)"},
		{"CallSign", ship.CallSign, "CQ2842"},
		{"MMSI", ship.MMSI, "255916593"},
		{"ShipUNSanction", ship.ShipUNSanction, "Not on list"},
		{"OwningEntityUNSanction", ship.OwningEntityUNSanction, "Not on list"},
		{"ShipType", ship.ShipType, "General Cargo Ship (Open Hatch)"},
		{"DateOfBuild", ship.DateOfBuild, "2020-01"},
		{"RegisteredOwner", ship.RegisteredOwner, "KIVIK SHIPPING LTD"},
		{"RegisteredOwnerIMOCompanyNum", ship.RegisteredOwnerIMOCompanyNum, "6101937"},
		{"AuthenticatedAs", ship.AuthenticatedAs, "Henning Fotland"},
		{"LastUpdated", ship.LastUpdated, "2026-05-26"},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("%s: got %q want %q", c.name, c.got, c.want)
		}
	}

	if ship.GrossTonnage != 23232 {
		t.Errorf("GrossTonnage: got %d want 23232", ship.GrossTonnage)
	}

	// History tables. Flag history is the clearest flag-hop signal — 4 distinct
	// flags within 6 years (Singapore -> Marshall Islands -> Panama -> Portugal).
	if len(ship.FlagHistory) < 4 {
		t.Errorf("FlagHistory: got %d entries, want >= 4 (textbook flag-hop pattern)", len(ship.FlagHistory))
	}
	wantFlags := map[string]string{
		"MAR (Portugal)":   "2025-12",
		"Panama":           "2025-05",
		"Marshall Islands": "2025-02",
		"Singapore":        "2020-01",
	}
	for _, fh := range ship.FlagHistory {
		if want, ok := wantFlags[fh.Value]; ok {
			if fh.Effective != want {
				t.Errorf("FlagHistory[%s]: effective got %q want %q", fh.Value, fh.Effective, want)
			}
			delete(wantFlags, fh.Value)
		}
	}
	if len(wantFlags) > 0 {
		t.Errorf("FlagHistory missing entries: %v", wantFlags)
	}

	// Name history — the vessel was renamed (Nord Abidjan -> SIDER ABIDJAN in 2025-05).
	if len(ship.NameHistory) < 2 {
		t.Errorf("NameHistory: got %d entries, want >= 2", len(ship.NameHistory))
	}

	// Registered-owner history. The owner table interleaves a top-level entry
	// (owner name + effective date) with indented company-detail sub-rows; only
	// the entry rows are ownership changes. Regression guard: an earlier version
	// emitted the company-detail field labels ("IMO Company Number", "Address",
	// "Company status", "Nationality of registration") as bogus history entries.
	if len(ship.RegisteredOwnerHistory) != 1 {
		t.Errorf("RegisteredOwnerHistory: got %d entries, want 1", len(ship.RegisteredOwnerHistory))
	}
	for _, oh := range ship.RegisteredOwnerHistory {
		switch oh.Value {
		case "IMO Company Number", "Nationality of registration", "Address", "Company status":
			t.Errorf("RegisteredOwnerHistory leaked a company-detail label as an entry: %q", oh.Value)
		}
	}
	if len(ship.RegisteredOwnerHistory) > 0 {
		if got := ship.RegisteredOwnerHistory[0]; got.Value != "Kivik Shipping Ltd" || got.Effective != "2025-12-15" {
			t.Errorf("RegisteredOwnerHistory[0]: got %+v, want {Value:\"Kivik Shipping Ltd\" Effective:\"2025-12-15\"}", got)
		}
	}

	// SourceURL and FetchedAt should be populated.
	if ship.SourceURL == "" {
		t.Error("SourceURL: empty, want populated")
	}
	if ship.FetchedAt == "" {
		t.Error("FetchedAt: empty, want populated")
	}
}

func TestIsLoginWall_ObjectMoved(t *testing.T) {
	body := []byte(`<html><head><title>Object moved</title></head><body>
<h2>Object moved to <a href="/Public/Shared/Public/Login.aspx?ReturnUrl=%2fPublic%2fSHIPS%2fShipDetails.aspx%3fIMONumber%3d9966233">here</a>.</h2>
</body></html>`)
	_, err := parseShipParticularsHTML(body, "9966233", "https://gisis.imo.org/Public/SHIPS/ShipDetails.aspx?IMONumber=9966233")
	if err == nil {
		t.Fatal("expected errLoginWall, got nil")
	}
	if err.Error() != errLoginWall.Error() {
		t.Errorf("expected errLoginWall, got %q", err.Error())
	}
}
