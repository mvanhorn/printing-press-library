package ucp

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// AP2Mandate is the common shape for IntentMandate, CartMandate, PaymentMandate.
// Each has metadata + a body that an external AP2 CLI/agent signs.
type AP2Mandate struct {
	Type      string          `json:"type"` // "intent" | "cart" | "payment"
	MandateID string          `json:"mandate_id"`
	IssuedAt  string          `json:"issued_at"`
	ExpiresAt string          `json:"expires_at,omitempty"`
	Subject   string          `json:"subject"` // user agent identifier or pseudonym
	Body      json.RawMessage `json:"body"`
	BodyHash  string          `json:"body_hash"`           // SHA-256 of canonicalized body
	Signature string          `json:"signature,omitempty"` // filled in by AP2 CLI
}

// IntentMandateBody describes what the user authorized in plain language.
type IntentMandateBody struct {
	Description      string   `json:"description"` // e.g. "Buy a durable dog rope toy under $30"
	MaxAmountCents   int      `json:"max_amount_cents"`
	Currency         string   `json:"currency"`
	AllowedMerchants []string `json:"allowed_merchants,omitempty"`
	ExpiresInHours   int      `json:"expires_in_hours"`
}

// CartMandateBody is the concrete cart the agent built that the user is approving.
type CartMandateBody struct {
	Merchant     string     `json:"merchant"`
	MerchantCart string     `json:"merchant_cart_token,omitempty"`
	CheckoutURL  string     `json:"checkout_url,omitempty"`
	LineItems    []LineItem `json:"line_items"`
	Subtotal     int        `json:"subtotal_cents"`
	Currency     string     `json:"currency"`
	IntentRef    string     `json:"intent_mandate_id"`
}

// PaymentMandateBody authorizes the payment-handler call for the cart.
type PaymentMandateBody struct {
	PaymentHandler string `json:"payment_handler"` // e.g. "com.google.pay"
	AmountCents    int    `json:"amount_cents"`
	Currency       string `json:"currency"`
	CartRef        string `json:"cart_mandate_id"`
	MerchantCart   string `json:"merchant_cart_token,omitempty"`
}

// FinalizationEnvelope is the JSON envelope `ucp-pp-cli checkout finalize` emits
// for an external AP2 CLI to sign and complete. It bundles the three mandates +
// merchant-cart context the AP2 CLI needs.
type FinalizationEnvelope struct {
	Version           string     `json:"envelope_version"` // "1.0"
	Subject           string     `json:"subject"`
	IntentMandate     AP2Mandate `json:"intent_mandate"`
	CartMandate       AP2Mandate `json:"cart_mandate"`
	PaymentMandate    AP2Mandate `json:"payment_mandate"`
	Merchant          string     `json:"merchant"`
	MerchantCartToken string     `json:"merchant_cart_token,omitempty"`
	CheckoutURL       string     `json:"checkout_url"`
	Instructions      string     `json:"instructions"`
}

func nowRFC3339() string { return time.Now().UTC().Format(time.RFC3339) }

func canonicalHash(v any) string {
	b, _ := json.Marshal(v)
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// BuildIntentMandate creates an unsigned AP2 IntentMandate for the given cart.
// The agent (or user) declares purchase intent; an external AP2 CLI signs it later.
func BuildIntentMandate(subject string, body IntentMandateBody) AP2Mandate {
	if body.Currency == "" {
		body.Currency = "USD"
	}
	if body.ExpiresInHours == 0 {
		body.ExpiresInHours = 24
	}
	bodyJSON, _ := json.Marshal(body)
	return AP2Mandate{
		Type:      "intent",
		MandateID: "intent_" + uuid.NewString(),
		IssuedAt:  nowRFC3339(),
		ExpiresAt: time.Now().UTC().Add(time.Duration(body.ExpiresInHours) * time.Hour).Format(time.RFC3339),
		Subject:   subject,
		Body:      bodyJSON,
		BodyHash:  canonicalHash(body),
	}
}

// BuildCartMandate creates an unsigned CartMandate from a local cart + merchant cart info.
func BuildCartMandate(subject, intentMandateID string, cart *Cart, merchantCartToken, checkoutURL string) AP2Mandate {
	body := CartMandateBody{
		Merchant:     cart.Merchant,
		MerchantCart: merchantCartToken,
		CheckoutURL:  checkoutURL,
		LineItems:    cart.LineItems,
		Currency:     cart.Currency,
		IntentRef:    intentMandateID,
	}
	if body.Currency == "" {
		body.Currency = "USD"
	}
	for _, li := range cart.LineItems {
		body.Subtotal += li.Item.Price * li.Quantity
	}
	bodyJSON, _ := json.Marshal(body)
	return AP2Mandate{
		Type:      "cart",
		MandateID: "cart_" + uuid.NewString(),
		IssuedAt:  nowRFC3339(),
		Subject:   subject,
		Body:      bodyJSON,
		BodyHash:  canonicalHash(body),
	}
}

// BuildPaymentMandate creates an unsigned PaymentMandate referencing the cart.
// paymentHandler is the negotiated handler (e.g. "com.google.pay" from the merchant manifest).
func BuildPaymentMandate(subject, cartMandateID, paymentHandler, merchantCartToken string, amountCents int, currency string) AP2Mandate {
	if currency == "" {
		currency = "USD"
	}
	body := PaymentMandateBody{
		PaymentHandler: paymentHandler,
		AmountCents:    amountCents,
		Currency:       currency,
		CartRef:        cartMandateID,
		MerchantCart:   merchantCartToken,
	}
	bodyJSON, _ := json.Marshal(body)
	return AP2Mandate{
		Type:      "payment",
		MandateID: "payment_" + uuid.NewString(),
		IssuedAt:  nowRFC3339(),
		Subject:   subject,
		Body:      bodyJSON,
		BodyHash:  canonicalHash(body),
	}
}
