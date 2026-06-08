package domain

type Price struct {
	DisplayAmount  float64  `json:"display_amount,omitempty"`
	Effective      *float64 `json:"effective_amount,omitempty"`
	OriginalAmount *float64 `json:"original_amount,omitempty"`
	DiscountAmount *float64 `json:"discount_amount,omitempty"`
	UnitAmount     *float64 `json:"unit_amount,omitempty"`
	SavingsPercent *float64 `json:"savings_percent,omitempty"`
	UnitLabel      string   `json:"unit_label,omitempty"`
	PackLabel      string   `json:"pack_label,omitempty"`
	Currency       string   `json:"currency,omitempty"`
	Confidence     string   `json:"confidence,omitempty"`
	HasPromotion   bool     `json:"has_promotion,omitempty"`
	HasDiscount    bool     `json:"has_discount,omitempty"`
	Ambiguities    []string `json:"ambiguities,omitempty"`
	PromotionText  []string `json:"promotion_text,omitempty"`
}
