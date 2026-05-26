package cli

// PATCH: Shared helpers for Blu-ray.com Phase 3 sitemap, HTML, and local-catalog commands.

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/url"
	"path"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/blu-ray/internal/client"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/blu-ray/internal/store"
	xhtml "golang.org/x/net/html"
	"golang.org/x/text/encoding/charmap"
)

const bluRayBrowserUA = "Mozilla/5.0 (Macintosh; Intel Mac OS X 14_6) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0.0.0 Safari/537.36"

var (
	releaseURLRE = regexp.MustCompile(`/([^/]+)/([^/]+)/(\d+)/?$`)
	movieURLRE   = regexp.MustCompile(`/movies/([^/]+)/(\d+)/?$`)
	newsIDRE     = regexp.MustCompile(`[?&]id=(\d+)`)
	yearRE       = regexp.MustCompile(`(?:^|[-(])((?:19|20)\d{2})(?:\)|$|-)`)
)

type catalogRelease struct {
	ID          int    `json:"id"`
	Kind        string `json:"kind"`
	Title       string `json:"title"`
	Slug        string `json:"slug"`
	Year        int    `json:"year,omitempty"`
	Country     string `json:"country,omitempty"`
	Distributor string `json:"distributor,omitempty"`
	URL         string `json:"url"`
}

func openBluRayStore(ctx context.Context) (*store.Store, error) {
	db, err := store.OpenWithContext(ctx, defaultDBPath("blu-ray-pp-cli"))
	if err != nil {
		return nil, err
	}
	if err := db.MigrateBluRayCatalog(); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

func bluRayHeaders(binary bool) map[string]string {
	h := map[string]string{
		"User-Agent": bluRayBrowserUA,
		"Accept":     "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8",
	}
	if binary {
		h[client.BinaryResponseHeader] = "true"
		h["Accept"] = "*/*"
	}
	return h
}

func bluRayGet(c *client.Client, rawURL string, binary bool) ([]byte, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}
	expectedHost := ""
	if base, err := url.Parse(c.BaseURL); err == nil {
		expectedHost = base.Host
	}
	if u.Host != "" && expectedHost != "" && !strings.EqualFold(u.Host, expectedHost) {
		return nil, fmt.Errorf("bluRayGet: URL host %q does not match expected %q", u.Host, expectedHost)
	}
	p := u.Path
	data, err := c.GetWithHeadersValues(p, u.Query(), bluRayHeaders(binary))
	return []byte(data), err
}

func decodeLatin1(raw []byte) string {
	// PATCH: Use the x/text decoder to avoid per-byte rune widening on large pages.
	decoded, err := charmap.ISO8859_1.NewDecoder().Bytes(raw)
	if err != nil {
		return string(raw)
	}
	return string(decoded)
}

func parseHTMLLatin1(raw []byte) (*xhtml.Node, error) {
	return xhtml.Parse(strings.NewReader(decodeLatin1(raw)))
}

func gunzipBytes(raw []byte) ([]byte, error) {
	r, err := gzip.NewReader(bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return io.ReadAll(r)
}

func parseReleaseURL(raw string) (kind, slug string, id int, ok bool) {
	u, err := url.Parse(raw)
	if err != nil {
		return "", "", 0, false
	}
	m := releaseURLRE.FindStringSubmatch(u.Path)
	if len(m) != 4 {
		return "", "", 0, false
	}
	id, _ = strconv.Atoi(m[3])
	return kindFromPathAndSlug(m[1], m[2]), m[2], id, id > 0
}

func kindFromPathAndSlug(section, slug string) string {
	lower := strings.ToLower(slug)
	switch section {
	case "dvd":
		return "dvd"
	case "digital":
		return "digital"
	case "itunes":
		return "itunes"
	case "ma":
		return "ma"
	case "uv":
		return "uv"
	}
	switch {
	case strings.Contains(lower, "4k-blu-ray"):
		return "4k"
	case strings.Contains(lower, "3d-blu-ray"):
		return "3d"
	default:
		return "bluray"
	}
}

func titleFromSlug(slug string) string {
	s := strings.TrimSpace(slug)
	for _, suffix := range []string{"-4K-Blu-ray", "-3D-Blu-ray", "-Blu-ray", "-DVD", "-Digital", "-iTunes"} {
		s = strings.TrimSuffix(s, suffix)
	}
	s = strings.ReplaceAll(s, "-", " ")
	return strings.Join(strings.Fields(s), " ")
}

func yearFromSlug(slug string) int {
	m := yearRE.FindStringSubmatch(slug)
	if len(m) != 2 {
		return 0
	}
	year, _ := strconv.Atoi(m[1])
	return year
}

func releaseURL(kind, slug string, id int) string {
	section := "movies"
	switch kind {
	case "dvd", "digital", "itunes", "ma", "uv":
		section = kind
	}
	return fmt.Sprintf("https://www.blu-ray.com/%s/%s/%d/", section, slug, id)
}

func firstText(n *xhtml.Node, tag string) string {
	var out string
	walkHTML(n, func(x *xhtml.Node) {
		if out == "" && x.Type == xhtml.ElementNode && strings.EqualFold(x.Data, tag) {
			out = cleanHTMLText(nodeText(x))
		}
	})
	return out
}

func absoluteBluRayURL(raw string) string {
	if raw == "" {
		return ""
	}
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	if u.IsAbs() {
		return u.String()
	}
	base, _ := url.Parse("https://www.blu-ray.com/")
	return base.ResolveReference(u).String()
}

func hashLines(lines []string) string {
	h := sha256.New()
	for _, line := range lines {
		io.WriteString(h, line)
		io.WriteString(h, "\n")
	}
	return hex.EncodeToString(h.Sum(nil))
}

func nullStringValue(ns sql.NullString) string {
	if ns.Valid {
		return ns.String
	}
	return ""
}

func nullFloatValue(nf sql.NullFloat64) float64 {
	if nf.Valid {
		return nf.Float64
	}
	return 0
}

func formatPrice(price float64) string {
	if price == 0 {
		return ""
	}
	return fmt.Sprintf("$%.2f", price)
}

func sitemapName(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return path.Base(raw)
	}
	return path.Base(u.Path)
}

func parseSitemapLocs(raw []byte) ([]string, error) {
	type locOnly struct {
		Locs []string `xml:"url>loc"`
	}
	var s locOnly
	// PATCH: Route sitemap XML through the permissive decoder used by sync.
	if err := decodePermissiveXML(raw, &s); err != nil {
		return nil, err
	}
	return s.Locs, nil
}

func parseSitemapIndex(raw []byte) ([]string, error) {
	type index struct {
		Locs []string `xml:"sitemap>loc"`
	}
	var s index
	// PATCH: Route sitemap XML through the permissive decoder used by sync.
	if err := decodePermissiveXML(raw, &s); err != nil {
		return nil, err
	}
	return s.Locs, nil
}

// PATCH: Blu-ray.com XML sitemaps sometimes contain latin-1 or invalid UTF-8 bytes.
func newPermissiveXMLDecoder(r io.Reader) *xml.Decoder {
	dec := xml.NewDecoder(r)
	dec.CharsetReader = func(charset string, input io.Reader) (io.Reader, error) {
		switch strings.ToLower(strings.TrimSpace(charset)) {
		case "", "utf-8", "utf8":
			return input, nil
		case "iso-8859-1", "latin1", "windows-1252", "cp1252":
			return latin1Reader(input)
		default:
			return input, nil
		}
	}
	dec.Strict = false
	return dec
}

// PATCH: Retry XML decode with invalid UTF-8 bytes replaced for mislabeled sitemap bodies.
func decodePermissiveXML(raw []byte, v any) error {
	if err := newPermissiveXMLDecoder(bytes.NewReader(raw)).Decode(v); err != nil {
		valid := strings.ToValidUTF8(string(raw), "?")
		resetDecodeTarget(v)
		if retryErr := newPermissiveXMLDecoder(strings.NewReader(valid)).Decode(v); retryErr != nil {
			return errors.Join(err, retryErr)
		}
	}
	return nil
}

// PATCH: Clear partially decoded sitemap data before fallback XML decode retries.
func resetDecodeTarget(v any) {
	rv := reflect.ValueOf(v)
	if rv.Kind() == reflect.Pointer && !rv.IsNil() {
		rv.Elem().Set(reflect.Zero(rv.Elem().Type()))
	}
}

// PATCH: Minimal latin-1/cp1252 decoder avoids adding a generated-tree dependency.
func latin1Reader(input io.Reader) (io.Reader, error) {
	data, err := io.ReadAll(input)
	if err != nil {
		return nil, err
	}
	runes := make([]rune, 0, len(data))
	for _, b := range data {
		runes = append(runes, rune(b))
	}
	return strings.NewReader(string(runes)), nil
}

func nowText() string {
	// PATCH: Preserve distinct rapid price observations in the TEXT primary key.
	return time.Now().UTC().Format(time.RFC3339Nano)
}
