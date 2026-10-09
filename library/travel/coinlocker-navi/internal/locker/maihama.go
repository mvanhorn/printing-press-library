// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.

package locker

import (
	"bytes"
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"golang.org/x/net/html"
)

// Maihama Station (Tokyo Disney Resort) live vacancy map on coinlocker-navi.
const (
	MaihamaPath = "/areamap/maihamaeki/"
	MaihamaLat  = 35.6366
	MaihamaLon  = 139.8840
)

// BlockSize is one size column of a Maihama block.
type BlockSize struct {
	Label     *string `json:"label"`
	LabelJA   string  `json:"label_ja"`
	Installed *int    `json:"installed"`
	Empty     *int    `json:"empty"`
}

// MaihamaBlock is one locker block on the Maihama map.
type MaihamaBlock struct {
	GID       string      `json:"gid"`
	Name      string      `json:"name"`
	Status    *string     `json:"status"`
	Sizes     []BlockSize `json:"sizes"`
	LockerID  *string     `json:"locker_id"`
	LockerURL *string     `json:"locker_url"`
}

// MaihamaBoard is the full live board.
type MaihamaBoard struct {
	Station    BiName         `json:"station"`
	SourceAsOf *string        `json:"source_as_of"`
	SourceURL  string         `json:"source_url"`
	Blocks     []MaihamaBlock `json:"blocks"`
	Legend     string         `json:"legend"`
}

var (
	gidRe      = regexp.MustCompile(`open_dialog\('(\d+)'\)"><img src="icon/([a-z]+)_`)
	asOfRe     = regexp.MustCompile(`(\d{4})/(\d{1,2})/(\d{1,2})\s+(\d{1,2}):(\d{2})現在`)
	installRe  = regexp.MustCompile(`設置数\s*([0-9,]+)\s*個`)
	emptyRe    = regexp.MustCompile(`空き数\s*([0-9,]+)\s*個`)
	statusByIc = map[string]string{"circle": "6_or_more_empty", "triangle": "5_or_fewer_empty", "cross": "none_empty"}
)

// ParseMaihamaIndex returns the gid list with map status and the as-of time
// (RFC3339, JST) printed on the page.
func ParseMaihamaIndex(page []byte) ([]MaihamaBlock, *string) {
	blocks := []MaihamaBlock{}
	seen := map[string]bool{}
	for _, m := range gidRe.FindAllSubmatch(page, -1) {
		gid := string(m[1])
		if seen[gid] {
			continue
		}
		seen[gid] = true
		b := MaihamaBlock{GID: gid, Sizes: []BlockSize{}}
		if s, ok := statusByIc[string(m[2])]; ok {
			b.Status = strPtr(s)
		}
		blocks = append(blocks, b)
	}
	var asOf *string
	if m := asOfRe.FindSubmatch(page); m != nil {
		t, err := time.ParseInLocation("2006/1/2 15:04", fmt.Sprintf("%s/%s/%s %s:%s", m[1], m[2], m[3], m[4], m[5]), JST)
		if err == nil {
			s := t.Format(time.RFC3339)
			asOf = &s
		}
	}
	return blocks, asOf
}

// ParseMaihamaBlock fills a block from its ajax fragment.
func ParseMaihamaBlock(b *MaihamaBlock, frag []byte, baseURL string) error {
	doc, err := html.Parse(bytes.NewReader(frag))
	if err != nil {
		return err
	}
	if h3 := findFirst(doc, byTag("h3")); h3 != nil {
		b.Name = clean(text(h3))
	}
	var labels []string
	for _, th := range findAll(doc, byTag("th")) {
		labels = append(labels, clean(text(th)))
	}
	var cells []string
	for _, td := range findAll(doc, byTag("td")) {
		t := clean(text(td))
		if strings.Contains(t, "設置数") || strings.Contains(t, "空き数") {
			cells = append(cells, t)
		}
	}
	for i, lab := range labels {
		bs := BlockSize{LabelJA: lab}
		if l, ok := sizeLabels[lab]; ok {
			bs.Label = strPtr(l)
		}
		if i < len(cells) {
			if m := installRe.FindStringSubmatch(cells[i]); m != nil {
				bs.Installed = atoiPtr(m[1])
			}
			if m := emptyRe.FindStringSubmatch(cells[i]); m != nil {
				bs.Empty = atoiPtr(m[1])
			}
		}
		b.Sizes = append(b.Sizes, bs)
	}
	if a := findFirst(doc, func(n *html.Node) bool {
		return n.Type == html.ElementNode && n.Data == "a" && clIDRe.MatchString(attr(n, "href"))
	}); a != nil {
		id := clIDRe.FindStringSubmatch(attr(a, "href"))[1]
		b.LockerID = strPtr(id)
		b.LockerURL = strPtr(strings.TrimRight(baseURL, "/") + "/cl/" + id)
	}
	if b.Name == "" && len(b.Sizes) == 0 {
		return fmt.Errorf("maihama block %s: empty fragment (page shape changed?)", b.GID)
	}
	return nil
}

// Maihama fetches the board and every block.
func (c *Client) Maihama(ctx context.Context) (MaihamaBoard, error) {
	base := c.NaviBase + MaihamaPath
	board := MaihamaBoard{
		Station:   BiName{JA: strPtr("舞浜駅"), EN: strPtr("Maihama Station")},
		SourceURL: base,
		Blocks:    []MaihamaBlock{},
		Legend:    "status comes from the map icon: 6_or_more_empty, 5_or_fewer_empty, none_empty; empty counts are physical empty boxes as published by the source",
	}
	page, err := c.fetch(ctx, base, nil)
	if err != nil {
		return board, err
	}
	blocks, asOf := ParseMaihamaIndex(page)
	board.SourceAsOf = asOf
	if len(blocks) == 0 {
		return board, fmt.Errorf("maihama map: no locker blocks found (service may have stopped)")
	}
	for i := range blocks {
		frag, err := c.fetch(ctx, base+"ajax?gid="+blocks[i].GID, map[string]string{"X-Requested-With": "XMLHttpRequest", "Referer": base})
		if err != nil {
			return board, err
		}
		if err := ParseMaihamaBlock(&blocks[i], frag, c.NaviBase); err != nil {
			return board, err
		}
	}
	board.Blocks = blocks
	return board, nil
}
