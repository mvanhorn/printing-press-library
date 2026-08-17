// Copyright 2026 Omar Shahine and contributors. Licensed under Apache-2.0. See LICENSE.

package faaparse

import (
	"os"
	"strings"
	"testing"
)

func load(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return b
}

func TestParseDetailExample(t *testing.T) {
	// detail_example.html is a synthetic co-owned-aircraft page (real FAA
	// devkit markup, placeholder owner names/tail/serial) so the parser is
	// exercised against the multi-owner + temporary-certificate shape without
	// shipping any real registrant data.
	d, err := ParseDetail(load(t, "detail_example.html"))
	if err != nil {
		t.Fatalf("ParseDetail: %v", err)
	}
	if d.NNumber != "N101EX" {
		t.Errorf("NNumber = %q, want N101EX", d.NNumber)
	}
	if d.Status != "Assigned" {
		t.Errorf("Status = %q, want Assigned", d.Status)
	}
	if got := d.Description["serial_number"]; got != "000-0001" {
		t.Errorf("serial_number = %q, want 000-0001", got)
	}
	if got := d.Description["manufacturer_name"]; got != "TEXTRON AVIATION INC" {
		t.Errorf("manufacturer_name = %q, want TEXTRON AVIATION INC", got)
	}
	if got := d.Description["model"]; got != "560XL" {
		t.Errorf("model = %q, want 560XL", got)
	}
	if got := d.Description["mode_s_code_base_16_hex"]; got != "A00801" {
		t.Errorf("mode_s hex = %q, want A00801", got)
	}
	if got := d.Owner["name"]; got != "EXAMPLE AVIATION LLC" {
		t.Errorf("owner name = %q, want EXAMPLE AVIATION LLC", got)
	}
	if len(d.OtherOwnerNames) < 10 {
		t.Errorf("OtherOwnerNames = %d entries, want >= 10", len(d.OtherOwnerNames))
	}
	foundCoOwner := false
	for _, n := range d.OtherOwnerNames {
		if strings.Contains(n, "EXAMPLE OWNER") {
			foundCoOwner = true
		}
	}
	if !foundCoOwner {
		t.Errorf("OtherOwnerNames missing co-owner entries; got %v", d.OtherOwnerNames)
	}
	if len(d.TemporaryCertificates) != 1 {
		t.Fatalf("TemporaryCertificates = %d, want 1", len(d.TemporaryCertificates))
	}
	if got := d.TemporaryCertificates[0]["certificate_number"]; got != "T000001" {
		t.Errorf("temp cert number = %q, want T000001", got)
	}
	if d.Airworthiness == nil {
		t.Fatalf("Airworthiness section missing")
	}
	if _, ok := d.Airworthiness["type_certificate_data_sheet"]; !ok {
		t.Errorf("Airworthiness missing type_certificate_data_sheet: %v", d.Airworthiness)
	}
	if len(d.FuelModifications) != 0 {
		t.Errorf("FuelModifications = %v, want empty (page says None)", d.FuelModifications)
	}
}

func TestParseListNameSearch(t *testing.T) {
	l, err := ParseList(load(t, "list_name_delta.html"))
	if err != nil {
		t.Fatalf("ParseList: %v", err)
	}
	if l.Total != 1558 {
		t.Errorf("Total = %d, want 1558", l.Total)
	}
	if l.Page != 1 || l.Pages != 32 {
		t.Errorf("Page/Pages = %d/%d, want 1/32", l.Page, l.Pages)
	}
	if len(l.Rows) != 50 {
		t.Errorf("Rows = %d, want 50", len(l.Rows))
	}
	found := false
	for _, r := range l.Rows {
		if r["n_number"] == "101DQ" {
			found = true
			if !strings.Contains(r["name_address"], "DELTA AIR LINES") {
				t.Errorf("row 101DQ name_address = %q", r["name_address"])
			}
		}
	}
	if !found {
		t.Errorf("row with n_number 101DQ not found")
	}
}

func TestParseListMakeModel(t *testing.T) {
	l, err := ParseList(load(t, "list_makemodel_sr22.html"))
	if err != nil {
		t.Fatalf("ParseList: %v", err)
	}
	if l.Total != 2 {
		t.Errorf("Total = %d, want 2", l.Total)
	}
	if len(l.Rows) != 2 {
		t.Fatalf("Rows = %d, want 2", len(l.Rows))
	}
	if got := l.Rows[0]["manufacturer_name"]; got != "CIRRUS DESIGN CORP" && !strings.Contains(got, "CIRRUS") {
		t.Errorf("manufacturer = %q, want CIRRUS*", got)
	}
}

func TestParseAutoKinds(t *testing.T) {
	r, err := ParseAuto(load(t, "detail_example.html"))
	if err != nil || r.Kind != "detail" {
		t.Errorf("detail fixture: kind=%v err=%v", r.Kind, err)
	}
	r, err = ParseAuto(load(t, "serial_17280005.html"))
	if err != nil || r.Kind != "list" {
		t.Errorf("serial fixture: kind=%v err=%v", r.Kind, err)
	}
	if r.List == nil || len(r.List.Rows) != 1 {
		t.Errorf("serial fixture rows = %+v, want 1 row", r.List)
	}
}

func TestSnake(t *testing.T) {
	cases := map[string]string{
		"Mode S Code (Base 16 / Hex)": "mode_s_code_base_16_hex",
		"A/W Date":                    "a_w_date",
		"Serial Number":               "serial_number",
		"# of Engines":                "of_engines",
	}
	for in, want := range cases {
		if got := snake(in); got != want {
			t.Errorf("snake(%q) = %q, want %q", in, got, want)
		}
	}
}
