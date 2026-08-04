package client

// Credentials holds the result of a successful Tableau REST sign-in.
type Credentials struct {
	Token     string
	SiteID    string
	ContentURL string
	UserID    string
}

// Project is a Tableau project on a site.
type Project struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	ParentID    string `json:"parentProjectId,omitempty"`
}

// Workbook is a published workbook summary.
type Workbook struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	ContentURL   string `json:"contentUrl,omitempty"`
	ProjectID    string `json:"projectId,omitempty"`
	ProjectName  string `json:"projectName,omitempty"`
	OwnerID      string `json:"ownerId,omitempty"`
	WebpageURL   string `json:"webpageUrl,omitempty"`
	Size         string `json:"size,omitempty"`
	CreatedAt    string `json:"createdAt,omitempty"`
	UpdatedAt    string `json:"updatedAt,omitempty"`
}

// Site is a Tableau site summary.
type Site struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	ContentURL string `json:"contentUrl"`
	State      string `json:"state,omitempty"`
}

// PublishResult is returned after a successful workbook publish.
type PublishResult struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	ContentURL string `json:"contentUrl,omitempty"`
	ProjectID  string `json:"projectId,omitempty"`
}
