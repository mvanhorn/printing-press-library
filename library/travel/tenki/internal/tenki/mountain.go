package tenki

import (
	"context"
	"errors"
	"strings"
	"time"

	"golang.org/x/net/html"
)

func (c *Client) Mountain(ctx context.Context, target string) (MountainResult, error) {
	raw, err := canonicalPlaceURL(target)
	if err != nil {
		if strings.Contains(target, "://") {
			return MountainResult{}, err
		}
		search, err := c.Search(ctx, target, "mountain", 10, 1)
		if err != nil {
			return MountainResult{}, err
		}
		if len(search.Places) == 0 {
			return MountainResult{}, errors.New("mountain not found in source directory")
		}
		if len(search.Places) != 1 || search.Truncated {
			return MountainResult{}, errors.New("mountain name is ambiguous; select a canonical URL")
		}
		raw = search.Places[0].URL
	}
	if !strings.HasPrefix(raw, "https://tenki.jp/mountain/") {
		return MountainResult{}, errors.New("target is not a mountain URL")
	}
	body, source, err := c.fetch(ctx, raw, forecastTTL)
	if err != nil {
		return MountainResult{}, err
	}
	return c.parseMountain(body, source, raw)
}

func (c *Client) parseMountain(body string, source Source, raw string) (MountainResult, error) {
	doc := parseHTML(body)
	place := bodyPlace(doc, raw, "mountain")
	result := MountainResult{Place: place, Source: source, Status: "ok", Warnings: []string{}, ModelKind: "nearby_model_guidance", Levels: []ModelLevel{}}
	anchor := c.cfg.Now().In(JST)
	if forecast, _ := forecastIssue(doc); !forecast.IsZero() {
		anchor = forecast
	}
	section := id(doc, "anchor-gpv")
	if section == nil {
		result.Status = "unavailable"
		result.ModelSource = source
		result.Warnings = append(result.Warnings, "No altitude model guidance available; summit forecast unavailable.")
		return result, nil
	}
	timeNode := find(section, func(n *html.Node) bool { return n.Data == "time" })
	initialRaw := nodeText(timeNode)
	initial := dayClock(initialRaw, anchor)
	if dt := attr(timeNode, "datetime"); dt != "" {
		if parsed, e := time.Parse(time.RFC3339, dt); e == nil {
			initial = parsed
			initialRaw = dt
		}
	}
	result.ModelInitialAt = stamp(initial)
	result.ModelInitialRaw = initialRaw
	source.FreshnessReference, source.FreshnessAt = "model_initial_at", stamp(initial)
	applyFreshness(&source, initial, 12*time.Hour, c.cfg.Now())
	result.Source, result.ModelSource = source, source
	if !initial.IsZero() {
		anchor = initial
	}
	result.Warnings = sourceWarnings(source)
	table := class(section, "gpv-table-box")
	if table == nil {
		return result, errors.New("mountain altitude model markup unavailable")
	}
	dateCells := all(table, func(n *html.Node) bool { return n.Data == "td" && hasClass(n, "date") })
	hourCells := all(table, func(n *html.Node) bool { return n.Data == "td" && hasClass(n, "time") })
	if initial.IsZero() {
		result.Warnings = append(result.Warnings, "Model initialization cannot be verified; valid dates use the page calendar context.")
	}
	if len(dateCells)*2 != len(hourCells) {
		return result, errors.New("mountain altitude model date/hour alignment changed")
	}
	dates := []time.Time{}
	last := anchor
	for _, cell := range dateCells {
		d := dateFromText(nodeText(cell), last)
		if d.IsZero() {
			day := atoi(strings.TrimSuffix(nodeText(cell), "日"))
			d = validDate(last.Year(), int(last.Month()), day)
			if !d.IsZero() && d.Day() < last.Day() {
				next := last.AddDate(0, 1, 0)
				d = validDate(next.Year(), int(next.Month()), day)
			}
		}
		if d.IsZero() {
			return result, errors.New("mountain model calendar date missing")
		}
		dates = append(dates, d)
		last = d
	}
	for _, row := range all(table, func(n *html.Node) bool { return n.Data == "tr" && hasClass(n, "gpv-table-entries") }) {
		alt := number(nodeText(class(row, "altitude")))
		if alt == nil {
			return result, errors.New("mountain model elevation missing")
		}
		cells := childElements(row, "td")
		if len(cells) != len(hourCells) {
			return result, errors.New("mountain model value count changed")
		}
		for i, cell := range cells {
			hour := atoi(strings.TrimSuffix(nodeText(hourCells[i]), "時"))
			if hour < 0 || hour > 24 {
				return result, errors.New("mountain model hour invalid")
			}
			result.Levels = append(result.Levels, ModelLevel{ElevationM: *alt, ValidAt: stamp(dates[i/2].Add(time.Duration(hour) * time.Hour)), TemperatureC: number(nodeText(class(cell, "temp"))), WindSpeedMS: number(nodeText(class(cell, "wind-speed"))), WindDirection: imageAlt(class(cell, "wind-icon"))})
		}
	}
	result.Warnings = append(result.Warnings, "Altitude values are nearby numerical calculation results, explicitly not weather forecasts. No summit interpolation is supplied; foothill municipality weather and mountain elevation are separate.")
	return result, nil
}
