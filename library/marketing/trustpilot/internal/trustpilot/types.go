// Package trustpilot contains the Trustpilot-specific transport, parsing,
// and store extensions used by every Trustpilot CLI command.
package trustpilot

import "time"

// Review mirrors props.pageProps.reviews[N] in the Next.js payload, with
// only the fields the CLI actually persists or surfaces.
type Review struct {
	ID                          string         `json:"id"`
	Domain                      string         `json:"domain"`
	Rating                      int            `json:"rating"`
	Title                       string         `json:"title"`
	Text                        string         `json:"text"`
	Language                    string         `json:"language"`
	Likes                       int            `json:"likes"`
	Source                      string         `json:"source"`
	Filtered                    bool           `json:"filtered"`
	IsPending                   bool           `json:"isPending"`
	IsVerified                  bool           `json:"isVerified"`
	ExperiencedDate             time.Time      `json:"experiencedDate,omitempty"`
	PublishedDate               time.Time      `json:"publishedDate,omitempty"`
	UpdatedDate                 time.Time      `json:"updatedDate,omitempty"`
	ConsumerID                  string         `json:"consumerId"`
	ConsumerName                string         `json:"consumerName"`
	ConsumerCountry             string         `json:"consumerCountry"`
	ConsumerNumberOfReviews     int            `json:"consumerNumberOfReviews"`
	ConsumerHasImage            bool           `json:"consumerHasImage"`
	ReplyMessage                string         `json:"replyMessage,omitempty"`
	ReplyPublishedDate          time.Time      `json:"replyPublishedDate,omitempty"`
	ConsumersReviewCountSameDomain int         `json:"consumersReviewCountSameDomain"`
}

// BusinessUnit mirrors props.pageProps.businessUnit.
type BusinessUnit struct {
	ID                       string             `json:"id"`
	DisplayName              string             `json:"displayName"`
	IdentifyingName          string             `json:"identifyingName"` // canonical domain key
	NumberOfReviews          int                `json:"numberOfReviews"`
	TrustScore               float64            `json:"trustScore"`
	Stars                    float64            `json:"stars"`
	WebsiteURL               string             `json:"websiteUrl"`
	ProfileImageURL          string             `json:"profileImageUrl"`
	IsClaimed                bool               `json:"isClaimed"`
	IsClosed                 bool               `json:"isClosed"`
	IsCollectingReviews      bool               `json:"isCollectingReviews"`
	Categories               []Category         `json:"categories,omitempty"`
	SimilarBusinessUnits     []SimilarBusiness  `json:"similarBusinessUnits,omitempty"`
	AISummary                string             `json:"aiSummary,omitempty"`
	AISummaryModelVersion    string             `json:"aiSummaryModelVersion,omitempty"`
	TopicAISummaries         []TopicSummary     `json:"topicAiSummaries,omitempty"`
	RatingHistogram          map[int]int        `json:"ratingHistogram,omitempty"` // 1..5 -> count
	TotalFilteredReviews     int                `json:"totalFilteredReviews"`
}

type Category struct {
	ID          string `json:"id"`
	DisplayName string `json:"displayName"`
}

type SimilarBusiness struct {
	IdentifyingName string  `json:"identifyingName"`
	DisplayName     string  `json:"displayName"`
	TrustScore      float64 `json:"trustScore"`
	NumberOfReviews int     `json:"numberOfReviews"`
	LogoURL         string  `json:"logoUrl,omitempty"`
}

type TopicSummary struct {
	Topic   string `json:"topic"`
	Summary string `json:"summary"`
}

// Pagination from props.pageProps.filters.pagination.
type Pagination struct {
	CurrentPage int `json:"currentPage"`
	PerPage     int `json:"perPage"`
	TotalCount  int `json:"totalCount"`
	TotalPages  int `json:"totalPages"`
}

// ReviewsPage is what a single Next.js JSON-API page returns (the bits we care about).
type ReviewsPage struct {
	BusinessUnit BusinessUnit `json:"businessUnit"`
	Reviews      []Review     `json:"reviews"`
	Pagination   Pagination   `json:"pagination"`
}

// SearchHit mirrors one entry from pageProps.businessUnits[].
type SearchHit struct {
	IdentifyingName string  `json:"identifyingName"`
	DisplayName     string  `json:"displayName"`
	TrustScore      float64 `json:"trustScore"`
	Stars           float64 `json:"stars"`
	NumberOfReviews int     `json:"numberOfReviews"`
	LogoURL         string  `json:"logoUrl,omitempty"`
}

// Session captures the per-process state needed to make replay calls.
type Session struct {
	AWSWAFToken      string    `json:"awsWafToken"`
	CookieJar        string    `json:"cookieJar"` // full Cookie header value
	ReviewsBuildID   string    `json:"reviewsBuildId"`
	SearchBuildID    string    `json:"searchBuildId"`
	HarvestedAt      time.Time `json:"harvestedAt"`
	UserAgent        string    `json:"userAgent"`
}

// IsFresh reports whether the cookie is still safely replayable. WAF tokens
// live 5-15 minutes; we treat 4 minutes as the safe upper bound to give
// ourselves time on slow runs.
func (s Session) IsFresh() bool {
	if s.AWSWAFToken == "" {
		return false
	}
	return time.Since(s.HarvestedAt) < 4*time.Minute
}
