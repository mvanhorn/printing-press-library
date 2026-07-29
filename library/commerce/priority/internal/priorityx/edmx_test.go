// Copyright 2026 alon-auto and contributors. Licensed under Apache-2.0. See LICENSE.

package priorityx

import (
	"testing"
	"time"
)

const sampleEDMX = `<?xml version="1.0" encoding="utf-8"?>
<edmx:Edmx xmlns:edmx="http://docs.oasis-open.org/odata/ns/edmx" Version="4.0">
  <edmx:DataServices>
    <Schema xmlns="http://docs.oasis-open.org/odata/ns/edm" Namespace="Priority.OData">
      <EntityType Name="ORDERS">
        <Property Name="ORDNAME" Type="Edm.String">
          <Annotation Term="Priority.OData.Mandatory" Bool="true"/>
        </Property>
        <Property Name="QPRICE" Type="Edm.Decimal"/>
        <NavigationProperty Name="ORDERITEMS_SUBFORM" Type="Collection(Priority.OData.ORDERITEMS)"/>
        <NavigationProperty Name="SHIPTO2_SUBFORM" Type="Priority.OData.SHIPTO2"/>
      </EntityType>
      <EntityType Name="CUSTOMERS">
        <Property Name="CUSTNAME" Type="Edm.String">
          <Annotation Term="Priority.OData.Description" String="Customer Number"/>
        </Property>
      </EntityType>
    </Schema>
  </edmx:DataServices>
</edmx:Edmx>`

func TestParseEDMX(t *testing.T) {
	tests := []struct {
		name      string
		raw       string
		wantForms int
		wantErr   bool
	}{
		{"sample", sampleEDMX, 2, false},
		{"invalid xml", "<not-edmx", 0, true},
		{"empty schema", `<edmx:Edmx xmlns:edmx="http://docs.oasis-open.org/odata/ns/edmx"><edmx:DataServices><Schema xmlns="http://docs.oasis-open.org/odata/ns/edm"/></edmx:DataServices></edmx:Edmx>`, 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			forms, err := ParseEDMX([]byte(tt.raw))
			if (err != nil) != tt.wantErr {
				t.Fatalf("ParseEDMX error = %v, wantErr %v", err, tt.wantErr)
			}
			if len(forms) != tt.wantForms {
				t.Fatalf("got %d forms, want %d", len(forms), tt.wantForms)
			}
		})
	}
}

func TestParseEDMXDetails(t *testing.T) {
	forms, err := ParseEDMX([]byte(sampleEDMX))
	if err != nil {
		t.Fatal(err)
	}
	// Sorted: CUSTOMERS, ORDERS.
	orders := forms[1]
	if orders.Name != "ORDERS" {
		t.Fatalf("expected ORDERS, got %s", orders.Name)
	}
	if !orders.Fields[0].Mandatory {
		t.Error("ORDNAME should be mandatory")
	}
	if orders.Fields[1].Type != "Decimal" {
		t.Errorf("QPRICE type = %s, want Decimal", orders.Fields[1].Type)
	}
	if !orders.Subforms[0].Collection || orders.Subforms[0].Target != "ORDERITEMS" {
		t.Errorf("ORDERITEMS_SUBFORM parsed wrong: %+v", orders.Subforms[0])
	}
	if orders.Subforms[1].Collection {
		t.Error("SHIPTO2_SUBFORM should be non-collection")
	}
	customers := forms[0]
	if customers.Fields[0].Description != "Customer Number" {
		t.Errorf("CUSTNAME description = %q", customers.Fields[0].Description)
	}
}

func TestDiffSchemas(t *testing.T) {
	baseline := []Form{
		{Name: "ORDERS", Fields: []Field{{Name: "ORDNAME", Type: "String"}, {Name: "OLD", Type: "String"}}},
		{Name: "GONE", Fields: []Field{{Name: "X", Type: "String"}}},
	}
	current := []Form{
		{Name: "ORDERS", Fields: []Field{{Name: "ORDNAME", Type: "String", Mandatory: true}, {Name: "NEW", Type: "String"}}},
		{Name: "ADDED", Fields: []Field{{Name: "Y", Type: "String"}}},
	}
	d := DiffSchemas(baseline, current)
	if len(d.AddedForms) != 1 || d.AddedForms[0] != "ADDED" {
		t.Errorf("AddedForms = %v", d.AddedForms)
	}
	if len(d.RemovedForms) != 1 || d.RemovedForms[0] != "GONE" {
		t.Errorf("RemovedForms = %v", d.RemovedForms)
	}
	if got := d.AddedFields["ORDERS"]; len(got) != 1 || got[0] != "NEW" {
		t.Errorf("AddedFields = %v", d.AddedFields)
	}
	if got := d.RemovedFields["ORDERS"]; len(got) != 1 || got[0] != "OLD" {
		t.Errorf("RemovedFields = %v", d.RemovedFields)
	}
	if got := d.ChangedFields["ORDERS"]; len(got) != 1 || got[0] != "ORDNAME" {
		t.Errorf("ChangedFields = %v", d.ChangedFields)
	}
	if d.Empty() {
		t.Error("diff should not be empty")
	}
	if !DiffSchemas(current, current).Empty() {
		t.Error("self-diff should be empty")
	}
}

func TestBucketFor(t *testing.T) {
	now := time.Date(2026, 7, 29, 0, 0, 0, 0, time.UTC)
	tests := []struct {
		date string
		want int
	}{
		{"2026-07-20T00:00:00Z", 0},
		{"2026-06-10T00:00:00Z", 1},
		{"2026-05-05T00:00:00Z", 2},
		{"2025-01-01T00:00:00Z", 3},
		{"2026-08-15T00:00:00Z", -1},
		{"2026-07-20", 0},
		{"garbage", -1},
	}
	for _, tt := range tests {
		if got := BucketFor(tt.date, now); got != tt.want {
			t.Errorf("BucketFor(%q) = %d, want %d", tt.date, got, tt.want)
		}
	}
}
