package normalize

import (
	"math"

	"github.com/mvanhorn/printing-press-library/library/commerce/continente/internal/domain"
)

func PriceSummary(product domain.Product) domain.Price {
	price := product.Price
	price.HasPromotion = len(price.PromotionText) > 0
	price.HasDiscount = false
	if price.DiscountAmount != nil && *price.DiscountAmount > 0 {
		price.HasDiscount = true
	}
	if price.SavingsPercent == nil && price.OriginalAmount != nil && price.Effective != nil && *price.OriginalAmount > 0 && *price.OriginalAmount > *price.Effective {
		savingsPercent := roundPercent(((*price.OriginalAmount - *price.Effective) / *price.OriginalAmount) * 100)
		price.SavingsPercent = &savingsPercent
		price.HasDiscount = true
	}
	return price
}

func roundPercent(v float64) float64 {
	return math.Round(v*10) / 10
}
