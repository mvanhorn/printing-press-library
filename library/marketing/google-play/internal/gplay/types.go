package gplay

// App is the full detail view of a Play Store listing.
type App struct {
	AppID         string   `json:"appId"`
	Title         string   `json:"title"`
	Summary       string   `json:"summary,omitempty"`
	Description   string   `json:"description,omitempty"`
	Developer     string   `json:"developer,omitempty"`
	DeveloperID   string   `json:"developerId,omitempty"`
	DeveloperWeb  string   `json:"developerWebsite,omitempty"`
	DeveloperMail string   `json:"developerEmail,omitempty"`
	Genre         string   `json:"genre,omitempty"`
	GenreID       string   `json:"genreId,omitempty"`
	Icon          string   `json:"icon,omitempty"`
	HeaderImage   string   `json:"headerImage,omitempty"`
	Screenshots   []string `json:"screenshots,omitempty"`
	Video         string   `json:"video,omitempty"`
	Score         float64  `json:"score"`
	ScoreText     string   `json:"scoreText,omitempty"`
	Ratings       int64    `json:"ratings"`
	Reviews       int64    `json:"reviews"`
	Histogram     [5]int64 `json:"histogram"`
	Installs      string   `json:"installs,omitempty"`
	MinInstalls   int64    `json:"minInstalls,omitempty"`
	RealInstalls  int64    `json:"realInstalls,omitempty"`
	Price         float64  `json:"price"`
	Currency      string   `json:"currency,omitempty"`
	Free          bool     `json:"free"`
	OffersIAP     bool     `json:"offersIAP"`
	IAPRange      string   `json:"iapRange,omitempty"`
	ContainsAds   bool     `json:"containsAds"`
	ContentRating string   `json:"contentRating,omitempty"`
	Released      string   `json:"released,omitempty"`
	Updated       int64    `json:"updated,omitempty"`
	Version       string   `json:"version,omitempty"`
	RecentChanges string   `json:"recentChanges,omitempty"`
	AndroidVer    string   `json:"androidVersion,omitempty"`
	PrivacyPolicy string   `json:"privacyPolicy,omitempty"`
	URL           string   `json:"url,omitempty"`
}

// LiteApp is the compact listing shape returned by charts, search, similar,
// and developer endpoints.
type LiteApp struct {
	AppID     string  `json:"appId"`
	Title     string  `json:"title"`
	Developer string  `json:"developer,omitempty"`
	Icon      string  `json:"icon,omitempty"`
	Score     float64 `json:"score,omitempty"`
	ScoreText string  `json:"scoreText,omitempty"`
	Price     float64 `json:"price"`
	Currency  string  `json:"currency,omitempty"`
	Free      bool    `json:"free"`
	Summary   string  `json:"summary,omitempty"`
	URL       string  `json:"url,omitempty"`
}

// Review is a single user review.
type Review struct {
	ID        string `json:"id"`
	UserName  string `json:"userName,omitempty"`
	Score     int    `json:"score"`
	Text      string `json:"text,omitempty"`
	At        int64  `json:"at,omitempty"` // unix seconds
	ThumbsUp  int    `json:"thumbsUp,omitempty"`
	Version   string `json:"version,omitempty"`
	ReplyText string `json:"replyText,omitempty"`
	RepliedAt int64  `json:"repliedAt,omitempty"`
}

// Permission is one declared Android permission.
type Permission struct {
	Group      string `json:"group"`
	Permission string `json:"permission"`
}

// DataSafetyEntry is one shared/collected data item.
type DataSafetyEntry struct {
	Data     string `json:"data"`
	Optional bool   `json:"optional,omitempty"`
	Purpose  string `json:"purpose,omitempty"`
	Type     string `json:"type,omitempty"`
}

// DataSafety is the privacy/data-safety summary.
type DataSafety struct {
	DataShared       []DataSafetyEntry `json:"dataShared"`
	DataCollected    []DataSafetyEntry `json:"dataCollected"`
	SecurityComments []string          `json:"securityPractices,omitempty"`
	PrivacyPolicyURL string            `json:"privacyPolicyUrl,omitempty"`
}
