package agent

import (
	"fmt"
	"time"
)

// DecayInput is the data needed to score freshness for one account.
type DecayInput struct {
	Account            Account
	LastActivityDate   *time.Time // most recent Task or Event
	LastOppStageChange *time.Time // most recent Opportunity.LastModifiedDate
	LastCaseDate       *time.Time // most recent Case.LastModifiedDate
	LastChatterDate    *time.Time // most recent FeedItem.CreatedDate
	OpenOppCount       int
	OpenCaseCount      int
}

// DecayResult is the structured output of `agent decay --account`.
type DecayResult struct {
	AccountID   string        `json:"account_id"`
	AccountName string        `json:"account_name"`
	Score       int           `json:"score"` // 0-100, higher = fresher
	Signals     []DecaySignal `json:"signals"`
	GeneratedAt time.Time     `json:"generated_at"`
}

// DecaySignal names one thing that pulled the score up or down.
type DecaySignal struct {
	Name      string `json:"name"`
	Severity  string `json:"severity"` // ok | warning | critical
	DaysAgo   *int   `json:"days_ago,omitempty"`
	Threshold string `json:"threshold"`
	Detail    string `json:"detail,omitempty"`
}

// ScoreDecay applies the weighted rubric. The rubric is intentionally simple
// in v1 so it ships deterministic and explainable. Future versions can
// factor in opportunity stage drift rates, rep activity ratios, etc.
func ScoreDecay(in DecayInput) DecayResult {
	now := time.Now().UTC()
	result := DecayResult{
		AccountID:   in.Account.ID,
		AccountName: in.Account.Name,
		GeneratedAt: now,
	}
	score := 100

	score, result.Signals = scoreSignal(
		score, result.Signals,
		"activity_recency",
		in.LastActivityDate,
		now,
		[3]int{14, 30, 60}, // warn > 14d, critical > 30d, severe > 60d
		[3]int{5, 15, 30},
	)
	score, result.Signals = scoreSignal(
		score, result.Signals,
		"opportunity_stage_change",
		in.LastOppStageChange,
		now,
		[3]int{30, 60, 90},
		[3]int{5, 15, 25},
	)
	score, result.Signals = scoreSignal(
		score, result.Signals,
		"case_activity",
		in.LastCaseDate,
		now,
		[3]int{30, 90, 180},
		[3]int{3, 10, 20},
	)
	score, result.Signals = scoreSignal(
		score, result.Signals,
		"chatter_activity",
		in.LastChatterDate,
		now,
		[3]int{14, 45, 90},
		[3]int{3, 8, 15},
	)

	if in.OpenCaseCount > 3 && (in.LastCaseDate == nil || now.Sub(*in.LastCaseDate) > 14*24*time.Hour) {
		score -= 10
		result.Signals = append(result.Signals, DecaySignal{
			Name:      "open_cases_without_recent_touch",
			Severity:  "critical",
			Threshold: ">3 open cases and no case touched in 14d",
			Detail:    fmt.Sprintf("%d open cases", in.OpenCaseCount),
		})
	}

	if score < 0 {
		score = 0
	}
	result.Score = score
	return result
}

func scoreSignal(
	score int,
	existing []DecaySignal,
	name string,
	last *time.Time,
	now time.Time,
	thresholds [3]int,
	penalties [3]int,
) (int, []DecaySignal) {
	if last == nil {
		existing = append(existing, DecaySignal{
			Name:      name,
			Severity:  "critical",
			Threshold: "no data synced",
			Detail:    "no recorded " + name,
		})
		return score - penalties[2], existing
	}
	days := int(now.Sub(*last).Hours() / 24)
	sig := DecaySignal{Name: name, DaysAgo: &days}
	switch {
	case days > thresholds[2]:
		sig.Severity = "critical"
		sig.Threshold = fmt.Sprintf(">%dd", thresholds[2])
		score -= penalties[2]
	case days > thresholds[1]:
		sig.Severity = "warning"
		sig.Threshold = fmt.Sprintf(">%dd", thresholds[1])
		score -= penalties[1]
	case days > thresholds[0]:
		sig.Severity = "warning"
		sig.Threshold = fmt.Sprintf(">%dd", thresholds[0])
		score -= penalties[0]
	default:
		sig.Severity = "ok"
		sig.Threshold = fmt.Sprintf("<=%dd", thresholds[0])
	}
	existing = append(existing, sig)
	return score, existing
}
