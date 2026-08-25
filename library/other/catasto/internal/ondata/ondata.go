// Copyright 2026 roberto-bissanti. Licensed under Apache-2.0. See LICENSE.

// Package ondata fetches and queries the ondata/dati_catastali Parquet
// dataset from GitHub. It maps Italian cadastral references
// (comune codice belfiore + foglio + particella) to WGS84 lon/lat
// centroids precomputed from the Agenzia delle Entrate WFS.
//
// Two Parquet sources:
//   - index.parquet: comune (codice belfiore) -> regional file name + ISTAT code + display name.
//   - <NN>_<Region>.parquet: per-region table keyed by comune+foglio+particella, with integer microdegree x/y.
//
// Coordinates in the Parquet files are stored as int x and int y in
// microdegrees (EPSG:6706 ≈ WGS84 for practical purposes). Divide by
// 1_000_000 to get decimal degrees.
package ondata

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/parquet-go/parquet-go"
)

const (
	BaseURL          = "https://raw.githubusercontent.com/ondata/dati_catastali/main/S_0000_ITALIA/anagrafica"
	IndexFileName    = "index.parquet"
	defaultTimeout   = 30 * time.Second
	parquetSizeHint  = 1 << 30 // 1GB upper bound on a regional file
	streamBufferSize = 16 << 20
)

// IndexEntry is one row of index.parquet.
type IndexEntry struct {
	Comune          string `parquet:"comune"`           // codice belfiore (e.g. H501)
	File            string `parquet:"file"`             // regional Parquet filename (e.g. 12_Lazio.parquet)
	CODISTAT        string `parquet:"CODISTAT"`         // ISTAT code (zero-padded)
	DenominazioneIT string `parquet:"DENOMINAZIONE_IT"` // display name in Italian (e.g. ROMA)
}

// ParcelRow is one row of a regional <NN>_<Region>.parquet file.
type ParcelRow struct {
	InspireID  string `parquet:"INSPIREID_LOCALID"`
	Comune     string `parquet:"comune"`
	Foglio     string `parquet:"foglio"`
	Particella string `parquet:"particella"`
	X          int64  `parquet:"x"`
	Y          int64  `parquet:"y"`
}

// Lon returns the WGS84 longitude in decimal degrees.
func (r ParcelRow) Lon() float64 { return float64(r.X) / 1_000_000.0 }

// Lat returns the WGS84 latitude in decimal degrees.
func (r ParcelRow) Lat() float64 { return float64(r.Y) / 1_000_000.0 }

// ParcelResult is the user-facing output shape.
type ParcelResult struct {
	InspireID  string  `json:"inspire_id"`
	Comune     string  `json:"comune"`
	Foglio     string  `json:"foglio"`
	Particella string  `json:"particella"`
	Lon        float64 `json:"lon"`
	Lat        float64 `json:"lat"`
	RegionFile string  `json:"region_file,omitempty"`
	ComuneName string  `json:"comune_name,omitempty"`
	CODISTAT   string  `json:"codistat,omitempty"`
}

// ErrNotFound is returned when no matching parcel exists in the dataset.
var ErrNotFound = errors.New("parcel not found")

// ErrComuneNotIndexed is returned when the comune codice belfiore is not in index.parquet.
// Most commonly this means Trentino-Alto-Adige (TAA runs autonomous cadastres).
var ErrComuneNotIndexed = errors.New("comune not in ondata index (Trentino-Alto-Adige is not covered)")

// Client is a small fetch+query helper.
type Client struct {
	HTTP     *http.Client
	CacheDir string // local dir to store downloaded Parquet files
}

// NewClient returns a client that caches Parquet files under cacheDir.
// If cacheDir is empty, a per-user default is used.
func NewClient(cacheDir string) *Client {
	if cacheDir == "" {
		base, err := os.UserCacheDir()
		if err != nil || base == "" {
			base = os.TempDir()
		}
		cacheDir = filepath.Join(base, "catasto-pp-cli", "ondata")
	}
	return &Client{
		HTTP:     &http.Client{Timeout: defaultTimeout},
		CacheDir: cacheDir,
	}
}

// LookupIndex returns the index entry for the given comune codice belfiore.
// Codes are case-insensitive; the function uppercases internally.
func (c *Client) LookupIndex(ctx context.Context, comune string) (*IndexEntry, error) {
	comune = strings.ToUpper(strings.TrimSpace(comune))
	path, err := c.fetch(ctx, IndexFileName)
	if err != nil {
		return nil, fmt.Errorf("fetch index.parquet: %w", err)
	}
	rows, err := readParquet[IndexEntry](path)
	if err != nil {
		return nil, fmt.Errorf("read index.parquet: %w", err)
	}
	for i := range rows {
		if strings.EqualFold(rows[i].Comune, comune) {
			return &rows[i], nil
		}
	}
	return nil, fmt.Errorf("%w: codice belfiore %q", ErrComuneNotIndexed, comune)
}

// LookupParcel resolves a cadastral reference to its WGS84 centroid.
// foglio and particella are matched as strings to preserve zero-padding
// quirks in the source data. The index lookup is done first to find
// the regional file.
func (c *Client) LookupParcel(ctx context.Context, comuneCode, foglio, particella string) (*ParcelResult, error) {
	idx, err := c.LookupIndex(ctx, comuneCode)
	if err != nil {
		return nil, err
	}
	regPath, err := c.fetch(ctx, idx.File)
	if err != nil {
		return nil, fmt.Errorf("fetch %s: %w", idx.File, err)
	}
	// Normalize foglio/particella: try exact and zero-padded forms.
	fogliosToTry := normalizeNumericForms(foglio, 4) // ondata foglio is "0002"
	particellasToTry := normalizeNumericForms(particella, 0)

	rows, err := readParquet[ParcelRow](regPath)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", idx.File, err)
	}
	// Track diagnostics so a not-found error can distinguish between
	// "wrong comune", "right comune but wrong foglio", and "right
	// comune+foglio but wrong particella". In the last case, surface a
	// few neighboring particella values so the user can tell whether
	// the input is wrong or the data is incomplete.
	var comuneRows, fogliosMatchedRows int
	matchedFoglios := map[string]struct{}{}
	var nearbyParticelle []string
	for i := range rows {
		r := &rows[i]
		if !strings.EqualFold(r.Comune, comuneCode) {
			continue
		}
		comuneRows++
		if !anyEqual(r.Foglio, fogliosToTry) {
			matchedFoglios[r.Foglio] = struct{}{}
			continue
		}
		fogliosMatchedRows++
		if !anyEqual(r.Particella, particellasToTry) {
			nearbyParticelle = append(nearbyParticelle, r.Particella)
			continue
		}
		return &ParcelResult{
			InspireID:  r.InspireID,
			Comune:     r.Comune,
			Foglio:     r.Foglio,
			Particella: r.Particella,
			Lon:        r.Lon(),
			Lat:        r.Lat(),
			RegionFile: idx.File,
			ComuneName: idx.DenominazioneIT,
			CODISTAT:   idx.CODISTAT,
		}, nil
	}
	upperC := strings.ToUpper(comuneCode)
	switch {
	case comuneRows == 0:
		return nil, fmt.Errorf("%w: comune %s has 0 rows in %s (check the codice belfiore)",
			ErrNotFound, upperC, idx.File)
	case fogliosMatchedRows == 0:
		nearest := nearestFoglio(foglio, matchedFoglios)
		hint := ""
		if nearest != "" {
			hint = fmt.Sprintf("; nearest existing foglio is %s", nearest)
		}
		return nil, fmt.Errorf("%w: comune=%s has %d distinct foglios in %s but none match foglio=%s%s",
			ErrNotFound, upperC, len(matchedFoglios), idx.File, foglio, hint)
	default:
		near := closestParticelle(particella, nearbyParticelle, 5)
		return nil, fmt.Errorf("%w: %s", ErrNotFound, missingGeometryMessage(upperC, foglio, particella, fogliosMatchedRows, idx.File, near))
	}
}

func missingGeometryMessage(comune, foglio, particella string, parcelCount int, regionFile string, nearest []string) string {
	hint := ""
	if len(nearest) > 0 {
		hint = fmt.Sprintf(" nearest represented particelle: %s.", strings.Join(nearest, ", "))
	}
	// PATCH: Distinguish cadastral existence from geometric representation gaps in ondata/AdE map data.
	return fmt.Sprintf("comune=%s foglio=%s is represented with %d mapped parcels in %s, but particella=%s is not represented in this geometry dataset. The cadastral unit may still exist in Catasto Fabbricati or in official records; this lookup can only return GPS coordinates for parcels present in the mapped ondata/AdE geometry.%s",
		comune, foglio, parcelCount, regionFile, particella, hint)
}

// nearestFoglio returns the foglio key lexicographically closest to target
// from the available set, or "" if the set is empty.
func nearestFoglio(target string, available map[string]struct{}) string {
	if len(available) == 0 {
		return ""
	}
	keys := make([]string, 0, len(available))
	for k := range available {
		keys = append(keys, k)
	}
	sortStringsAsc(keys)
	for _, k := range keys {
		if k >= target {
			return k
		}
	}
	return keys[len(keys)-1]
}

// closestParticelle returns up to maxN particella values lexicographically
// adjacent to target from the provided slice.
func closestParticelle(target string, vals []string, maxN int) []string {
	if len(vals) == 0 || maxN <= 0 {
		return nil
	}
	sortStringsAsc(vals)
	insert := len(vals)
	for i, v := range vals {
		if v >= target {
			insert = i
			break
		}
	}
	lo := insert - maxN/2
	if lo < 0 {
		lo = 0
	}
	hi := lo + maxN
	if hi > len(vals) {
		hi = len(vals)
		lo = hi - maxN
		if lo < 0 {
			lo = 0
		}
	}
	return vals[lo:hi]
}

func sortStringsAsc(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j-1] > s[j]; j-- {
			s[j-1], s[j] = s[j], s[j-1]
		}
	}
}

// fetch downloads a Parquet file if not already cached locally.
// Caches under CacheDir/<fileName>.
func (c *Client) fetch(ctx context.Context, fileName string) (string, error) {
	if err := os.MkdirAll(c.CacheDir, 0o755); err != nil {
		return "", err
	}
	local := filepath.Join(c.CacheDir, fileName)
	if fi, err := os.Stat(local); err == nil && fi.Size() > 0 {
		return local, nil
	}
	url := BaseURL + "/" + fileName
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "catasto-pp-cli (+https://github.com/mvanhorn/cli-printing-press)")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GET %s: HTTP %d", url, resp.StatusCode)
	}
	tmp, err := os.CreateTemp(c.CacheDir, fileName+".part-*")
	if err != nil {
		return "", err
	}
	if _, err := io.Copy(tmp, resp.Body); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return "", err
	}
	if err := tmp.Close(); err != nil {
		return "", err
	}
	if err := os.Rename(tmp.Name(), local); err != nil {
		os.Remove(tmp.Name())
		return "", err
	}
	return local, nil
}

// readParquet reads an entire Parquet file into a slice of T.
// For the ondata files (index ~50KB, regional <50MB), this is fast enough.
// Switch to a row-iterator if memory becomes a concern.
func readParquet[T any](path string) ([]T, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		return nil, err
	}
	pf, err := parquet.OpenFile(f, fi.Size())
	if err != nil {
		return nil, err
	}
	reader := parquet.NewGenericReader[T](pf)
	defer reader.Close()
	out := make([]T, reader.NumRows())
	n, err := reader.Read(out)
	if err != nil && err != io.EOF {
		return nil, err
	}
	return out[:n], nil
}

func normalizeNumericForms(s string, pad int) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return []string{s}
	}
	forms := []string{s}
	// Try as integer for stripped leading zeros + padded.
	if n, err := strconv.Atoi(s); err == nil {
		forms = appendUnique(forms, strconv.Itoa(n))
		if pad > 0 {
			forms = appendUnique(forms, fmt.Sprintf("%0*d", pad, n))
		}
	}
	return forms
}

func anyEqual(s string, opts []string) bool {
	for _, o := range opts {
		if strings.EqualFold(s, o) {
			return true
		}
	}
	return false
}

func appendUnique(xs []string, s string) []string {
	for _, x := range xs {
		if x == s {
			return xs
		}
	}
	return append(xs, s)
}
