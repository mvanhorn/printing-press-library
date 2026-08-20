// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/health/vagaro/internal/vagaro"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSlotOpen(t *testing.T) {
	groups := []vagaro.SlotGroup{
		{Date: "24 Jul 2026", Provider: "Ronnel", Times: []string{"10:00 AM", "10:15 AM"}},
		{Date: "25 Jul 2026", Provider: "Ronnel", Times: []string{"09:00 AM"}},
	}
	at := time.Date(2026, 7, 24, 10, 0, 0, 0, time.UTC)
	open, sameDay := slotOpen(groups, at, "10:00 AM", "")
	assert.True(t, open)
	assert.Equal(t, []string{"10:00 AM", "10:15 AM"}, sameDay)

	// Requested time not present that day.
	at2 := time.Date(2026, 7, 24, 12, 30, 0, 0, time.UTC)
	open2, sameDay2 := slotOpen(groups, at2, "12:30 PM", "")
	assert.False(t, open2)
	assert.Equal(t, []string{"10:00 AM", "10:15 AM"}, sameDay2)

	// Different day, no match.
	at3 := time.Date(2026, 7, 26, 10, 0, 0, 0, time.UTC)
	open3, _ := slotOpen(groups, at3, "10:00 AM", "")
	assert.False(t, open3)
}

func TestSlotOpen_datelessFallback(t *testing.T) {
	// HTML-fragment fallback groups carry no date; match on the clock label.
	groups := []vagaro.SlotGroup{{Times: []string{"10:00 AM", "1:15 PM"}}}
	at := time.Date(2026, 7, 24, 10, 0, 0, 0, time.UTC)
	open, _ := slotOpen(groups, at, "10:00 AM", "")
	assert.True(t, open)
}

func TestBookLabel(t *testing.T) {
	res := bookResult{
		ServiceName: "Skin Fade", ProviderName: "Ronnel Getz", BusinessName: "Central Barber",
		At: "Fri Jul 24 10:00 AM",
	}
	assert.Equal(t, "Skin Fade with Ronnel Getz at Central Barber on Fri Jul 24 10:00 AM", bookLabel(res))

	// Falls back to ids/slug when names are missing.
	res2 := bookResult{ServiceID: "34098477", ProviderID: "43931725", Slug: "centralbarber", At: "Fri Jul 24 10:00 AM"}
	assert.Equal(t, "service 34098477 with provider 43931725 at centralbarber on Fri Jul 24 10:00 AM", bookLabel(res2))
}

func TestBookVerifyShortCircuit(t *testing.T) {
	t.Setenv("PRINTING_PRESS_VERIFY", "1")
	cmd := RootCmd()
	var out, errBuf writerBuf
	cmd.SetOut(&out)
	cmd.SetErr(&errBuf)
	cmd.SetArgs([]string{"book", "centralbarber", "--service", "34098477", "--provider", "43931725", "--at", "2026-07-24T10:00", "--confirm", "--json"})
	require.NoError(t, cmd.Execute())
	got := out.String()
	assert.Contains(t, got, "would book")
	// Must NOT have attempted a network mutation or emitted a book-now URL.
	assert.NotContains(t, got, "book-now")
}

// writerBuf is a minimal io.Writer capturing output for command tests.
type writerBuf struct{ b []byte }

func (w *writerBuf) Write(p []byte) (int, error) { w.b = append(w.b, p...); return len(p), nil }
func (w *writerBuf) String() string              { return string(w.b) }

// TestSlotOpenProviderAttribution verifies that a slot group explicitly
// attributed to a different provider is not treated as availability for the
// requested provider (regression for the "Fallback Loses Providers" finding),
// while unattributed groups remain matchable for a provider-scoped call.
func TestSlotOpenProviderAttribution(t *testing.T) {
	at, _ := time.ParseInLocation(atLayout, "2026-07-24T10:00", time.UTC)

	// A group attributed to provider B must NOT satisfy a request for provider A.
	otherProvider := []vagaro.SlotGroup{{Date: "24 Jul 2026", ProviderID: "B", Times: []string{"10:00 AM"}}}
	if open, _ := slotOpen(otherProvider, at, "10:00 AM", "A"); open {
		t.Fatalf("slot attributed to provider B matched a request for provider A")
	}

	// The same group DOES satisfy a request for provider B.
	if open, _ := slotOpen(otherProvider, at, "10:00 AM", "B"); !open {
		t.Fatalf("slot attributed to provider B did not match a request for provider B")
	}

	// An unattributed group (empty ProviderID) is matchable for a provider-scoped
	// call, because the availability request was already scoped to that provider.
	unattributed := []vagaro.SlotGroup{{Date: "24 Jul 2026", Times: []string{"10:00 AM"}}}
	if open, _ := slotOpen(unattributed, at, "10:00 AM", "A"); !open {
		t.Fatalf("unattributed slot did not match a provider-scoped request")
	}
}
