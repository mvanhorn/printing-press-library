package client

const etsyDefaultRateLimit = 4.5

// etsyEffectiveRateLimit caps auto mode below Etsy's advertised ceiling.
// Explicit positive ceilings are preserved; --rate-limit 0 still disables limiting.
func etsyEffectiveRateLimit(requested float64, key string) float64 {
	if requested < 0 {
		if q, known := LoadEtsyQuota(key); known && q.LimitPerSecond > 0 {
			return 0.9 * float64(q.LimitPerSecond)
		}
		return etsyDefaultRateLimit
	}
	return requested
}
