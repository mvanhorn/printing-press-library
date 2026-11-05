package cli

import "testing"

func TestItemStatusLine(t *testing.T) {
	cases := []struct {
		size, price, perUnit, availability, label, want string
	}{
		{"1.5 kg", "$20.99", "$1.40/100g", "available", "Many in stock", "1.5 kg  $20.99 ($1.40/100g)  in stock (Many in stock)"},
		{"2 lb", "$16.99", "", "unavailable", "Out of stock", "2 lb  $16.99  UNAVAILABLE (Out of stock)"},
		{"400 g", "", "", "low_stock", "Likely out of stock", "400 g  low stock (Likely out of stock)"},
		{"", "", "", "unknown", "", "availability unknown"},
		{"", "", "", "", "", ""},
	}
	for _, c := range cases {
		if got := itemStatusLine(c.size, c.price, c.perUnit, c.availability, c.label); got != c.want {
			t.Errorf("got %q want %q", got, c.want)
		}
	}
}
