package trustpilot

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"
)

var nextDataRE = regexp.MustCompile(`(?s)<script id="__NEXT_DATA__"[^>]*>(.*?)</script>`)

// ParseNextDataHTML pulls the __NEXT_DATA__ JSON blob out of a Trustpilot
// review or search HTML response. Returns the buildId and the raw pageProps
// JSON for callers that want to decode their own shape (the shape differs
// between review pages and search pages).
func ParseNextDataHTML(body []byte) (buildID string, pageProps json.RawMessage, err error) {
	m := nextDataRE.FindSubmatch(body)
	if len(m) < 2 {
		return "", nil, fmt.Errorf("no __NEXT_DATA__ script tag in response")
	}
	var envelope struct {
		BuildID string `json:"buildId"`
		Props   struct {
			PageProps json.RawMessage `json:"pageProps"`
		} `json:"props"`
	}
	if err := json.Unmarshal(m[1], &envelope); err != nil {
		return "", nil, fmt.Errorf("decoding __NEXT_DATA__: %w", err)
	}
	return envelope.BuildID, envelope.Props.PageProps, nil
}

// ParseReviewsPage decodes the pageProps blob returned by either the HTML or
// the /_next/data review endpoint into our compact ReviewsPage form.
func ParseReviewsPage(domain string, pageProps json.RawMessage) (ReviewsPage, error) {
	var raw struct {
		BusinessUnit struct {
			ID                  string  `json:"id"`
			DisplayName         string  `json:"displayName"`
			IdentifyingName     string  `json:"identifyingName"`
			NumberOfReviews     int     `json:"numberOfReviews"`
			TrustScore          float64 `json:"trustScore"`
			Stars               float64 `json:"stars"`
			WebsiteURL          string  `json:"websiteUrl"`
			ProfileImageURL     string  `json:"profileImageUrl"`
			IsClaimed           bool    `json:"isClaimed"`
			IsClosed            bool    `json:"isClosed"`
			IsCollectingReviews bool    `json:"isCollectingReviews"`
			Categories          []struct {
				ID          string `json:"id"`
				DisplayName string `json:"displayName"`
			} `json:"categories"`
		} `json:"businessUnit"`
		Reviews              []rawReview `json:"reviews"`
		SimilarBusinessUnits []struct {
			IdentifyingName string  `json:"identifyingName"`
			DisplayName     string  `json:"displayName"`
			TrustScore      float64 `json:"trustScore"`
			NumberOfReviews int     `json:"numberOfReviews"`
			LogoURL         string  `json:"logoUrl"`
		} `json:"similarBusinessUnits"`
		AISummary struct {
			Summary      string `json:"summary"`
			ModelVersion string `json:"modelVersion"`
		} `json:"aiSummary"`
		TopicAiSummaries []struct {
			Topic   string `json:"topic"`
			Summary string `json:"summary"`
		} `json:"topicAiSummaries"`
		Filters struct {
			TotalNumberOfFilteredReviews int `json:"totalNumberOfFilteredReviews"`
			Pagination                   struct {
				CurrentPage int `json:"currentPage"`
				PerPage     int `json:"perPage"`
				TotalCount  int `json:"totalCount"`
				TotalPages  int `json:"totalPages"`
			} `json:"pagination"`
			ReviewStatistics struct {
				Ratings map[string]int `json:"ratings"`
			} `json:"reviewStatistics"`
		} `json:"filters"`
	}
	if err := json.Unmarshal(pageProps, &raw); err != nil {
		return ReviewsPage{}, fmt.Errorf("decoding reviews pageProps: %w", err)
	}

	page := ReviewsPage{
		BusinessUnit: BusinessUnit{
			ID:                    raw.BusinessUnit.ID,
			DisplayName:           raw.BusinessUnit.DisplayName,
			IdentifyingName:       raw.BusinessUnit.IdentifyingName,
			NumberOfReviews:       raw.BusinessUnit.NumberOfReviews,
			TrustScore:            raw.BusinessUnit.TrustScore,
			Stars:                 raw.BusinessUnit.Stars,
			WebsiteURL:            raw.BusinessUnit.WebsiteURL,
			ProfileImageURL:       raw.BusinessUnit.ProfileImageURL,
			IsClaimed:             raw.BusinessUnit.IsClaimed,
			IsClosed:              raw.BusinessUnit.IsClosed,
			IsCollectingReviews:   raw.BusinessUnit.IsCollectingReviews,
			AISummary:             raw.AISummary.Summary,
			AISummaryModelVersion: raw.AISummary.ModelVersion,
			TotalFilteredReviews:  raw.Filters.TotalNumberOfFilteredReviews,
			RatingHistogram:       parseHistogram(raw.Filters.ReviewStatistics.Ratings),
		},
		Pagination: Pagination(raw.Filters.Pagination),
	}
	if raw.BusinessUnit.IdentifyingName != "" {
		domain = raw.BusinessUnit.IdentifyingName
	}
	for _, c := range raw.BusinessUnit.Categories {
		page.BusinessUnit.Categories = append(page.BusinessUnit.Categories, Category(c))
	}
	for _, s := range raw.SimilarBusinessUnits {
		page.BusinessUnit.SimilarBusinessUnits = append(page.BusinessUnit.SimilarBusinessUnits, SimilarBusiness{
			IdentifyingName: s.IdentifyingName,
			DisplayName:     s.DisplayName,
			TrustScore:      s.TrustScore,
			NumberOfReviews: s.NumberOfReviews,
			LogoURL:         s.LogoURL,
		})
	}
	for _, t := range raw.TopicAiSummaries {
		page.BusinessUnit.TopicAISummaries = append(page.BusinessUnit.TopicAISummaries, TopicSummary{
			Topic:   t.Topic,
			Summary: t.Summary,
		})
	}
	for _, rr := range raw.Reviews {
		page.Reviews = append(page.Reviews, rr.toReview(domain))
	}
	return page, nil
}

// rawReview decouples the on-wire schema (which has dates wrapped in a sub-object
// and consumer wrapped in another) from our flatter Review type.
type rawReview struct {
	ID        string `json:"id"`
	Rating    int    `json:"rating"`
	Title     string `json:"title"`
	Text      string `json:"text"`
	Language  string `json:"language"`
	Likes     int    `json:"likes"`
	Source    string `json:"source"`
	Filtered  bool   `json:"filtered"`
	IsPending bool   `json:"isPending"`
	Labels    struct {
		Verification struct {
			IsVerified bool `json:"isVerified"`
		} `json:"verification"`
	} `json:"labels"`
	Dates struct {
		ExperiencedDate string `json:"experiencedDate"`
		PublishedDate   string `json:"publishedDate"`
		UpdatedDate     string `json:"updatedDate"`
	} `json:"dates"`
	Consumer struct {
		ID              string `json:"id"`
		DisplayName     string `json:"displayName"`
		CountryCode     string `json:"countryCode"`
		NumberOfReviews int    `json:"numberOfReviews"`
		HasImage        bool   `json:"hasImage"`
	} `json:"consumer"`
	Reply *struct {
		Message       string `json:"message"`
		PublishedDate string `json:"publishedDate"`
	} `json:"reply"`
	ConsumersReviewCountOnSameDomain int `json:"consumersReviewCountOnSameDomain"`
}

func (rr rawReview) toReview(domain string) Review {
	r := Review{
		ID:                          rr.ID,
		Domain:                      domain,
		Rating:                      rr.Rating,
		Title:                       rr.Title,
		Text:                        rr.Text,
		Language:                    rr.Language,
		Likes:                       rr.Likes,
		Source:                      rr.Source,
		Filtered:                    rr.Filtered,
		IsPending:                   rr.IsPending,
		IsVerified:                  rr.Labels.Verification.IsVerified,
		ExperiencedDate:             parseTime(rr.Dates.ExperiencedDate),
		PublishedDate:               parseTime(rr.Dates.PublishedDate),
		UpdatedDate:                 parseTime(rr.Dates.UpdatedDate),
		ConsumerID:                  rr.Consumer.ID,
		ConsumerName:                rr.Consumer.DisplayName,
		ConsumerCountry:             rr.Consumer.CountryCode,
		ConsumerNumberOfReviews:     rr.Consumer.NumberOfReviews,
		ConsumerHasImage:            rr.Consumer.HasImage,
		ConsumersReviewCountSameDomain: rr.ConsumersReviewCountOnSameDomain,
	}
	if rr.Reply != nil {
		r.ReplyMessage = rr.Reply.Message
		r.ReplyPublishedDate = parseTime(rr.Reply.PublishedDate)
	}
	return r
}

func parseTime(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
		return t
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t
	}
	return time.Time{}
}

func parseHistogram(m map[string]int) map[int]int {
	if len(m) == 0 {
		return nil
	}
	out := make(map[int]int, len(m))
	for k, v := range m {
		switch k {
		case "fiveStars", "5":
			out[5] = v
		case "fourStars", "4":
			out[4] = v
		case "threeStars", "3":
			out[3] = v
		case "twoStars", "2":
			out[2] = v
		case "oneStar", "1":
			out[1] = v
		default:
			// Ignore unrecognized keys; the API has used several shapes over the years.
			_ = strings.TrimSpace
		}
	}
	return out
}

// ParseSearchPage decodes the pageProps blob from a Trustpilot search response.
func ParseSearchPage(pageProps json.RawMessage) ([]SearchHit, error) {
	// Two known shapes:
	//   1. /_next/data/<searchBuild>/search.json returns pageProps.businessUnits = [ ... ]
	//   2. /search HTML returns pageProps.businessUnits = [ ... ] (same flat list)
	var raw struct {
		BusinessUnits []struct {
			IdentifyingName string  `json:"identifyingName"`
			DisplayName     string  `json:"displayName"`
			TrustScore      float64 `json:"trustScore"`
			Stars           float64 `json:"stars"`
			NumberOfReviews int     `json:"numberOfReviews"`
			LogoURL         string  `json:"logoUrl"`
		} `json:"businessUnits"`
	}
	if err := json.Unmarshal(pageProps, &raw); err != nil {
		return nil, fmt.Errorf("decoding search pageProps: %w", err)
	}
	out := make([]SearchHit, 0, len(raw.BusinessUnits))
	for _, b := range raw.BusinessUnits {
		out = append(out, SearchHit{
			IdentifyingName: b.IdentifyingName,
			DisplayName:     b.DisplayName,
			TrustScore:      b.TrustScore,
			Stars:           b.Stars,
			NumberOfReviews: b.NumberOfReviews,
			LogoURL:         b.LogoURL,
		})
	}
	return out, nil
}
