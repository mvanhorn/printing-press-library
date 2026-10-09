// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/travel/japan-timed-tickets/internal/tickets"
)

func TestRenderAvailabilityHumanKeepsUnknownSlotsApart(t *testing.T) {
	rows := []availabilityRow{{
		Sight:     "shibuya-sky",
		Date:      "2026-11-02",
		Status:    tickets.DayAvailable,
		SlotsRead: true,
		Slots: []tickets.Slot{
			{Start: "10:00", Status: tickets.SlotAvailable},
			{Start: "11:00", Status: tickets.SlotFew},
			{Start: "12:00", Status: tickets.SlotSoldOut},
			{Start: "13:00", Status: tickets.SlotUnknown},
		},
	}}
	var out bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	if err := renderAvailabilityHuman(cmd, rows, nil); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	if !strings.Contains(got, "2 open (10:00...11:00), 1 unknown") {
		t.Fatalf("unexpected slot summary:\n%s", got)
	}
}
