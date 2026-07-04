// Hand-authored `show` command: full AI rating breakdown for one hotel, parsed
// from the hotelist.com /modal/{id} HTML fragment. Not generated.
package cli

import (
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/travel/hotelist/internal/cliutil"
)

type ratingRow struct {
	Label   string  `json:"label"`
	Value   float64 `json:"value"`
	Updated string  `json:"updated,omitempty"`
}

type hotelDetail struct {
	Source     string      `json:"source"`
	Disclaimer string      `json:"disclaimer"`
	HotelID    string      `json:"hotel_id"`
	Name       string      `json:"name"`
	URL        string      `json:"url"`
	Ratings    []ratingRow `json:"ratings"`
	Amenities  []string    `json:"verified_amenities"`
	Pros       []string    `json:"pros,omitempty"`
	Cons       []string    `json:"cons,omitempty"`
	Note       string      `json:"note,omitempty"`
}

var (
	reModalH1       = regexp.MustCompile(`(?s)<h1[^>]*>(.*?)</h1>`)
	reRatingRow     = regexp.MustCompile(`(?s)<td class="key">(.*?)</td>\s*<td class="value">.*?class="filling"[^>]*>\s*([0-9]+(?:\.[0-9]+)?)\s*</div>(.*?)</td>`)
	reLastUpdated   = regexp.MustCompile(`(?s)class="last-updated">(.*?)</div>`)
	reAmenityName   = regexp.MustCompile(`(?s)class="amenity-name"[^>]*>(.*?)</`)
	reTagStrip      = regexp.MustCompile(`<[^>]+>`)
	reProsConsBlock = regexp.MustCompile(`(?s)class="tab tab-pros-cons".*?>(.*?)(?:<div class="tab |</div>\s*</div>\s*$)`)
	reLi            = regexp.MustCompile(`(?s)<li[^>]*>(.*?)</li>`)
)

func stripTags(s string) string {
	return cliutil.CleanText(reTagStrip.ReplaceAllString(s, " "))
}

func parseHotelModal(id, html string) hotelDetail {
	d := hotelDetail{
		Source:     hotelistSource,
		Disclaimer: hotelistDisclaimer,
		HotelID:    id,
		URL:        "https://hotelist.com/hotel/" + id,
	}
	if m := reModalH1.FindStringSubmatch(html); m != nil {
		d.Name = stripTags(m[1])
	}
	for _, m := range reRatingRow.FindAllStringSubmatch(html, -1) {
		label := stripTags(m[1])
		if label == "" {
			label = "source"
		}
		var val float64
		// Best-effort numeric parse; a non-numeric rating cell legitimately
		// leaves val at 0.0, so the scan error is intentionally ignored.
		if _, err := fmt.Sscanf(strings.TrimSpace(m[2]), "%g", &val); err != nil {
			val = 0
		}
		row := ratingRow{Label: label, Value: val}
		if u := reLastUpdated.FindStringSubmatch(m[3]); u != nil {
			row.Updated = stripTags(u[1])
		}
		d.Ratings = append(d.Ratings, row)
	}
	seen := map[string]bool{}
	for _, m := range reAmenityName.FindAllStringSubmatch(html, -1) {
		a := stripTags(m[1])
		if a == "" || seen[strings.ToLower(a)] {
			continue
		}
		seen[strings.ToLower(a)] = true
		d.Amenities = append(d.Amenities, a)
	}
	// Pros/cons from the pros-cons tab list items (best-effort).
	if pc := reProsConsBlock.FindStringSubmatch(html); pc != nil {
		for _, li := range reLi.FindAllStringSubmatch(pc[1], -1) {
			text := stripTags(li[1])
			if text == "" {
				continue
			}
			if strings.HasPrefix(text, "−") || strings.HasPrefix(text, "✗") || strings.HasPrefix(text, "👎") {
				d.Cons = append(d.Cons, text)
			} else {
				d.Pros = append(d.Pros, text)
			}
		}
	}
	if len(d.Ratings) == 0 && d.Name == "" {
		d.Note = "could not parse the detail modal; the site layout may have changed"
	}
	return d
}

func newShowCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show <hotel-id>",
		Short: "Show one hotel's full AI rating breakdown, verified amenities, and pros/cons",
		Long: "Fetch and parse a hotel's detail from hotelist.com: the overall Hotelist Score, the AI " +
			"rating of photos and of reviews, per-source normalized ratings, photo-verified amenities, " +
			"and pros/cons. Pass a hotel id (e.g. KYLCGAVE) as returned by 'search'. Data is scraped " +
			"from hotelist.com (community/AI-rated, not an official API).",
		Example: trimExample(`
  hotelist-pp-cli show KYLCGAVE
  hotelist-pp-cli show KYLCGAVE --json`),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a hotel id is required (get one from 'hotelist-pp-cli search <city>')"))
			}
			id := strings.TrimSpace(args[0])

			c, err := flags.politeClient()
			if err != nil {
				return err
			}
			data, err := c.Get(cmd.Context(), "/modal/"+id, nil)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			d := parseHotelModal(id, string(data))

			out := cmd.OutOrStdout()
			if !wantsHumanTable(out, flags) {
				return printJSONFiltered(out, d, flags)
			}
			printHotelDetailHuman(out, d)
			return nil
		},
	}
	return cmd
}

func printHotelDetailHuman(out io.Writer, d hotelDetail) {
	name := d.Name
	if name == "" {
		name = d.HotelID
	}
	fmt.Fprintf(out, "%s\n", name)
	fmt.Fprintln(out, strings.Repeat("-", 60))
	for _, r := range d.Ratings {
		upd := ""
		if r.Updated != "" {
			upd = "  (" + r.Updated + ")"
		}
		fmt.Fprintf(out, "  %-26s %.1f%s\n", r.Label, r.Value, upd)
	}
	if len(d.Amenities) > 0 {
		fmt.Fprintf(out, "\nVerified amenities: %s\n", strings.Join(d.Amenities, ", "))
	}
	if len(d.Pros) > 0 {
		fmt.Fprintln(out, "\nPros:")
		for _, p := range d.Pros {
			fmt.Fprintf(out, "  + %s\n", p)
		}
	}
	if len(d.Cons) > 0 {
		fmt.Fprintln(out, "\nCons:")
		for _, c := range d.Cons {
			fmt.Fprintf(out, "  - %s\n", c)
		}
	}
	if d.Note != "" {
		fmt.Fprintf(out, "\nnote: %s\n", d.Note)
	}
	fmt.Fprintf(out, "\n%s\n%s\n", d.URL, d.Disclaimer)
}
