// Copyright 2026 rob-coco. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"strings"
	"testing"
)

func TestResolveDeviceByName(t *testing.T) {
	devices := []deviceRow{
		{ID: "p", Name: "iPhone", Type: "Smartphone", Source: "live"},
		{ID: "lre", Name: "Living Room Echo", Type: "Speaker", Source: "live"},
		{ID: "os", Name: "Office Speaker", Type: "Speaker", Source: "live"},
		{ID: "br", Name: "Bedroom", Type: "Speaker", Source: "cache"},
		{ID: "kd", Name: "Kitchen Display", Type: "Speaker", Source: "live"},
	}

	tests := []struct {
		name    string
		query   string
		wantID  string
		wantErr string
		devices []deviceRow
	}{
		{name: "exact match (case-insensitive)", query: "iphone", wantID: "p", devices: devices},
		{name: "prefix match", query: "living", wantID: "lre", devices: devices},
		// "echo" appears only in "Living Room Echo" so the substring is unique.
		{name: "substring match", query: "echo", wantID: "lre", devices: devices},
		// "room" appears in "Living Room Echo" and "Bedroom" — two substring hits.
		{name: "ambiguous substring 'room'", query: "room", wantErr: "ambiguous", devices: devices},
		// Even though "bedroom" substring-matches "Bedroom", the exact-match bucket
		// (case-insensitive) wins over the substring bucket.
		{name: "ambiguous-then-exact wins: 'bedroom' beats 'room' bucket", query: "bedroom", wantID: "br", devices: devices},
		{name: "no match", query: "garage", wantErr: "no device matches", devices: devices},
		{name: "empty device list", query: "anything", wantErr: "no devices known", devices: nil},
		{name: "exact beats prefix and substring", query: "iPhone", wantID: "p", devices: devices},
		{name: "case-insensitive exact", query: "LIVING ROOM ECHO", wantID: "lre", devices: devices},
		// "office" prefix-matches "Office Speaker" alone, so it resolves there.
		{name: "unique prefix", query: "office", wantID: "os", devices: devices},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := resolveDeviceByName(tc.query, tc.devices)
			if tc.wantErr != "" {
				if err == nil {
					t.Fatalf("want error containing %q, got success with id=%q", tc.wantErr, got.ID)
				}
				if !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("error %q does not contain %q", err.Error(), tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.ID != tc.wantID {
				t.Fatalf("want id=%q, got id=%q (name=%q)", tc.wantID, got.ID, got.Name)
			}
		})
	}
}

func TestWakeHintFor(t *testing.T) {
	tests := []struct {
		devType string
		wantSub string
	}{
		{"Smartphone", "open the Spotify app on this device"},
		{"smartphone", "open the Spotify app on this device"},
		{"Tablet", "open the Spotify app on this device"},
		{"Computer", "open Spotify on this computer"},
		{"Speaker", "Alexa/Sonos"},
		{"AVR", "Alexa/Sonos"},
		{"CastVideo", "Alexa/Sonos"},
		{"UnknownThing", "bring the device online"},
	}
	for _, tc := range tests {
		t.Run(tc.devType, func(t *testing.T) {
			got := wakeHintFor(tc.devType)
			if !strings.Contains(got, tc.wantSub) {
				t.Fatalf("wakeHintFor(%q) = %q; want substring %q", tc.devType, got, tc.wantSub)
			}
		})
	}
}
