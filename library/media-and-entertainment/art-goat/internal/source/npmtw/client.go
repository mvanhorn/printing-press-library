// Copyright 2026 justinwfu. Licensed under Apache-2.0.

// Package npmtw implements the art-goat Source for the National Palace
// Museum, Taipei (NPM Taiwan) — the imperial Chinese collection moved
// from Beijing's Forbidden City to Taipei in 1949. ~700,000 objects;
// bronzes, ceramics, jades, paintings, calligraphy.
//
// # Strategy: static-curated
//
// NPM Taiwan does not expose a query-able REST API. The museum's open
// data portal (https://theme.npm.edu.tw/opendata/, redirects to
// https://digitalarchive.npm.gov.tw/opendata) is a web search UI over
// ~108k records with image-license metadata (CC0 for ~1MP, CC BY 4.0
// for ~6MP) but no documented JSON download URLs or programmatic
// endpoint as of 2026-05. Scraping the live HTML is out of scope per
// the art-goat source contract; the upstream HTML is not a documented
// machine surface, ToS aside.
//
// In place of a live sync, this source ships a hand-curated list of
// ~17 of the museum's best-known works. Each entry is researched and
// the image URL is a Wikimedia Commons file verified to exist at write
// time (2026-05-21), drawn from the english-language Wikipedia
// articles or the Commons category
// https://commons.wikimedia.org/wiki/Category:Collections_of_the_National_Palace_Museum.
// All curated entries are pre-1923 works in the public domain; the
// individual photographs are licensed CC-BY-SA or public-domain on
// Commons. Source URLs point at the english-language Wikipedia article
// for the work (stable, citable), with a Commons-fallback when no
// dedicated article exists.
//
// # Caveats (surface in the user-facing README)
//
//   - Static-curated coverage. ~17 works vs. ~700k in the real
//     collection; this source is a discovery surface, not a true
//     mirror. Calling Sync with --full does not pull more.
//   - Image URLs are Wikimedia Commons thumbnails, not the NPM's own
//     6MP CC BY 4.0 high-resolution masters. Resolution and crop may
//     differ from what the museum publishes.
//   - SourceURL points at the english-language Wikipedia article, not
//     the NPM's own object page (no stable english-language object-URL
//     pattern is documented). The museum's own page may differ in
//     accession-number form or be Chinese-only.
//   - If the open-data portal ever exposes a documented JSON endpoint,
//     this source should be reworked into a live-fetch shape and the
//     static list dropped; that is a v1.2+ task.
package npmtw

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/art-goat/internal/source"
)

func init() {
	source.Register(&Client{})
}

type Client struct{}

func (c *Client) Name() string {
	return "npmtw"
}

func (c *Client) Description() string {
	return "National Palace Museum Taiwan — Chinese imperial collection"
}

func (c *Client) AuthRequired() bool {
	return false
}

// curatedEntry is the in-package shape for a hand-authored work. Kept
// distinct from source.Work so the curated list reads compactly; the
// mapper below converts to source.Work with the right composite ID,
// canonical creator form, and RawJSON snapshot.
type curatedEntry struct {
	Slug           string `json:"slug"`
	Title          string `json:"title"`
	TitleZh        string `json:"title_zh,omitempty"`
	Creator        string `json:"creator"`
	DateText       string `json:"date_text"`
	DateStart      int    `json:"date_start"`
	DateEnd        int    `json:"date_end"`
	Medium         string `json:"medium"`
	Period         string `json:"period,omitempty"`
	Description    string `json:"description"`
	ImageURL       string `json:"image_url"`
	SourceURL      string `json:"source_url"`
	Classification string `json:"classification,omitempty"`
}

// Sync returns the curated NPM Taiwan list, capped by opts.Limit. The
// curated list is the source's "natural curated default" — opts.Full
// has no additional effect because there is no live archive to pull.
func (c *Client) Sync(ctx context.Context, opts source.SyncOpts) ([]source.Work, error) {
	now := time.Now().UTC()
	out := make([]source.Work, 0, len(curated))
	for _, e := range curated {
		select {
		case <-ctx.Done():
			return out, ctx.Err()
		default:
		}
		out = append(out, entryToWork(e, now))
		if opts.Limit > 0 && len(out) >= opts.Limit {
			break
		}
	}
	return out, nil
}

func entryToWork(e curatedEntry, syncedAt time.Time) source.Work {
	title := e.Title
	if e.TitleZh != "" {
		title = e.Title + " (" + e.TitleZh + ")"
	}
	w := source.Work{
		ID:               "npmtw:" + e.Slug,
		Source:           "npmtw",
		SourceID:         e.Slug,
		Title:            title,
		Creator:          e.Creator,
		CreatorCanonical: strings.ToLower(strings.TrimSpace(e.Creator)),
		DateText:         e.DateText,
		DateStart:        e.DateStart,
		DateEnd:          e.DateEnd,
		Medium:           e.Medium,
		Classification:   e.Classification,
		Period:           e.Period,
		CultureRegion:    "Taiwan / Imperial China",
		Description:      e.Description,
		ImageURL:         e.ImageURL,
		ThumbnailURL:     e.ImageURL,
		License:          "Public domain",
		SourceURL:        e.SourceURL,
		SyncedAt:         syncedAt,
	}
	raw, _ := json.Marshal(e)
	w.RawJSON = string(raw)
	return w
}

// curated is the hand-authored list of well-known NPM Taipei works.
// Image URLs were verified against Wikimedia Commons / english-language
// Wikipedia article infoboxes at 2026-05-21. Entries are public-domain
// originals; the photographs themselves on Commons are CC-BY-SA or PD.
// Add entries only when (a) the work is in the NPM Taipei collection,
// (b) the original is pre-1923, and (c) a stable Commons image URL is
// verifiable.
var curated = []curatedEntry{
	{
		Slug:           "jadeite-cabbage",
		Title:          "Jadeite Cabbage",
		TitleZh:        "翠玉白菜",
		Creator:        "Unknown",
		DateText:       "Qing dynasty, 19th century",
		DateStart:      1800,
		DateEnd:        1900,
		Medium:         "Jadeite carving",
		Period:         "Qing dynasty",
		Classification: "Jade",
		Description:    "A small jadeite carving of a bok choy with a katydid and locust nestled in its leaves. One of the museum's most beloved objects, originally part of a dowry for Consort Jin.",
		ImageURL:       "https://upload.wikimedia.org/wikipedia/commons/thumb/d/de/Jadeite_Cabbage%2C_National_Palace_Museum.jpg/330px-Jadeite_Cabbage%2C_National_Palace_Museum.jpg",
		SourceURL:      "https://en.wikipedia.org/wiki/Jadeite_Cabbage",
	},
	{
		Slug:           "meat-shaped-stone",
		Title:          "Meat-shaped Stone",
		TitleZh:        "肉形石",
		Creator:        "Unknown",
		DateText:       "Qing dynasty, 17th–18th century",
		DateStart:      1644,
		DateEnd:        1800,
		Medium:         "Banded jasper",
		Period:         "Qing dynasty",
		Classification: "Stone carving",
		Description:    "A piece of jasper carved and dyed to resemble a slab of braised Dongpo pork. Often paired in popular imagination with the Jadeite Cabbage as the museum's twin curiosities.",
		ImageURL:       "https://upload.wikimedia.org/wikipedia/commons/thumb/b/bd/Meat-Shaped_Stone_Gathering_of_Treasures_1.jpg/330px-Meat-Shaped_Stone_Gathering_of_Treasures_1.jpg",
		SourceURL:      "https://en.wikipedia.org/wiki/Meat-Shaped_Stone",
	},
	{
		Slug:           "mao-gong-ding",
		Title:          "Mao Gong Ding",
		TitleZh:        "毛公鼎",
		Creator:        "Unknown",
		DateText:       "Late Western Zhou, c. 9th–8th century BCE",
		DateStart:      -900,
		DateEnd:        -800,
		Medium:         "Bronze ritual vessel",
		Period:         "Western Zhou",
		Classification: "Bronze",
		Description:    "A bronze ding cauldron with the longest known Western Zhou inscription — 500 characters recording a charge from King Xuan to his uncle Duke Mao. A cornerstone of early Chinese epigraphy.",
		ImageURL:       "https://upload.wikimedia.org/wikipedia/commons/thumb/e/e2/Ding_cauldron_of_Duke_Mao.jpg/250px-Ding_cauldron_of_Duke_Mao.jpg",
		SourceURL:      "https://en.wikipedia.org/wiki/Mao_Gong_ding",
	},
	{
		Slug:           "travelers-among-mountains-and-streams",
		Title:          "Travelers Among Mountains and Streams",
		TitleZh:        "谿山行旅圖",
		Creator:        "Fan Kuan",
		DateText:       "Early 11th century",
		DateStart:      1000,
		DateEnd:        1050,
		Medium:         "Hanging scroll, ink on silk",
		Period:         "Northern Song",
		Classification: "Painting",
		Description:    "Fan Kuan's monumental landscape — a sheer cliff dominates the upper register, with travelers and a mule train rendered tiny along a stream below. A founding image of the monumental-landscape tradition.",
		ImageURL:       "https://upload.wikimedia.org/wikipedia/commons/thumb/c/c2/Fan_Kuan_-_Travelers_Among_Mountains_and_Streams_-_Google_Art_Project.jpg/250px-Fan_Kuan_-_Travelers_Among_Mountains_and_Streams_-_Google_Art_Project.jpg",
		SourceURL:      "https://en.wikipedia.org/wiki/Fan_Kuan",
	},
	{
		Slug:           "early-spring",
		Title:          "Early Spring",
		TitleZh:        "早春圖",
		Creator:        "Guo Xi",
		DateText:       "1072",
		DateStart:      1072,
		DateEnd:        1072,
		Medium:         "Hanging scroll, ink and light color on silk",
		Period:         "Northern Song",
		Classification: "Painting",
		Description:    "A landscape of writhing mountains and mist, painted at court for Emperor Shenzong. Guo Xi's signature evocation of mountains as living, breathing forms in the moment the year turns.",
		ImageURL:       "https://upload.wikimedia.org/wikipedia/commons/thumb/8/86/Guo_Xi_-_Early_Spring_%28large%29.jpg/250px-Guo_Xi_-_Early_Spring_%28large%29.jpg",
		SourceURL:      "https://en.wikipedia.org/wiki/Early_Spring_(painting)",
	},
	{
		Slug:           "autumn-in-the-river-valley",
		Title:          "Autumn in the River Valley",
		Creator:        "Guo Xi",
		DateText:       "11th century",
		DateStart:      1050,
		DateEnd:        1090,
		Medium:         "Hanging scroll, ink on silk",
		Period:         "Northern Song",
		Classification: "Painting",
		Description:    "An autumn landscape attributed to Guo Xi; quieter and smaller in scale than Early Spring but built from the same vocabulary of furled cloud-like rock.",
		ImageURL:       "https://upload.wikimedia.org/wikipedia/commons/thumb/3/37/Kuo_Hsi_001.jpg/250px-Kuo_Hsi_001.jpg",
		SourceURL:      "https://en.wikipedia.org/wiki/Guo_Xi",
	},
	{
		Slug:           "autumn-colors-on-the-que-and-hua-mountains",
		Title:          "Autumn Colors on the Que and Hua Mountains",
		TitleZh:        "鵲華秋色圖",
		Creator:        "Zhao Mengfu",
		DateText:       "1296",
		DateStart:      1296,
		DateEnd:        1296,
		Medium:         "Handscroll, ink and color on paper",
		Period:         "Yuan dynasty",
		Classification: "Painting",
		Description:    "Painted for a friend who had never seen his ancestral landscape; Zhao Mengfu evokes the two Shandong mountains from memory and from the older Tang-dynasty manner he was reviving.",
		ImageURL:       "https://upload.wikimedia.org/wikipedia/commons/thumb/9/9a/2a_Zhao_Mengfu_Autumn_Colors_on_the_Qiao_and_Hua_Mountains_%28central_part%29Handscroll%2C_ink_and_colors_on_paper%2C_28.4_x_93.2_cm_National_Palace_Museum%2C_Taipei.jpg/1280px-2a_Zhao_Mengfu_Autumn_Colors_on_the_Qiao_and_Hua_Mountains_%28central_part%29Handscroll%2C_ink_and_colors_on_paper%2C_28.4_x_93.2_cm_National_Palace_Museum%2C_Taipei.jpg",
		SourceURL:      "https://en.wikipedia.org/wiki/Zhao_Mengfu",
	},
	{
		Slug:           "sheep-and-goat",
		Title:          "Sheep and Goat",
		Creator:        "Zhao Mengfu",
		DateText:       "Late 13th century",
		DateStart:      1280,
		DateEnd:        1300,
		Medium:         "Handscroll, ink on paper",
		Period:         "Yuan dynasty",
		Classification: "Painting",
		Description:    "A pair of animals — one sheep, one goat — painted from observation in the calligraphic ink-brushwork that defines Zhao Mengfu's reform of painting through the discipline of writing.",
		ImageURL:       "https://upload.wikimedia.org/wikipedia/commons/thumb/6/67/Zhao_Mengfu%2C_Sheep_and_Goat.jpg/960px-Zhao_Mengfu%2C_Sheep_and_Goat.jpg",
		SourceURL:      "https://en.wikipedia.org/wiki/Zhao_Mengfu",
	},
	{
		Slug:           "along-the-river-qingming-qing-court",
		Title:          "Along the River During the Qingming Festival (Qing Court Version)",
		TitleZh:        "清明上河圖（清院本）",
		Creator:        "Chen Mei, Sun Hu, Jin Kun, Dai Hong, Cheng Zhidao",
		DateText:       "1736",
		DateStart:      1736,
		DateEnd:        1736,
		Medium:         "Handscroll, ink and color on silk",
		Period:         "Qing dynasty",
		Classification: "Painting",
		Description:    "A Qing-court reimagining of Zhang Zeduan's twelfth-century panorama of urban life, repopulated with eighteenth-century costume, architecture, and event. The Beijing Palace Museum holds the Song original; the NPM Taipei holds this longer Qianlong-era version.",
		ImageURL:       "https://upload.wikimedia.org/wikipedia/commons/thumb/2/2c/Along_the_River_During_the_Qingming_Festival_%28Qing_Court_Version%29.jpg/3840px-Along_the_River_During_the_Qingming_Festival_%28Qing_Court_Version%29.jpg",
		SourceURL:      "https://en.wikipedia.org/wiki/Along_the_River_During_the_Qingming_Festival",
	},
}
