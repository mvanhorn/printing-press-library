package traveloka

import (
	"github.com/mvanhorn/printing-press-library/library/travel/traveloka/internal/cliutil"
	"math"
)

// SetRateLimit applies the CLI's explicit pacing setting without losing typed throttling.
// -1 selects adaptive source pacing, zero disables pacing, and positive values set a ceiling.
func (c *Client) SetRateLimit(rate float64) error {
	if math.IsNaN(rate) || math.IsInf(rate, 0) || (rate < 0 && rate != -1) {
		return apiError("INVALID_INPUT", "--rate-limit must be auto, zero or a finite positive request rate", 0, false)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if rate == -1 {
		c.limiter = cliutil.NewAdaptiveLimiterAuto(2)
	} else {
		c.limiter = cliutil.NewAdaptiveLimiter(rate)
	}
	return nil
}
