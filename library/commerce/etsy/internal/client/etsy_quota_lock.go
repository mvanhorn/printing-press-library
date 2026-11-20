package client

import (
	"context"
	"math/rand"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/cliutil"
)

// withEtsyQuotaLock bounds advisory-state contention, including same-process peers.
func withEtsyQuotaLock(ctx context.Context, path string, fn func() error) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
	defer cancel()
	for {
		if err := ctx.Err(); err != nil {
			return false, err
		}
		acquired, err := cliutil.TryWithFileLock(path, fn)
		if acquired || err != nil {
			return acquired, err
		}
		timer := time.NewTimer(time.Duration(2+rand.Intn(9)) * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return false, ctx.Err()
		case <-timer.C:
		}
	}
}
