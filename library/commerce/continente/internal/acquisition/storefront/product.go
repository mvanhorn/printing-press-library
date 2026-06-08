package storefront

import (
	"context"
	"encoding/json"
	"fmt"
)

func FetchProduct(ctx context.Context, c Getter, slugAndPID string) (json.RawMessage, error) {
	if c == nil {
		return nil, fmt.Errorf("storefront product fetch: nil client")
	}
	return c.Get(ctx, "/produto/"+slugAndPID+".html", map[string]string{})
}
