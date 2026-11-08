package consumer

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// GetMe returns the authenticated consumer account (preferred over GET /user/).
func (c *Client) GetMe() (UserAccount, error) {
	raw, err := c.doJSONData(http.MethodGet, PathUsersMe, nil)
	if err != nil {
		return UserAccount{}, err
	}
	return parseUserAccount(raw)
}

// ResolveEmail prefers /users/me/, falling back to /user/.
func (c *Client) ResolveEmail(preferred string) (string, error) {
	if preferred != "" {
		return preferred, nil
	}
	if me, err := c.GetMe(); err == nil && me.Email != "" {
		return me.Email, nil
	}
	user, err := c.GetUser()
	if err != nil {
		return "", err
	}
	if user.Email == "" {
		return "", fmt.Errorf("account email unavailable")
	}
	return user.Email, nil
}

func parseUserAccount(raw json.RawMessage) (UserAccount, error) {
	var wire struct {
		ID            FlexInt `json:"id"`
		Email         string  `json:"email"`
		PhoneNumber   string  `json:"phone_number"`
		Phone         string  `json:"phone"`
		MobilePhone   string  `json:"mobile_phone"`
		FirstName     string  `json:"first_name"`
		LastName      string  `json:"last_name"`
		DefaultCardID FlexInt `json:"default_card_id"`
		ContactInfo   struct {
			PhoneNumber string `json:"phone_number"`
		} `json:"contact_info"`
		PaymentMethods []struct {
			ID             FlexInt `json:"id"`
			CardID         FlexInt `json:"card_id"`
			CardExternalID string  `json:"card_external_id"`
			IsDefault      bool    `json:"is_default"`
		} `json:"payment_methods"`
		Cards       []creditCardWire `json:"cards"`
		CreditCards []creditCardWire `json:"credit_cards"`
	}
	if err := json.Unmarshal(raw, &wire); err != nil {
		return UserAccount{}, err
	}
	out := UserAccount{
		ID:        int(wire.ID),
		Email:     wire.Email,
		FirstName: wire.FirstName,
		LastName:  wire.LastName,
	}
	out.PhoneNumber = firstNonEmpty(wire.PhoneNumber, wire.Phone, wire.MobilePhone, wire.ContactInfo.PhoneNumber)
	if wire.DefaultCardID > 0 {
		out.DefaultCardID = int(wire.DefaultCardID)
	}
	cards := append(append([]creditCardWire{}, wire.CreditCards...), wire.Cards...)
	for _, pm := range wire.PaymentMethods {
		cards = append(cards, creditCardWire{
			ID:             pm.ID,
			CardID:         pm.CardID,
			CardExternalID: pm.CardExternalID,
			IsDefault:      pm.IsDefault,
		})
	}
	out.CreditCards = normalizeCreditCards(cards)
	if ext, id, ok := pickDefaultCardExternal(out.CreditCards, out.DefaultCardID); ok {
		out.DefaultCardExternalID = ext
		if out.DefaultCardID == 0 {
			out.DefaultCardID = id
		}
	}
	return out, nil
}

type creditCardWire struct {
	ID             FlexInt `json:"id"`
	CardID         FlexInt `json:"card_id"`
	CardExternalID string  `json:"card_external_id"`
	CardLast4      string  `json:"card_last4"`
	IsDefault      bool    `json:"is_default"`
	IsDeleted      bool    `json:"is_deleted"`
	Deleted        bool    `json:"deleted"`
}

func normalizeCreditCards(wires []creditCardWire) []CreditCard {
	out := make([]CreditCard, 0, len(wires))
	for _, w := range wires {
		if w.IsDeleted || w.Deleted {
			continue
		}
		cardID := int(w.CardID)
		if cardID == 0 {
			cardID = int(w.ID)
		}
		ext := w.CardExternalID
		if cardID == 0 && ext == "" {
			continue
		}
		out = append(out, CreditCard{
			CardID:         cardID,
			CardExternalID: ext,
			CardLast4:      w.CardLast4,
			IsDefault:      w.IsDefault,
		})
	}
	return out
}

func pickDefaultCardExternal(cards []CreditCard, preferredID int) (externalID string, cardID int, ok bool) {
	if preferredID > 0 {
		for _, c := range cards {
			if c.CardID == preferredID && c.CardExternalID != "" {
				return c.CardExternalID, c.CardID, true
			}
		}
	}
	for _, c := range cards {
		if c.IsDefault && c.CardExternalID != "" {
			return c.CardExternalID, c.CardID, true
		}
	}
	for _, c := range cards {
		if c.CardExternalID != "" {
			return c.CardExternalID, c.CardID, true
		}
	}
	return "", 0, false
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

func resolvePhoneNumber(me UserAccount, override string) (string, error) {
	if override != "" {
		return override, nil
	}
	if me.PhoneNumber != "" {
		return me.PhoneNumber, nil
	}
	return "", fmt.Errorf("phone_number required; pass --phone or add contact info on account")
}

func resolveCardExternalID(me UserAccount, cardExternalID string, cardID int) (string, error) {
	if cardExternalID != "" {
		return cardExternalID, nil
	}
	lookupID := cardID
	if lookupID == 0 {
		lookupID = me.DefaultCardID
	}
	if cardID > 0 {
		// An explicit id that does not match must fail: quietly charging the default
		// card instead would spend from a card the owner did not choose.
		for _, card := range me.CreditCards {
			if card.CardID == cardID && card.CardExternalID != "" {
				return card.CardExternalID, nil
			}
		}
		return "", fmt.Errorf("no saved card with card_id %d; see account cards", cardID)
	}
	if ext, _, ok := pickDefaultCardExternal(me.CreditCards, lookupID); ok {
		return ext, nil
	}
	if lookupID > 0 {
		return "", fmt.Errorf("no card_external_id for card_id %d; pass --card-external-id", lookupID)
	}
	return "", fmt.Errorf("no default payment card; pass --card-external-id")
}

// ListCreditCards returns saved cards for a user id (GET /users/{id}/credit-cards/).
func (c *Client) ListCreditCards(userID int) ([]CreditCard, error) {
	path := fmt.Sprintf(PathUserCreditCards, userID)
	raw, err := c.doJSONData(http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	return parseCreditCards(raw)
}

func parseCreditCards(raw json.RawMessage) ([]CreditCard, error) {
	var list []creditCardWire
	if err := json.Unmarshal(raw, &list); err == nil && len(list) > 0 {
		return normalizeCreditCards(list), nil
	}
	var wrapped struct {
		Results []creditCardWire `json:"results"`
	}
	if err := json.Unmarshal(raw, &wrapped); err != nil {
		return nil, err
	}
	return normalizeCreditCards(wrapped.Results), nil
}

// EnrichCreditCards loads cards from GET /users/{id}/credit-cards/ when /users/me/ omits them.
func (c *Client) EnrichCreditCards(me UserAccount) (UserAccount, error) {
	if len(me.CreditCards) > 0 {
		return me, nil
	}
	if me.ID == 0 {
		return me, fmt.Errorf("account id unavailable for credit card lookup")
	}
	cards, err := c.ListCreditCards(me.ID)
	if err != nil {
		return me, err
	}
	me.CreditCards = cards
	if ext, id, ok := pickDefaultCardExternal(me.CreditCards, me.DefaultCardID); ok {
		me.DefaultCardExternalID = ext
		if me.DefaultCardID == 0 {
			me.DefaultCardID = id
		}
	}
	return me, nil
}

// ListVehicles returns saved vehicles for a user id.
func (c *Client) ListVehicles(userID int) ([]VehicleProfile, error) {
	path := fmt.Sprintf(PathUserVehicles, userID)
	raw, err := c.doJSONData(http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	return parseVehicles(raw)
}

func parseVehicles(raw json.RawMessage) ([]VehicleProfile, error) {
	var bare []vehicleWire
	if err := json.Unmarshal(raw, &bare); err == nil && len(bare) > 0 {
		out := make([]VehicleProfile, 0, len(bare))
		for _, v := range bare {
			out = append(out, v.normalize())
		}
		return out, nil
	}
	var wrapped struct {
		Results []vehicleWire `json:"results"`
	}
	if err := json.Unmarshal(raw, &wrapped); err != nil {
		return nil, err
	}
	out := make([]VehicleProfile, 0, len(wrapped.Results))
	for _, v := range wrapped.Results {
		out = append(out, v.normalize())
	}
	return out, nil
}

type vehicleWire struct {
	ID FlexInt `json:"id"`
	// LicensePlate is an object on the live API; older captures had a bare string. A
	// wrong type here failed the whole vehicle list, which made every plate-required
	// booking look like the account had no vehicle.
	LicensePlate      plateWire `json:"license_plate"`
	LicensePlateState string    `json:"license_plate_state"`
	IsDefault         bool      `json:"is_default"`
	Info              struct {
		Description string `json:"display_description"`
	} `json:"info"`
}

type plateWire struct {
	Value string
	State string
}

func (p *plateWire) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err == nil {
		p.Value = s
		return nil
	}
	var o struct {
		Value string `json:"value"`
		State string `json:"state"`
	}
	if err := json.Unmarshal(b, &o); err != nil {
		return err
	}
	p.Value, p.State = o.Value, o.State
	return nil
}

func (v vehicleWire) normalize() VehicleProfile {
	state := v.LicensePlateState
	if state == "" {
		state = v.LicensePlate.State
	}
	last2 := v.LicensePlate.Value
	if len(last2) > 2 {
		last2 = last2[len(last2)-2:]
	}
	return VehicleProfile{
		ID:                int(v.ID),
		PlateLast2:        last2,
		LicensePlateState: state,
		Description:       v.Info.Description,
		IsDefault:         v.IsDefault,
	}
}

func pickDefaultVehicle(vehicles []VehicleProfile) (VehicleProfile, bool) {
	for _, v := range vehicles {
		if v.IsDefault {
			return v, true
		}
	}
	if len(vehicles) == 1 {
		return vehicles[0], true
	}
	if len(vehicles) > 0 {
		return vehicles[0], true
	}
	return VehicleProfile{}, false
}
