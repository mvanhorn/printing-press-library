// Package manifest parses the UFO/UAP CSV manifest from the PURSUE initiative.
package manifest

import (
	"context"
	"crypto/sha256"
	"encoding/csv"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/other/ufo-goat/internal/cliutil"
)

const ManifestURL = "https://raw.githubusercontent.com/DenisSergeevitch/UFO-USA/main/metadata/uap-csv.csv"

// File represents a single declassified UAP file from the CSV manifest.
type File struct {
	ID               string `json:"id"`
	Title            string `json:"title"`
	Type             string `json:"type"` // PDF, VID, IMG
	Agency           string `json:"agency"`
	ReleaseDate      string `json:"release_date"`
	IncidentDate     string `json:"incident_date"`
	ParsedDate       string `json:"parsed_date,omitempty"` // RFC3339 or empty
	IncidentLocation string `json:"incident_location"`
	Description      string `json:"description"`
	Redacted         bool   `json:"redacted"`
	DownloadURL      string `json:"download_url"`
	ThumbnailURL     string `json:"thumbnail_url,omitempty"`
	DVIDSVideoID     string `json:"dvids_video_id,omitempty"`
	VideoTitle       string `json:"video_title,omitempty"`
	VideoPairing     string `json:"video_pairing,omitempty"`
	PDFPairing       string `json:"pdf_pairing,omitempty"`
	ModalImage       string `json:"modal_image,omitempty"`
	PDFImageLink     string `json:"pdf_image_link,omitempty"`
}

// FetchManifest downloads the CSV manifest from GitHub and parses it.
func FetchManifest(ctx context.Context) ([]File, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", ManifestURL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("User-Agent", "ufo-goat-pp-cli/1.0")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching manifest: %w", err)
	}
	defer resp.Body.Close()

	// Retry once on rate limit (HTTP 429) with backoff from Retry-After header.
	if resp.StatusCode == 429 {
		wait := cliutil.RetryAfter(resp)
		resp.Body.Close()
		time.Sleep(wait)
		resp, err = http.DefaultClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("fetching manifest (retry): %w", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode == 429 {
			body, _ := io.ReadAll(resp.Body)
			return nil, &cliutil.RateLimitError{
				URL:        ManifestURL,
				RetryAfter: cliutil.RetryAfter(resp),
				Body:       string(body),
			}
		}
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("manifest fetch returned HTTP %d", resp.StatusCode)
	}

	return ParseCSV(resp.Body)
}

// ParseCSV parses the UAP CSV manifest from a reader.
// CSV columns: Redaction, Release Date, Title, Type, Video Pairing, PDF Pairing,
// Description Blurb, DVIDS Video ID, Video Title, Agency, Incident Date,
// Incident Location, PDF|Image Link, Modal Image
func ParseCSV(reader io.Reader) ([]File, error) {
	r := csv.NewReader(reader)
	r.LazyQuotes = true
	r.TrimLeadingSpace = true
	r.FieldsPerRecord = -1 // allow variable column count

	// Read header row
	header, err := r.Read()
	if err != nil {
		return nil, fmt.Errorf("reading CSV header: %w", err)
	}

	// Build column index map (case-insensitive, trimmed, normalized whitespace)
	colIdx := make(map[string]int)
	for i, h := range header {
		key := strings.TrimSpace(strings.ToLower(h))
		colIdx[key] = i
		// Also store a version with spaces collapsed around pipe characters
		// so "pdf | image link" matches lookup for "pdf|image link" and vice versa
		normalized := strings.ReplaceAll(key, " | ", "|")
		colIdx[normalized] = i
	}

	var files []File
	for {
		record, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			// Skip malformed rows
			continue
		}

		f := File{}

		f.Title = getCol(record, colIdx, "title")
		if f.Title == "" {
			continue // skip rows with no title
		}

		// Generate a stable ID from the title
		f.ID = generateID(f.Title)

		// Redaction
		redactionVal := strings.TrimSpace(getCol(record, colIdx, "redaction"))
		f.Redacted = strings.EqualFold(redactionVal, "yes") ||
			strings.EqualFold(redactionVal, "true") ||
			strings.EqualFold(redactionVal, "redacted") ||
			strings.EqualFold(redactionVal, "partial") ||
			redactionVal == "1"

		f.ReleaseDate = getCol(record, colIdx, "release date")
		f.Type = normalizeType(getCol(record, colIdx, "type"))
		f.VideoPairing = getCol(record, colIdx, "video pairing")
		f.PDFPairing = getCol(record, colIdx, "pdf pairing")
		f.Description = getCol(record, colIdx, "description blurb")
		f.DVIDSVideoID = getCol(record, colIdx, "dvids video id")
		f.VideoTitle = getCol(record, colIdx, "video title")
		f.Agency = normalizeAgency(getCol(record, colIdx, "agency"))
		f.IncidentDate = getCol(record, colIdx, "incident date")
		f.ParsedDate = parseIncidentDate(f.IncidentDate)
		f.IncidentLocation = getCol(record, colIdx, "incident location")
		f.PDFImageLink = getCol(record, colIdx, "pdf|image link")
		f.ModalImage = getCol(record, colIdx, "modal image")

		// Build download URL from PDF/Image link
		if f.PDFImageLink != "" {
			if strings.HasPrefix(f.PDFImageLink, "http") {
				f.DownloadURL = f.PDFImageLink
			} else {
				f.DownloadURL = "https://www.war.gov/medialink/ufo/release_1/" + f.PDFImageLink
			}
		}

		// Build thumbnail URL from modal image
		if f.ModalImage != "" {
			if strings.HasPrefix(f.ModalImage, "http") {
				f.ThumbnailURL = f.ModalImage
			} else {
				f.ThumbnailURL = "https://www.war.gov/medialink/ufo/release_1/" + f.ModalImage
			}
		}

		files = append(files, f)
	}

	return files, nil
}

// getCol safely retrieves a column value by header name.
func getCol(record []string, colIdx map[string]int, name string) string {
	idx, ok := colIdx[name]
	if !ok || idx >= len(record) {
		return ""
	}
	return strings.TrimSpace(record[idx])
}

// generateID creates a short, stable ID from a title.
func generateID(title string) string {
	h := sha256.Sum256([]byte(title))
	return fmt.Sprintf("%x", h[:6]) // 12 hex chars
}

// normalizeType maps raw CSV type values to standardized types.
func normalizeType(t string) string {
	t = strings.TrimSpace(strings.ToUpper(t))
	switch {
	case strings.Contains(t, "PDF"):
		return "PDF"
	case strings.Contains(t, "VID") || strings.Contains(t, "VIDEO"):
		return "VID"
	case strings.Contains(t, "IMG") || strings.Contains(t, "IMAGE") || strings.Contains(t, "PHOTO"):
		return "IMG"
	case t == "":
		return "PDF" // default
	default:
		return t
	}
}

// normalizeAgency normalizes agency names.
func normalizeAgency(a string) string {
	a = strings.TrimSpace(a)
	switch strings.ToLower(a) {
	case "dod", "department of defense", "dow", "department of war":
		return "DoD"
	case "fbi", "federal bureau of investigation":
		return "FBI"
	case "nasa", "national aeronautics and space administration":
		return "NASA"
	case "state", "state department", "department of state", "dos":
		return "State"
	case "cia", "central intelligence agency":
		return "CIA"
	case "":
		return "Unknown"
	default:
		return a
	}
}

var datePatterns = []struct {
	re     *regexp.Regexp
	layout string
}{
	{regexp.MustCompile(`^\d{1,2}/\d{1,2}/\d{4}$`), "1/2/2006"},
	{regexp.MustCompile(`^\d{1,2}/\d{1,2}/\d{2}$`), "1/2/06"},
	{regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`), "2006-01-02"},
	{regexp.MustCompile(`^\w+ \d{1,2}, \d{4}$`), "January 2, 2006"},
	{regexp.MustCompile(`^\w+ \d{4}$`), "January 2006"},
	{regexp.MustCompile(`^\d{4}$`), "2006"},
}

// parseIncidentDate attempts to parse various date formats into RFC3339.
func parseIncidentDate(s string) string {
	s = strings.TrimSpace(s)
	if s == "" || strings.EqualFold(s, "n/a") || strings.EqualFold(s, "unknown") {
		return ""
	}

	// Handle "Late 2025", "Early 1947", etc.
	yearRe := regexp.MustCompile(`(\d{4})`)
	for _, p := range datePatterns {
		if p.re.MatchString(s) {
			t, err := time.Parse(p.layout, s)
			if err == nil {
				// For 2-digit years, Go parses them as 2000+, adjust for pre-2000
				if t.Year() > time.Now().Year()+10 {
					t = t.AddDate(-100, 0, 0)
				}
				return t.Format("2006-01-02")
			}
		}
	}

	// Fallback: extract 4-digit year
	if m := yearRe.FindString(s); m != "" {
		return m + "-01-01"
	}

	return ""
}
