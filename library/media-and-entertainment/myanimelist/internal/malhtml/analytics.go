package malhtml

import (
	"fmt"
	"math"
	"sort"
)

// Divisiveness describes the *shape* of a score distribution: two titles can
// share an 8.5 average while one is beloved by everyone and the other is a
// love-it-or-hate-it split. MyAnimeList exposes the underlying 1-10 vote
// distribution only as a rendered bar chart, so this is the computation that
// makes it queryable.
type Divisiveness struct {
	Kind           string  `json:"kind"`
	ID             int     `json:"id"`
	Title          string  `json:"title,omitempty"`
	Mean           float64 `json:"mean"`
	StdDev         float64 `json:"std_dev"`
	LoveShare      float64 `json:"love_share_percent"` // 9-10 votes
	HateShare      float64 `json:"hate_share_percent"` // 1-2 votes
	PolarizedShare float64 `json:"polarized_share_percent"`
	Index          float64 `json:"divisiveness_index"` // 0-100
	Verdict        string  `json:"verdict"`
	Votes          int     `json:"votes"`
}

// Divisiveness computes the polarization reading from a parsed statistics page.
func (s *Stats) Divisiveness() (Divisiveness, error) {
	if len(s.Buckets) == 0 {
		return Divisiveness{}, fmt.Errorf("no score distribution available for %s %d; run the stats command first so the distribution is cached", s.Kind, s.ID)
	}
	d := Divisiveness{Kind: s.Kind, ID: s.ID, Title: s.Title}
	var weighted, weight float64
	for _, b := range s.Buckets {
		weighted += float64(b.Score) * float64(b.Votes)
		weight += float64(b.Votes)
		d.Votes += b.Votes
		switch b.Score {
		case 10, 9:
			d.LoveShare += b.Percent
		case 1, 2:
			d.HateShare += b.Percent
		}
	}
	if weight == 0 {
		return Divisiveness{}, fmt.Errorf("score distribution for %s %d has zero votes", s.Kind, s.ID)
	}
	d.Mean = weighted / weight
	var variance float64
	for _, b := range s.Buckets {
		variance += float64(b.Votes) * math.Pow(float64(b.Score)-d.Mean, 2)
	}
	d.StdDev = math.Sqrt(variance / weight)
	// Divisiveness is about conflict, not enthusiasm: a title with 76% tens and
	// 2% ones is beloved, not split. So the reading is the size of the two
	// opposing camps (both must be populated for conflict to exist), scaled by
	// how wide the rest of the distribution is.
	d.PolarizedShare = 2 * math.Min(d.LoveShare, d.HateShare)
	spread := d.StdDev / 2.5
	if spread > 1 {
		spread = 1
	}
	idx := (d.PolarizedShare / 100.0) * (1 + spread)
	if idx > 1 {
		idx = 1
	}
	d.Index = math.Round(idx*1000) / 10
	switch {
	case d.Index < 15:
		d.Verdict = "broadly loved"
	case d.Index < 30:
		d.Verdict = "mostly agreed"
	case d.Index < 50:
		d.Verdict = "polarizing"
	default:
		d.Verdict = "bitterly split"
	}
	return d, nil
}

// DropRisk estimates abandonment from the status distribution, which is the
// signal a raw average score cannot express.
type DropRisk struct {
	Kind            string  `json:"kind"`
	ID              int     `json:"id"`
	Title           string  `json:"title,omitempty"`
	Episodes        int     `json:"episodes,omitempty"`
	DroppedShare    float64 `json:"dropped_share_percent"`
	OnHoldShare     float64 `json:"on_hold_share_percent"`
	CompletionShare float64 `json:"completion_share_percent"`
	AbandonShare    float64 `json:"abandon_share_percent"`
	RiskBand        string  `json:"risk_band"`
	Total           int     `json:"total"`
}

// DropRisk computes abandonment likelihood for an anime of the given episode
// count (0 when unknown).
func (s *Stats) DropRisk(episodes int) (DropRisk, error) {
	if s.Total == 0 {
		return DropRisk{}, fmt.Errorf("no status distribution available for %s %d; run the stats command first", s.Kind, s.ID)
	}
	d := DropRisk{Kind: s.Kind, ID: s.ID, Title: s.Title, Episodes: episodes, Total: s.Total}
	pct := func(n int) float64 { return math.Round(float64(n)/float64(s.Total)*1000) / 10 }
	d.DroppedShare = pct(s.Dropped)
	d.OnHoldShare = pct(s.OnHold)
	d.CompletionShare = pct(s.Completed)
	d.AbandonShare = math.Round((d.DroppedShare+d.OnHoldShare)*10) / 10
	switch {
	case d.DroppedShare < 3:
		d.RiskBand = "low"
	case d.DroppedShare < 7:
		d.RiskBand = "moderate"
	case d.DroppedShare < 12:
		d.RiskBand = "elevated"
	default:
		d.RiskBand = "high"
	}
	return d, nil
}

// consistencyWindow is the number of episodes in each comparison window, and
// minTrendEpisodes is the smallest sample whose first and last windows do not
// overlap. Below it the two windows contain the same episodes, their means are
// identical by construction, and any slope would be a fabricated zero.
const (
	consistencyWindow = 3
	minTrendEpisodes  = 2 * consistencyWindow
)

// ReceptionCurve describes how episode-level reception moves across a season.
type ReceptionCurve struct {
	Kind          string    `json:"kind"`
	ID            int       `json:"id"`
	Episodes      int       `json:"episodes"`
	MeanPoll      float64   `json:"mean_poll_average"`
	StdDevPoll    float64   `json:"std_dev_poll_average"`
	First3Mean    float64   `json:"first_three_mean"`
	Last3Mean     float64   `json:"last_three_mean"`
	Slope         float64   `json:"slope"`
	SlopeComputed bool      `json:"slope_computed"`
	WorstEpisode  int       `json:"worst_episode,omitempty"`
	WorstPoll     float64   `json:"worst_poll_average,omitempty"`
	BestEpisode   int       `json:"best_episode,omitempty"`
	BestPoll      float64   `json:"best_poll_average,omitempty"`
	RatedEpisodes int       `json:"rated_episodes"`
	Verdict       string    `json:"verdict"`
	Curve         []float64 `json:"curve,omitempty"`
}

// Consistency computes the episode reception curve. Episodes without a poll
// are excluded from every aggregate so an unrated episode cannot masquerade as
// a zero-rated one.
func Consistency(id int, eps []Episode) (ReceptionCurve, error) {
	rated := make([]Episode, 0, len(eps))
	for _, e := range eps {
		if e.PollAverage > 0 {
			rated = append(rated, e)
		}
	}
	if len(rated) == 0 {
		return ReceptionCurve{}, fmt.Errorf("no episode poll averages available for anime %d; poll averages are only published for episodes users voted on", id)
	}
	sort.Slice(rated, func(i, j int) bool { return rated[i].Number < rated[j].Number })
	c := ReceptionCurve{Kind: "anime", ID: id, Episodes: len(eps), RatedEpisodes: len(rated)}
	c.Curve = make([]float64, 0, len(rated))
	var sum float64
	for _, e := range rated {
		c.Curve = append(c.Curve, e.PollAverage)
		sum += e.PollAverage
		if c.BestEpisode == 0 || e.PollAverage > c.BestPoll {
			c.BestEpisode, c.BestPoll = e.Number, e.PollAverage
		}
		if c.WorstEpisode == 0 || e.PollAverage < c.WorstPoll {
			c.WorstEpisode, c.WorstPoll = e.Number, e.PollAverage
		}
	}
	c.MeanPoll = sum / float64(len(rated))
	var variance float64
	for _, e := range rated {
		variance += math.Pow(e.PollAverage-c.MeanPoll, 2)
	}
	c.StdDevPoll = math.Sqrt(variance / float64(len(rated)))
	first, last := consistencyWindow, consistencyWindow
	if len(rated) < first {
		first = len(rated)
	}
	if len(rated) < last {
		last = len(rated)
	}
	c.First3Mean = meanPoll(rated[:first])
	c.Last3Mean = meanPoll(rated[len(rated)-last:])
	// Only report a direction when the two windows do not overlap. With one or
	// two rated episodes they cover the same episodes, so the means match no
	// matter what order the episodes are in — "holds steady" there would be an
	// artifact of the sample size, not a finding. Slope is left at zero and
	// slope_computed=false marks it as not computed.
	c.SlopeComputed = len(rated) >= minTrendEpisodes && first <= len(rated)-last
	if c.SlopeComputed {
		c.Slope = math.Round((c.Last3Mean-c.First3Mean)*100) / 100
		switch {
		case c.Slope > 0.3:
			c.Verdict = "improves"
		case c.Slope < -0.3:
			c.Verdict = "falls off"
		default:
			c.Verdict = "holds steady"
		}
	} else {
		c.Verdict = fmt.Sprintf("insufficient data for a trend (%d rated episode(s); %d non-overlapping are needed)", len(rated), minTrendEpisodes)
	}
	c.MeanPoll = math.Round(c.MeanPoll*100) / 100
	c.StdDevPoll = math.Round(c.StdDevPoll*100) / 100
	c.First3Mean = math.Round(c.First3Mean*100) / 100
	c.Last3Mean = math.Round(c.Last3Mean*100) / 100
	return c, nil
}

func meanPoll(eps []Episode) float64 {
	if len(eps) == 0 {
		return 0
	}
	var sum float64
	for _, e := range eps {
		sum += e.PollAverage
	}
	return sum / float64(len(eps))
}
