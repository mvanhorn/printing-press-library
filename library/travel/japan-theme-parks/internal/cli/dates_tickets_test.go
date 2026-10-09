package cli

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/japan-theme-parks/internal/parks"
)

func TestPrintSelectedTicketsShowsSelection(t *testing.T) {
	rows := []dateRow{{
		Date: "2026-11-02",
		Park: "tdl",
		Tickets: []ticketView{{
			TDRTicket:   parks.TDRTicket{ID: "T8", NameEN: "Weeknight Passport", Status: "on_sale", PricesYen: map[string]int{"adult": 6500}},
			SaleOpensAt: &saleRule{At: "2026-09-02T14:00:00+09:00"},
		}},
	}}
	var buf bytes.Buffer
	if err := printSelectedTickets(&buf, rows); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{"T8", "Weeknight Passport", "on_sale", "6500", "2026-09-02T14:00:00+09:00"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

func TestPrintSelectedTicketsNone(t *testing.T) {
	var buf bytes.Buffer
	if err := printSelectedTickets(&buf, []dateRow{{Date: "2026-11-02", Park: "usj"}}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "none") {
		t.Fatalf("expected a none line, got %q", buf.String())
	}
}

func TestPacingBudget(t *testing.T) {
	if got := pacingBudget(0, 3); got != 0 {
		t.Errorf("auto rate budget = %v, want 0", got)
	}
	if got := pacingBudget(-1, 3); got != 0 {
		t.Errorf("negative rate budget = %v, want 0", got)
	}
	if got := pacingBudget(0.1, 2); got != 20*time.Second {
		t.Errorf("0.1 rps x2 budget = %v, want 20s", got)
	}
}
