// Copyright 2026 trevin-chow. Licensed under Apache-2.0. See LICENSE.

package dreows

import (
	"encoding/json"
	"testing"
)

func TestParseFrame_TopLevelFields(t *testing.T) {
	raw := []byte(`{"devicesn":"sn-1","method":"state","poweron":true,"windlevel":3}`)
	upd, ok := parseFrame(raw)
	if !ok {
		t.Fatalf("expected ok")
	}
	if upd.DeviceSn != "sn-1" {
		t.Fatalf("got sn=%q want sn-1", upd.DeviceSn)
	}
	if v, _ := upd.Fields["poweron"].(bool); !v {
		t.Errorf("poweron not true: %v", upd.Fields["poweron"])
	}
	if v, _ := upd.Fields["windlevel"].(float64); v != 3 {
		t.Errorf("windlevel=%v want 3", upd.Fields["windlevel"])
	}
	if _, isMethod := upd.Fields["method"]; isMethod {
		t.Errorf("method should be stripped from fields")
	}
}

func TestParseFrame_NestedReported(t *testing.T) {
	raw := []byte(`{"devicesn":"sn-2","reported":{"temperature":74,"humidity":40}}`)
	upd, ok := parseFrame(raw)
	if !ok {
		t.Fatalf("expected ok")
	}
	if upd.DeviceSn != "sn-2" {
		t.Fatalf("got sn=%q want sn-2", upd.DeviceSn)
	}
	if v, _ := upd.Fields["temperature"].(float64); v != 74 {
		t.Errorf("temperature=%v want 74", upd.Fields["temperature"])
	}
	if v, _ := upd.Fields["humidity"].(float64); v != 40 {
		t.Errorf("humidity=%v want 40", upd.Fields["humidity"])
	}
}

func TestParseFrame_AltSnKey(t *testing.T) {
	raw := []byte(`{"deviceSn":"sn-3","params":{"oscmode":1}}`)
	upd, ok := parseFrame(raw)
	if !ok {
		t.Fatalf("expected ok")
	}
	if upd.DeviceSn != "sn-3" {
		t.Errorf("got sn=%q want sn-3 (deviceSn alt key)", upd.DeviceSn)
	}
}

func TestParseFrame_Invalid(t *testing.T) {
	_, ok := parseFrame([]byte(`not json`))
	if ok {
		t.Errorf("expected !ok for invalid JSON")
	}
	// Empty envelope with no fields → rejected
	_, ok = parseFrame([]byte(`{}`))
	if ok {
		t.Errorf("expected !ok for empty envelope")
	}
}

func TestParseFrame_RawPreserved(t *testing.T) {
	in := []byte(`{"devicesn":"x","windlevel":5}`)
	upd, ok := parseFrame(in)
	if !ok {
		t.Fatalf("expected ok")
	}
	var got map[string]any
	if err := json.Unmarshal(upd.Raw, &got); err != nil {
		t.Fatalf("Raw not valid JSON: %v", err)
	}
	if got["devicesn"] != "x" {
		t.Errorf("Raw lost devicesn")
	}
}

func TestFirstString(t *testing.T) {
	m := map[string]any{"a": "", "b": "hit", "c": 5}
	v, ok := firstString(m, "a", "b", "c")
	if !ok || v != "hit" {
		t.Errorf("got (%q, %v) want (hit, true)", v, ok)
	}
	v, ok = firstString(m, "c")
	if ok {
		t.Errorf("non-string c should not match: %q", v)
	}
}
