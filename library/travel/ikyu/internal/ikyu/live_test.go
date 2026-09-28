package ikyu

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"
)

func TestLiveCoreQueries(t *testing.T) {
	if os.Getenv("IKYU_LIVE_TEST") != "1" {
		t.Skip("explicit live opt-in")
	}
	c, err := NewClient(Options{})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	in := os.Getenv("IKYU_LIVE_CHECK_IN")
	out := os.Getenv("IKYU_LIVE_CHECK_OUT")
	if in == "" {
		in = time.Now().In(japan).AddDate(0, 0, 7).Format("2006-01-02")
	}
	if out == "" {
		date, e := time.Parse("2006-01-02", in)
		if e != nil {
			t.Fatal(e)
		}
		out = date.AddDate(0, 0, 1).Format("2006-01-02")
	}
	s := Stay{CheckIn: in, CheckOut: out, Adults: 2, Rooms: 1}
	p, err := c.Property(ctx, "00002889")
	if err != nil {
		t.Fatal("property", err)
	}
	r, err := c.Rooms(ctx, RoomsRequest{PropertyID: "00002889", Stay: s, Limit: 2})
	if err != nil {
		t.Fatal("rooms", err)
	}
	o, err := c.Offer(ctx, OfferRequest{PropertyID: "00002889", RoomID: "10193741", PlanID: "11055986", Stay: s})
	if err != nil {
		t.Fatal("offer", err)
	}
	var quote *int64
	if o.Data.Offer.Price != nil {
		quote = o.Data.Offer.Price.Amount
	}
	b, _ := json.Marshal(map[string]any{"property": p.Data.Name, "rooms": len(r.Data), "total": r.Pagination.Total, "offer_amount": quote, "stats": c.Stats()})
	t.Log(string(b))
}
