// Hand-authored StackAdapt reporting layer: queries the campaignDelivery union
// (CampaignDeliveryOutcome | Progress) and returns per-campaign delivery
// metrics. Shared by bottleneck, stale-campaigns, and delivery-drift. No
// generated header: preserved across regen.
package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"
)

// deliveryRecord is one campaign's delivery metrics over a window.
type deliveryRecord struct {
	CampaignID   string             `json:"campaign_id"`
	CampaignName string             `json:"campaign_name"`
	Metrics      map[string]float64 `json:"metrics"`
}

// asFloat parses a JSON value that may be a number, a quoted number
// (MoneyValue/BigInt scalars often serialize as strings), or null.
func asFloat(r json.RawMessage) (float64, bool) {
	if len(r) == 0 || string(r) == "null" {
		return 0, false
	}
	s := string(r)
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		s = s[1 : len(s)-1]
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, false
	}
	return f, true
}

// dateRange returns ISO8601 from/to strings for the last `days` days.
func dateRange(days int) (string, string) {
	to := time.Now().UTC()
	from := to.AddDate(0, 0, -days)
	const iso = "2006-01-02"
	return from.Format(iso), to.Format(iso)
}

// priorWindow returns the current window (last `days`) and the immediately
// preceding window of the same length, as ISO8601 from/to strings.
func priorWindow(days int) (curFrom, curTo, priorFrom, priorTo string) {
	const iso = "2006-01-02"
	now := time.Now().UTC()
	curTo = now.Format(iso)
	curFrom = now.AddDate(0, 0, -days).Format(iso)
	priorTo = curFrom
	priorFrom = now.AddDate(0, 0, -2*days).Format(iso)
	return
}

// the delivery metric fields we select; all live on DeliveryStatsRecord.
const deliveryMetricFields = `cost ctr cvr conversions conversionRevenue ecpa ecpc clicksBigint impressionsBigint`

// fetchCampaignDelivery queries campaignDelivery for a window/granularity and
// returns per-campaign records. Surfaces the async Progress path as a clear
// error rather than empty data.
func fetchCampaignDelivery(ctx context.Context, flags *rootFlags, filter map[string]any, granularity, from, to string) ([]deliveryRecord, error) {
	q := fmt.Sprintf(`query($date:DateRangeInput,$gran:DeliveryStatsGranularity!,$f:CampaignFilters,$dt:DeliveryStatsDataType!){
		campaignDelivery(date:$date, granularity:$gran, filterBy:$f, dataType:$dt){
			__typename
			... on CampaignDeliveryOutcome {
				records { nodes { campaign { id name } metrics { %s } } }
			}
			... on Progress { __typename }
		}
	}`, deliveryMetricFields)
	vars := map[string]any{
		"date": map[string]any{"from": from, "to": to},
		"gran": granularity,
		"f":    filter,
		"dt":   "TABLE",
	}
	data, err := runQuery(ctx, flags, q, vars)
	if err != nil {
		return nil, err
	}
	var top struct {
		CampaignDelivery struct {
			TypeName string `json:"__typename"`
			Records  struct {
				Nodes []struct {
					Campaign struct {
						ID   json.RawMessage `json:"id"`
						Name string          `json:"name"`
					} `json:"campaign"`
					Metrics map[string]json.RawMessage `json:"metrics"`
				} `json:"nodes"`
			} `json:"records"`
		} `json:"campaignDelivery"`
	}
	if err := json.Unmarshal(data, &top); err != nil {
		return nil, fmt.Errorf("parsing delivery response: %w", err)
	}
	if top.CampaignDelivery.TypeName == "Progress" {
		return nil, fmt.Errorf("StackAdapt is still computing this report (async); re-run in a moment")
	}
	out := make([]deliveryRecord, 0, len(top.CampaignDelivery.Records.Nodes))
	for _, n := range top.CampaignDelivery.Records.Nodes {
		m := make(map[string]float64, len(n.Metrics))
		for k, v := range n.Metrics {
			if f, ok := asFloat(v); ok {
				m[k] = f
			}
		}
		out = append(out, deliveryRecord{
			CampaignID:   trimQuotes(n.Campaign.ID),
			CampaignName: n.Campaign.Name,
			Metrics:      m,
		})
	}
	return out, nil
}

// roas returns return-on-ad-spend (conversionRevenue / cost), or -1 when cost
// is zero (undefined).
func roas(m map[string]float64) float64 {
	cost := m["cost"]
	if cost <= 0 {
		return -1
	}
	return m["conversionRevenue"] / cost
}
