package gplay

import (
	"context"
	"fmt"
)

// Review sort modes (wire integers).
const (
	SortHelpfulness = 1
	SortNewest      = 2
	SortRating      = 3
)

// NormalizeSort maps a user-facing sort name to its wire integer.
func NormalizeSort(s string) (int, bool) {
	switch s {
	case "HELPFULNESS", "helpfulness", "RELEVANCE", "relevance":
		return SortHelpfulness, true
	case "NEWEST", "newest", "":
		return SortNewest, true
	case "RATING", "rating":
		return SortRating, true
	}
	return 0, false
}

// Reviews fetches up to num reviews for an app via the oCPfdb RPC, following
// continuation tokens until num is reached or the store runs out. score (1-5)
// and device (2 mobile,3 tablet,5 chromebook,6 tv) are 0 when unfiltered.
func (c *Client) Reviews(ctx context.Context, appID string, sort, num, score, device int) ([]Review, error) {
	if appID == "" {
		return nil, fmt.Errorf("appId is required")
	}
	if num <= 0 {
		num = 100
	}
	const pageSize = 100
	var out []Review
	token := ""
	for len(out) < num {
		want := pageSize
		if remaining := num - len(out); remaining < want {
			want = remaining
		}
		page, next, err := c.reviewsPage(ctx, appID, sort, want, score, device, token)
		if err != nil {
			return out, err
		}
		out = append(out, page...)
		if next == "" || len(page) == 0 {
			break
		}
		token = next
	}
	if len(out) > num {
		out = out[:num]
	}
	return out, nil
}

func (c *Client) reviewsPage(ctx context.Context, appID string, sort, count, score, device int, token string) ([]Review, string, error) {
	// Inner payload shape (oCPfdb), per live capture + JoMingyu request.py:
	//   [null,[2,<sort>,[<count>(,null,"<token>")],null,[null,<score|null>,...,<device|null>]],["<appId>",7]]
	scoreField := "null"
	if score >= 1 && score <= 5 {
		scoreField = fmt.Sprintf("%d", score)
	}
	deviceField := "null"
	if device > 0 {
		deviceField = fmt.Sprintf("%d", device)
	}
	countTriple := fmt.Sprintf("[%d]", count)
	if token != "" {
		countTriple = fmt.Sprintf("[%d,null,%q]", count, token)
	}
	inner := fmt.Sprintf(
		`[null,[2,%d,%s,null,[null,%s,null,null,null,null,null,null,%s]],[%q,7]]`,
		sort, countTriple, scoreField, deviceField, appID,
	)
	payload, err := c.batchExecute(ctx, "oCPfdb", inner, "generic", "")
	if err != nil {
		return nil, "", err
	}
	if payload == nil {
		return nil, "", nil
	}
	root := decode(payload)
	reviewsArr := root.path(0)
	var out []Review
	for _, r := range reviewsArr.arr() {
		rv := Review{
			ID:        r.path(0).str(),
			UserName:  r.path(1, 0).cleanStr(),
			Score:     r.path(2).int(),
			Text:      r.path(4).cleanStr(),
			At:        r.path(5, 0).int64(),
			ThumbsUp:  r.path(6).int(),
			Version:   r.path(10).str(),
			ReplyText: r.path(7, 1).cleanStr(),
			RepliedAt: r.path(7, 2, 0).int64(),
		}
		if rv.ID != "" || rv.Text != "" {
			out = append(out, rv)
		}
	}
	next := root.path(1, 1).str()
	return out, next, nil
}
