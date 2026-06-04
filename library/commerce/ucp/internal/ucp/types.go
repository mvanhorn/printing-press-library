// Package ucp implements client-side primitives for Google's Universal Commerce Protocol.
// See https://ucp.dev/2026-04-08/specification/overview for the canonical spec.
package ucp

import "encoding/json"

// Manifest is the JSON document a UCP-compliant merchant publishes at /.well-known/ucp.
type Manifest struct {
	UCP UCPBlock `json:"ucp"`
}

// UCPBlock is the protocol-version-scoped portion of the manifest. Per the
// UCP 2026-04-08 spec (and verified against checkout.coffeecircle.com),
// payment_handlers lives INSIDE the ucp block.
type UCPBlock struct {
	Version           string                     `json:"version"`
	SupportedVersions map[string]string          `json:"supported_versions,omitempty"`
	Services          map[string][]Service       `json:"services,omitempty"`
	Capabilities      map[string][]Capability    `json:"capabilities,omitempty"`
	PaymentHandlers   map[string][]PaymentConfig `json:"payment_handlers,omitempty"`
}

// Service describes a transport binding for a UCP service (e.g. dev.ucp.shopping).
type Service struct {
	Version   string `json:"version"`
	Spec      string `json:"spec,omitempty"`
	Transport string `json:"transport"`
	Endpoint  string `json:"endpoint,omitempty"`
	Schema    string `json:"schema,omitempty"`
}

// Capability describes one UCP capability and its extension chain.
type Capability struct {
	Version  string          `json:"version"`
	Spec     string          `json:"spec,omitempty"`
	Schema   string          `json:"schema,omitempty"`
	Extends  json.RawMessage `json:"extends,omitempty"` // string or []string in the wild
	Requires json.RawMessage `json:"requires,omitempty"`
	Config   json.RawMessage `json:"config,omitempty"`
}

// PaymentConfig is one entry under payment_handlers[<reverse-domain>].
type PaymentConfig struct {
	ID      string          `json:"id"`
	Version string          `json:"version"`
	Spec    string          `json:"spec,omitempty"`
	Schema  string          `json:"schema,omitempty"`
	Config  json.RawMessage `json:"config,omitempty"`
}

// Cart is the local representation of a UCP cart. It mirrors the
// dev.ucp.shopping.cart schema shape but is intentionally a subset
// sufficient for the search-to-checkout-prep flow.
type Cart struct {
	ID        string     `json:"id"`
	Merchant  string     `json:"merchant"`
	Currency  string     `json:"currency,omitempty"`
	LineItems []LineItem `json:"line_items"`
	Buyer     *Buyer     `json:"buyer,omitempty"`
	Totals    []Total    `json:"totals,omitempty"`
	Status    string     `json:"status,omitempty"` // mirrors UCP status state machine
	CreatedAt string     `json:"created_at"`
	UpdatedAt string     `json:"updated_at"`
}

// LineItem is one item in a cart or checkout session.
type LineItem struct {
	ID       string  `json:"id"`
	Item     Item    `json:"item"`
	Quantity int     `json:"quantity"`
	Totals   []Total `json:"totals,omitempty"`
}

// Item describes a product. SKU and GTIN are optional; Title and Price are required.
type Item struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Price int    `json:"price"` // minor units (cents)
	SKU   string `json:"sku,omitempty"`
	GTIN  string `json:"gtin,omitempty"`
	URL   string `json:"url,omitempty"`
	// VariantID is the Shopify numeric variant ID. Required for ShopifyCartAdd;
	// zero for non-Shopify items.
	VariantID int64 `json:"variant_id,omitempty"`
}

// Buyer is the shopper identity attached to a cart/checkout.
type Buyer struct {
	FullName string `json:"full_name,omitempty"`
	Email    string `json:"email,omitempty"`
}

// Total is one entry in a UCP totals array (subtotal, discount, tax, shipping, total).
type Total struct {
	Type   string `json:"type"`
	Amount int    `json:"amount"` // minor units
}

// SearchHit is one normalized product result from a catalog search.
type SearchHit struct {
	Merchant string `json:"merchant"`
	Title    string `json:"title"`
	Price    int    `json:"price,omitempty"`
	Currency string `json:"currency,omitempty"`
	SKU      string `json:"sku,omitempty"`
	GTIN     string `json:"gtin,omitempty"`
	URL      string `json:"url,omitempty"`
	// VariantID is the Shopify numeric variant ID (e.g. 42563115909207).
	// Required by ShopifyCartAdd; zero for non-Shopify adapters.
	VariantID int64 `json:"variant_id,omitempty"`
}

// CheckoutDraft is the JSON envelope an AP2 CLI consumes to authorize
// payment for a cart-to-checkout transition. It captures the cart,
// the merchant identity, the negotiated payment handler, and the
// would-be checkout-session payload — without actually issuing
// POST /checkout-sessions/{id}/complete (that is AP2's role).
type CheckoutDraft struct {
	CartID            string          `json:"cart_id"`
	Merchant          string          `json:"merchant"`
	MerchantDomain    string          `json:"merchant_domain"`
	NegotiatedPayment string          `json:"negotiated_payment_handler,omitempty"`
	CheckoutSession   json.RawMessage `json:"checkout_session"`
	MissingFields     []string        `json:"missing_fields,omitempty"`
	AP2Ready          bool            `json:"ap2_ready"`
}
