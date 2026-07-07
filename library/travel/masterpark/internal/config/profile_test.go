package config

import (
	"encoding/json"
	"testing"
)

func TestProfileFromLoginDataNested(t *testing.T) {
	// Shape where customer fields are nested under "customer" and vehicles is
	// a sibling array (one plausible verifyLogin response).
	data := json.RawMessage(`{
		"customer": {
			"first_name": "Alice",
			"last_name": "Smith",
			"email": "alice@example.com",
			"phone": "phone-test",
			"id": "C123"
		},
		"vehicles": [
			{"make":"Honda","model":"Civic","color":"Blue","license":"ABC123","state":"WA","type":"standard"},
			{"make":"Toyota","model":"Tacoma","license":"XYZ789","type":"oversize"}
		]
	}`)
	p := ProfileFromLoginData(data)
	if p == nil {
		t.Fatal("expected a profile, got nil")
	}
	if p.FirstName != "Alice" || p.LastName != "Smith" || p.Email != "alice@example.com" || p.Phone != "phone-test" {
		t.Errorf("customer fields mismatch: %+v", p)
	}
	if p.CustomerID != "C123" {
		t.Errorf("customer id = %q", p.CustomerID)
	}
	if len(p.Vehicles) != 2 {
		t.Fatalf("want 2 vehicles, got %d: %+v", len(p.Vehicles), p.Vehicles)
	}
	if p.Vehicles[0].Make != "Honda" || p.Vehicles[0].License != "ABC123" || p.Vehicles[0].State != "WA" {
		t.Errorf("vehicle[0] = %+v", p.Vehicles[0])
	}
	if p.Vehicles[1].Model != "Tacoma" || p.Vehicles[1].Type != "oversize" {
		t.Errorf("vehicle[1] = %+v", p.Vehicles[1])
	}
}

func TestProfileFromLoginDataFlat(t *testing.T) {
	// Shape where customer fields are at the top level and a "plate" alias is
	// used for the license.
	data := json.RawMessage(`{
		"first_name": "Bob",
		"lastName": "Jones",
		"email": "bob@example.com",
		"phone": "5095559876",
		"customer_id": 42,
		"vehicles": [{"make":"Ford","model":"F150","plate":"PLT1","type":"standard"}]
	}`)
	p := ProfileFromLoginData(data)
	if p == nil {
		t.Fatal("expected a profile, got nil")
	}
	if p.FirstName != "Bob" || p.LastName != "Jones" {
		t.Errorf("name mismatch: %+v", p)
	}
	if p.CustomerID != "42" {
		t.Errorf("numeric customer id should stringify: %q", p.CustomerID)
	}
	if len(p.Vehicles) != 1 || p.Vehicles[0].License != "PLT1" {
		t.Errorf("vehicle plate alias not honored: %+v", p.Vehicles)
	}
}

func TestProfileFromLoginDataEmpty(t *testing.T) {
	if p := ProfileFromLoginData(json.RawMessage(`{"customer":123}`)); p != nil {
		t.Errorf("expected nil profile for data without usable fields, got %+v", p)
	}
	if p := ProfileFromLoginData(nil); p != nil {
		t.Errorf("expected nil profile for empty data, got %+v", p)
	}
	if p := ProfileFromLoginData(json.RawMessage(`not json`)); p != nil {
		t.Errorf("expected nil profile for invalid json, got %+v", p)
	}
}
