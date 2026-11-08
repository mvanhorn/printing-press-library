package consumer

import (
	_ "embed"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/commerce/spothero/internal/session"
)

//go:embed testdata/users_me_live_shape.json
var usersMeFixtureAccount []byte

//go:embed testdata/users_vehicles_live_shape.json
var usersVehiclesFixtureAccount []byte

//go:embed testdata/users_credit_cards_live_shape.json
var usersCreditCardsFixtureAccount []byte

func TestParseUserAccountLiveShape(t *testing.T) {
	acct, err := parseUserAccount(usersMeFixtureAccount)
	if err != nil {
		t.Fatal(err)
	}
	if acct.ID != 42 || acct.Email != "user@example.com" || acct.PhoneNumber != "+14155550100" {
		t.Fatalf("%+v", acct)
	}
	if len(acct.CreditCards) != 0 || acct.DefaultCardExternalID != "" {
		t.Fatalf("live /users/me/ should not embed cards: %+v", acct)
	}
}

func TestParseCreditCardsLiveShape(t *testing.T) {
	cards, err := parseCreditCards(usersCreditCardsFixtureAccount)
	if err != nil {
		t.Fatal(err)
	}
	if len(cards) != 1 {
		t.Fatalf("expected 1 non-deleted card, got %+v", cards)
	}
	if cards[0].CardID != 10000001 || cards[0].CardLast4 != "4242" || !cards[0].IsDefault {
		t.Fatalf("%+v", cards[0])
	}
	if cards[0].CardExternalID != "00000000-0000-4000-8000-000000000001" {
		t.Fatalf("external=%q", cards[0].CardExternalID)
	}
}

func TestEnrichCreditCardsFetchesWhenMeOmitsCards(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/users/42/credit-cards/":
			_, _ = w.Write(envelopeData(json.RawMessage(usersCreditCardsFixtureAccount)))
		default:
			t.Fatalf("unexpected %s", r.URL.Path)
		}
	}))
	t.Cleanup(srv.Close)

	c := New(&session.Session{Cookies: map[string]string{"sessionid": "abc"}})
	c.BaseURL = srv.URL
	c.HTTP = srv.Client()

	me := UserAccount{ID: 42, Email: "user@example.com"}
	enriched, err := c.EnrichCreditCards(me)
	if err != nil {
		t.Fatal(err)
	}
	if enriched.DefaultCardExternalID != "00000000-0000-4000-8000-000000000001" {
		t.Fatalf("external=%q", enriched.DefaultCardExternalID)
	}
	if enriched.DefaultCardID != 10000001 {
		t.Fatalf("card_id=%d", enriched.DefaultCardID)
	}
}

func TestResolveCardExternalIDFromNumericID(t *testing.T) {
	cards, err := parseCreditCards(usersCreditCardsFixtureAccount)
	if err != nil {
		t.Fatal(err)
	}
	acct := UserAccount{CreditCards: cards}
	ext, err := resolveCardExternalID(acct, "", 10000001)
	if err != nil || ext != "00000000-0000-4000-8000-000000000001" {
		t.Fatalf("ext=%q err=%v", ext, err)
	}
}

func TestParseVehiclesDefault(t *testing.T) {
	vehicles, err := parseVehicles(usersVehiclesFixtureAccount)
	if err != nil {
		t.Fatal(err)
	}
	v, ok := pickDefaultVehicle(vehicles)
	if !ok || v.ID != 20000001 || v.PlateLast2 != "23" {
		t.Fatalf("%+v ok=%v", v, ok)
	}
}

func TestParseUserAccountPaymentMethodsFallback(t *testing.T) {
	raw := json.RawMessage(`{"id":1,"email":"a@b.com","payment_methods":[{"id":99,"card_external_id":"uuid-99","is_default":true}]}`)
	acct, err := parseUserAccount(raw)
	if err != nil {
		t.Fatal(err)
	}
	if acct.DefaultCardExternalID != "uuid-99" {
		t.Fatalf("external=%q", acct.DefaultCardExternalID)
	}
}

func TestPickDefaultCardPrefersIsDefault(t *testing.T) {
	cards := []CreditCard{
		{CardID: 1, CardExternalID: "ext-1", IsDefault: false},
		{CardID: 2, CardExternalID: "ext-2", IsDefault: true},
	}
	ext, id, ok := pickDefaultCardExternal(cards, 0)
	if !ok || ext != "ext-2" || id != 2 {
		t.Fatalf("ext=%q id=%d ok=%v", ext, id, ok)
	}
}
