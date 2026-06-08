// Copyright 2026 "Chris Carson" and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"sort"
	"strconv"
	"strings"
	"testing"
)

func TestPatchGapsSortOrder(t *testing.T) {
	rows := []patchGapRow{
		{OrganizationName: "Beta", DeviceID: 2, KBNumber: "KB9"},
		{OrganizationName: "Acme", DeviceID: 5, KBNumber: "KB2"},
		{OrganizationName: "Acme", DeviceID: 5, KBNumber: "KB1"},
		{OrganizationName: "Acme", DeviceID: 3, KBNumber: "KB7"},
	}
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].OrganizationName != rows[j].OrganizationName {
			return rows[i].OrganizationName < rows[j].OrganizationName
		}
		if rows[i].DeviceID != rows[j].DeviceID {
			return rows[i].DeviceID < rows[j].DeviceID
		}
		return rows[i].KBNumber < rows[j].KBNumber
	})
	want := []string{"Acme/3/KB7", "Acme/5/KB1", "Acme/5/KB2", "Beta/2/KB9"}
	for i, w := range want {
		got := rows[i].OrganizationName + "/" + strconv.FormatInt(rows[i].DeviceID, 10) + "/" + rows[i].KBNumber
		if got != w {
			t.Fatalf("row %d = %s, want %s", i, got, w)
		}
	}
}

func TestPatchGapsDryRun(t *testing.T) {
	out, err := runNovelDryRun(t, newNovelPatchGapsCmd, "--type", "all")
	if err != nil {
		t.Fatalf("dry-run err: %v", err)
	}
	if !strings.Contains(out, "would") {
		t.Fatalf("dry-run output missing 'would': %q", out)
	}
}
