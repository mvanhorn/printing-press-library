package client

import (
	"encoding/xml"
	"fmt"
	"io"
	"strings"
)

// XML response shapes for Tableau REST API.
// Attribute names match Tableau's tsResponse schema.

type tsResponse struct {
	XMLName     xml.Name        `xml:"tsResponse"`
	Credentials *xmlCredentials `xml:"credentials"`
	Projects    *xmlProjects    `xml:"projects"`
	Workbooks   *xmlWorkbooks   `xml:"workbooks"`
	Workbook    *xmlWorkbook    `xml:"workbook"`
	Sites       *xmlSites       `xml:"sites"`
	Site        *xmlSite        `xml:"site"`
	Pagination  *xmlPagination  `xml:"pagination"`
	Error       *xmlError       `xml:"error"`
	FileUpload  *xmlFileUpload  `xml:"fileUpload"`
}

type xmlCredentials struct {
	Token string  `xml:"token,attr"`
	Site  xmlSite `xml:"site"`
	User  xmlUser `xml:"user"`
}

type xmlSite struct {
	ID         string `xml:"id,attr"`
	Name       string `xml:"name,attr"`
	ContentURL string `xml:"contentUrl,attr"`
	State      string `xml:"state,attr"`
}

type xmlUser struct {
	ID string `xml:"id,attr"`
}

type xmlProjects struct {
	Project []xmlProject `xml:"project"`
}

type xmlProject struct {
	ID              string `xml:"id,attr"`
	Name            string `xml:"name,attr"`
	Description     string `xml:"description,attr"`
	ParentProjectID string `xml:"parentProjectId,attr"`
}

type xmlWorkbooks struct {
	Workbook []xmlWorkbook `xml:"workbook"`
}

type xmlWorkbook struct {
	ID         string     `xml:"id,attr"`
	Name       string     `xml:"name,attr"`
	ContentURL string     `xml:"contentUrl,attr"`
	WebpageURL string     `xml:"webpageUrl,attr"`
	Size       string     `xml:"size,attr"`
	CreatedAt  string     `xml:"createdAt,attr"`
	UpdatedAt  string     `xml:"updatedAt,attr"`
	Project    xmlProject `xml:"project"`
	Owner      xmlUser    `xml:"owner"`
}

type xmlSites struct {
	Site []xmlSite `xml:"site"`
}

type xmlPagination struct {
	PageNumber     string `xml:"pageNumber,attr"`
	PageSize       string `xml:"pageSize,attr"`
	TotalAvailable string `xml:"totalAvailable,attr"`
}

type xmlError struct {
	Code    string `xml:"code,attr"`
	Summary string `xml:"summary"`
	Detail  string `xml:"detail"`
}

type xmlFileUpload struct {
	UploadSessionID string `xml:"uploadSessionId,attr"`
	FileSize        string `xml:"fileSize,attr"`
}

// ParseSignInResponse parses a Tableau auth/signin XML body into Credentials.
func ParseSignInResponse(r io.Reader) (*Credentials, error) {
	var resp tsResponse
	if err := xml.NewDecoder(r).Decode(&resp); err != nil {
		return nil, fmt.Errorf("decode sign-in response: %w", err)
	}
	if resp.Error != nil {
		return nil, apiError(resp.Error)
	}
	if resp.Credentials == nil || resp.Credentials.Token == "" {
		return nil, fmt.Errorf("sign-in response missing credentials token")
	}
	return &Credentials{
		Token:      resp.Credentials.Token,
		SiteID:     resp.Credentials.Site.ID,
		ContentURL: resp.Credentials.Site.ContentURL,
		UserID:     resp.Credentials.User.ID,
	}, nil
}

// ParseProjectsResponse parses a projects list XML body.
func ParseProjectsResponse(r io.Reader) ([]Project, *xmlPagination, error) {
	var resp tsResponse
	if err := xml.NewDecoder(r).Decode(&resp); err != nil {
		return nil, nil, fmt.Errorf("decode projects response: %w", err)
	}
	if resp.Error != nil {
		return nil, nil, apiError(resp.Error)
	}
	var out []Project
	if resp.Projects != nil {
		for _, p := range resp.Projects.Project {
			out = append(out, Project{
				ID:          p.ID,
				Name:        p.Name,
				Description: p.Description,
				ParentID:    p.ParentProjectID,
			})
		}
	}
	return out, resp.Pagination, nil
}

// ParseWorkbooksResponse parses a workbooks list or single-workbook XML body.
func ParseWorkbooksResponse(r io.Reader) ([]Workbook, *xmlPagination, error) {
	var resp tsResponse
	if err := xml.NewDecoder(r).Decode(&resp); err != nil {
		return nil, nil, fmt.Errorf("decode workbooks response: %w", err)
	}
	if resp.Error != nil {
		return nil, nil, apiError(resp.Error)
	}
	var out []Workbook
	if resp.Workbooks != nil {
		for _, w := range resp.Workbooks.Workbook {
			out = append(out, workbookFromXML(w))
		}
	}
	if resp.Workbook != nil {
		out = append(out, workbookFromXML(*resp.Workbook))
	}
	return out, resp.Pagination, nil
}

// ParsePublishResponse parses a publish workbook XML body.
func ParsePublishResponse(r io.Reader) (*PublishResult, error) {
	workbooks, _, err := ParseWorkbooksResponse(r)
	if err != nil {
		return nil, err
	}
	if len(workbooks) == 0 {
		return nil, fmt.Errorf("publish response missing workbook")
	}
	w := workbooks[0]
	return &PublishResult{
		ID:         w.ID,
		Name:       w.Name,
		ContentURL: w.ContentURL,
		ProjectID:  w.ProjectID,
	}, nil
}

// ParseSitesResponse parses a sites list XML body.
func ParseSitesResponse(r io.Reader) ([]Site, *xmlPagination, error) {
	var resp tsResponse
	if err := xml.NewDecoder(r).Decode(&resp); err != nil {
		return nil, nil, fmt.Errorf("decode sites response: %w", err)
	}
	if resp.Error != nil {
		return nil, nil, apiError(resp.Error)
	}
	var out []Site
	if resp.Sites != nil {
		for _, s := range resp.Sites.Site {
			out = append(out, Site{
				ID:         s.ID,
				Name:       s.Name,
				ContentURL: s.ContentURL,
				State:      s.State,
			})
		}
	}
	if resp.Site != nil {
		out = append(out, Site{
			ID:         resp.Site.ID,
			Name:       resp.Site.Name,
			ContentURL: resp.Site.ContentURL,
			State:      resp.Site.State,
		})
	}
	return out, resp.Pagination, nil
}

// ParseFileUploadResponse parses initiate/append file upload XML.
func ParseFileUploadResponse(r io.Reader) (sessionID string, err error) {
	var resp tsResponse
	if err := xml.NewDecoder(r).Decode(&resp); err != nil {
		return "", fmt.Errorf("decode file upload response: %w", err)
	}
	if resp.Error != nil {
		return "", apiError(resp.Error)
	}
	if resp.FileUpload == nil || resp.FileUpload.UploadSessionID == "" {
		return "", fmt.Errorf("file upload response missing uploadSessionId")
	}
	return resp.FileUpload.UploadSessionID, nil
}

// ParseErrorResponse extracts a Tableau error from an error response body.
func ParseErrorResponse(r io.Reader) error {
	var resp tsResponse
	if err := xml.NewDecoder(r).Decode(&resp); err != nil {
		return nil // not XML / not a Tableau error body
	}
	if resp.Error != nil {
		return apiError(resp.Error)
	}
	return nil
}

func workbookFromXML(w xmlWorkbook) Workbook {
	return Workbook{
		ID:          w.ID,
		Name:        w.Name,
		ContentURL:  w.ContentURL,
		ProjectID:   w.Project.ID,
		ProjectName: w.Project.Name,
		OwnerID:     w.Owner.ID,
		WebpageURL:  w.WebpageURL,
		Size:        w.Size,
		CreatedAt:   w.CreatedAt,
		UpdatedAt:   w.UpdatedAt,
	}
}

func apiError(e *xmlError) error {
	parts := []string{}
	if e.Code != "" {
		parts = append(parts, "code "+e.Code)
	}
	if e.Summary != "" {
		parts = append(parts, e.Summary)
	}
	if e.Detail != "" {
		parts = append(parts, e.Detail)
	}
	if len(parts) == 0 {
		return fmt.Errorf("tableau API error")
	}
	return fmt.Errorf("tableau API error: %s", strings.Join(parts, ": "))
}

// BuildSignInPATRequest builds the XML body for PAT-based sign-in.
func BuildSignInPATRequest(patName, patSecret, siteContentURL string) string {
	// contentUrl="" is the default site on Tableau Server.
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>`)
	b.WriteString(`<tsRequest>`)
	b.WriteString(`<credentials personalAccessTokenName="`)
	b.WriteString(xmlEscapeAttr(patName))
	b.WriteString(`" personalAccessTokenSecret="`)
	b.WriteString(xmlEscapeAttr(patSecret))
	b.WriteString(`">`)
	b.WriteString(`<site contentUrl="`)
	b.WriteString(xmlEscapeAttr(siteContentURL))
	b.WriteString(`" />`)
	b.WriteString(`</credentials>`)
	b.WriteString(`</tsRequest>`)
	return b.String()
}

// BuildPublishWorkbookPayload builds the request_payload XML for publish.
func BuildPublishWorkbookPayload(name, projectID string) string {
	var b strings.Builder
	b.WriteString(`<tsRequest><workbook name="`)
	b.WriteString(xmlEscapeAttr(name))
	b.WriteString(`"><project id="`)
	b.WriteString(xmlEscapeAttr(projectID))
	b.WriteString(`"/></workbook></tsRequest>`)
	return b.String()
}

func xmlEscapeAttr(s string) string {
	var b strings.Builder
	xml.EscapeText(&b, []byte(s))
	return b.String()
}
