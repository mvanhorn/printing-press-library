// internal/render/render.go
package render

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/mvanhorn/printing-press-library/library/other/running-race-results/internal/domain"
)

// Table writes a human-readable two-column view; empty fields are skipped.
func Table(w io.Writer, r domain.Result) error {
	tw := tabwriter.NewWriter(w, 0, 2, 2, ' ', 0)
	row := func(label, val string) {
		if val != "" {
			fmt.Fprintf(tw, "%s\t%s\n", label, val)
		}
	}
	row("Provider", r.Provider)
	if r.Year > 0 {
		row("Race", fmt.Sprintf("%s %d", r.RaceName, r.Year))
	} else {
		row("Race", r.RaceName)
	}
	row("Runner", r.Runner)
	row("Bib", r.Bib)
	row("Net time", r.NetTime)
	row("Gun time", r.GunTime)
	if r.OverallPlace > 0 {
		row("Overall place", fmt.Sprintf("%d", r.OverallPlace))
	}
	if r.GenderPlace > 0 {
		row("Gender place", fmt.Sprintf("%d", r.GenderPlace))
	}
	row("Age group", r.AgeGroup)
	if r.AgeGroupPlace > 0 {
		row("Age group place", fmt.Sprintf("%d", r.AgeGroupPlace))
	}
	row("Source", r.SourceURL)
	return tw.Flush()
}

// JSON writes the result as indented JSON.
func JSON(w io.Writer, r domain.Result) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(r)
}

// Column is one column of a List rendering.
type Column struct {
	Header string
	Value  func(domain.Result) string
}

// List writes rows as a header + aligned columns.
func List(w io.Writer, rows []domain.Result, cols []Column) error {
	tw := tabwriter.NewWriter(w, 0, 2, 2, ' ', 0)
	headers := make([]string, len(cols))
	for i, c := range cols {
		headers[i] = c.Header
	}
	fmt.Fprintln(tw, strings.Join(headers, "\t"))
	for _, r := range rows {
		cells := make([]string, len(cols))
		for i, c := range cols {
			cells[i] = c.Value(r)
		}
		fmt.Fprintln(tw, strings.Join(cells, "\t"))
	}
	return tw.Flush()
}

// Athletes writes an athlete disambiguation table.
func Athletes(w io.Writer, as []domain.Athlete) error {
	tw := tabwriter.NewWriter(w, 0, 2, 2, ' ', 0)
	fmt.Fprintln(tw, "Name\tCity\tAge\tRacerID")
	for _, a := range as {
		loc := a.City
		if a.StateProv != "" {
			loc = a.City + ", " + a.StateProv
		}
		fmt.Fprintf(tw, "%s\t%s\t%d\t%s\n", a.Name, loc, a.Age, a.ID)
	}
	return tw.Flush()
}

// JSONValue writes any value as indented JSON.
func JSONValue(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}
