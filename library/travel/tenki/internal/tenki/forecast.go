package tenki

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"golang.org/x/net/html"
)

func (c *Client) forecastTarget(ctx context.Context, target string) (Place, error) {
	if strings.HasPrefix(target, "https://") {
		u, err := canonicalPlaceURL(target)
		if err != nil {
			return Place{}, err
		}
		if municipalityPath.MatchString(strings.TrimPrefix(u, "https://tenki.jp")) {
			return Place{URL: u, Kind: "municipality", ForecastReferenceURL: u, Scope: "municipal", ID: strings.TrimPrefix(u, "https://tenki.jp/")}, nil
		}
	}
	r, err := c.Resolve(ctx, target)
	if err != nil {
		return Place{}, err
	}
	if r.Place.ForecastReferenceURL == "" {
		return Place{}, fmt.Errorf("no linked municipality forecast for %s", r.Place.Name)
	}
	return r.Place, nil
}

func (c *Client) Daily(ctx context.Context, target string) (ForecastResult, error) {
	place, err := c.forecastTarget(ctx, target)
	if err != nil {
		return ForecastResult{}, err
	}
	body, source, err := c.fetch(ctx, place.ForecastReferenceURL+"10days.html", forecastTTL)
	if err != nil {
		return ForecastResult{}, err
	}
	return c.parseDaily(body, source, place)
}

func (c *Client) Hourly(ctx context.Context, target string) (ForecastResult, error) {
	place, err := c.forecastTarget(ctx, target)
	if err != nil {
		return ForecastResult{}, err
	}
	body, source, err := c.fetch(ctx, place.ForecastReferenceURL+"1hour.html", forecastTTL)
	if err != nil {
		return ForecastResult{}, err
	}
	return c.parseHourly(body, source, place)
}

func forecastPlace(doc *html.Node, place Place) Place {
	municipal := bodyPlace(doc, place.ForecastReferenceURL, "municipality")
	if place.Kind == "municipality" {
		return municipal
	}
	if place.ForecastReferenceName == "" {
		place.ForecastReferenceName = municipal.Name
	}
	return place
}

func coverage(result *ForecastResult) {
	if len(result.Periods) == 0 {
		return
	}
	result.CoverageStart = result.Periods[0].Start
	result.CoverageEnd = result.Periods[len(result.Periods)-1].End
}

func (c *Client) parseHourly(body string, source Source, place Place) (ForecastResult, error) {
	doc := parseHTML(body)
	issue, raw := forecastIssue(doc)
	applyIssue(&source, issue, raw, 2*time.Hour, c.cfg.Now())
	result := ForecastResult{Place: forecastPlace(doc, place), Source: source, Status: "ok", Warnings: sourceWarnings(source), Periods: []Period{}}
	anchor := issue
	if anchor.IsZero() {
		anchor = c.cfg.Now().In(JST)
		result.Warnings = append(result.Warnings, "Forecast issuance is absent; month/day calendar labels use the fetch-time year with nearest-year rollover as an explicit assumption.")
	}
	tables := all(doc, func(n *html.Node) bool { return n.Data == "table" && hasClass(n, "forecast-point-1h") })
	if len(tables) == 0 {
		return result, errors.New("tenki.jp hourly markup unavailable: no forecast-point-1h tables")
	}
	for _, table := range tables {
		day := dateFromText(nodeText(class(table, "day-box")), anchor)
		if day.IsZero() {
			return result, errors.New("hourly table has no valid calendar date")
		}
		rows := map[string][]*html.Node{}
		for _, row := range all(table, func(n *html.Node) bool { return n.Data == "tr" }) {
			for _, name := range strings.Fields(attr(row, "class")) {
				rows[name] = childElements(row, "td")
			}
		}
		hours := rows["hour"]
		if len(hours) != 24 {
			return result, fmt.Errorf("hourly date %s has %d hour columns, expected 24", day.Format("2006-01-02"), len(hours))
		}
		for i, h := range hours {
			hour := atoi(nodeText(h))
			if hour < 1 || hour > 24 {
				return result, errors.New("hourly table has unsupported hour label")
			}
			end := day.Add(time.Duration(hour) * time.Hour)
			p := Period{Date: day.Format("2006-01-02"), Start: stamp(end.Add(-time.Hour)), End: stamp(end), ValidAt: stamp(end), Kind: kindAt(hours, i), Weather: altAt(rows["weather"], i), TemperatureC: valueAt(rows["temperature"], i), PrecipProbabilityPct: valueAt(rows["prob-precip"], i), PrecipRateMMH: valueAt(rows["precipitation"], i), HumidityPct: valueAt(rows["humidity"], i), WindSpeedMS: valueAt(rows["wind-speed"], i), WindDirection: altAt(rows["wind-blow"], i)}
			if kindAt(rows["weather"], i) == "estimated_actual" {
				p.Kind = "estimated_actual"
			}
			p.TemperatureKind, p.WeatherProbabilityKind = p.Kind, p.Kind
			result.Periods = append(result.Periods, p)
		}
	}
	coverage(&result)
	result.Warnings = append(result.Warnings, "Hourly precipitation covers the preceding hour; temperature and wind are instantaneous at valid_at. Grey elapsed values are nearby estimates, not station observations.")
	return result, nil
}

var chartDataRE = regexp.MustCompile(`\bdata\s*:\s*\[([^\]]*)\]`)

func (c *Client) parseDaily(body string, source Source, place Place) (ForecastResult, error) {
	doc := parseHTML(body)
	issue, raw := forecastIssue(doc)
	applyIssue(&source, issue, raw, 2*time.Hour, c.cfg.Now())
	result := ForecastResult{Place: forecastPlace(doc, place), Source: source, Status: "ok", Warnings: sourceWarnings(source), Periods: []Period{}, Intervals: []Period{}, Instants: []Period{}}
	anchor := issue
	if anchor.IsZero() {
		anchor = c.cfg.Now().In(JST)
		result.Warnings = append(result.Warnings, "Forecast issuance is absent; month/day calendar labels use the fetch-time year with nearest-year rollover as an explicit assumption.")
	}
	entries := all(doc, func(n *html.Node) bool { return n.Data == "dd" && hasClass(n, "forecast10days-actab") })
	if len(entries) == 0 {
		return result, errors.New("tenki.jp daily markup unavailable: no forecast10days-actab entries")
	}
	for _, entry := range entries {
		day := dateFromText(nodeText(class(entry, "days")), anchor)
		if day.IsZero() {
			return result, errors.New("daily entry has no valid calendar date")
		}
		p := Period{Date: day.Format("2006-01-02"), Start: stamp(day), End: stamp(day.AddDate(0, 0, 1)), Kind: "forecast", Weather: imageAlt(class(entry, "forecast")), MinTemperatureC: number(nodeText(class(entry, "low-temp"))), MaxTemperatureC: number(nodeText(class(entry, "high-temp"))), PrecipProbabilityPct: number(nodeText(class(entry, "prob-precip"))), PrecipAmountMM: number(nodeText(class(entry, "precip"))), Confidence: nodeText(class(entry, "accuracy"))}
		p.TemperatureKind, p.WeatherProbabilityKind = "forecast", "forecast"
		if day.Format("2006-01-02") == anchor.Format("2006-01-02") {
			p.Partial = true
			p.Kind = "mixed"
			p.TemperatureKind = "forecast_or_estimated_actual"
			if past(class(entry, "high-temp")) && past(class(entry, "low-temp")) {
				p.TemperatureKind = "estimated_actual"
			}
			if !issue.IsZero() {
				p.WeatherProbabilityFrom = stamp(issue)
			}
		}
		result.Periods = append(result.Periods, p)
		detail := class(entry, "forecast10days-actab-content-list")
		if detail == nil {
			continue
		}
		hours := childElements(class(detail, "time-item"), "span")
		weather := childElements(class(detail, "forecast-item"), "p")
		pop := childElements(class(detail, "prob-precip-item"), "span")
		rain := childElements(class(detail, "precip-item"), "span")
		humidity := childElements(class(detail, "humidity-item"), "span")
		wind := childElements(class(detail, "wind-item"), "p")
		temps := []*float64{}
		if m := chartDataRE.FindStringSubmatch(rawText(class(detail, "temp-item"))); m != nil {
			for _, v := range strings.Split(m[1], ",") {
				temps = append(temps, number(strings.TrimSpace(v)))
			}
		}
		if len(hours) != 5 {
			return result, fmt.Errorf("daily detail %s has unsupported instant labels", p.Date)
		}
		for i, h := range hours {
			hour := atoi(nodeText(h))
			if hour != i*6 {
				return result, errors.New("daily detail instant alignment changed")
			}
			instant := day.Add(time.Duration(hour) * time.Hour)
			inst := Period{Date: p.Date, ValidAt: stamp(instant), Kind: kindAt(hours, i), HumidityPct: valueAt(humidity, i), WindSpeedMS: valueAt(wind, i), WindDirection: altAt(wind, i)}
			inst.TemperatureKind = inst.Kind
			if i < len(temps) {
				inst.TemperatureC = temps[i]
			}
			result.Instants = append(result.Instants, inst)
			if i < 4 {
				interval := Period{Date: p.Date, Start: stamp(instant), End: stamp(instant.Add(6 * time.Hour)), Kind: kindAt(pop, i), Weather: altAt(weather, i), PrecipProbabilityPct: valueAt(pop, i), PrecipAmountMM: valueAt(rain, i)}
				result.Intervals = append(result.Intervals, interval)
			}
		}
	}
	coverage(&result)
	if len(result.Periods) > 0 && result.Periods[0].Partial && !issue.IsZero() {
		result.CoverageStart = stamp(issue)
	}
	result.Warnings = append(result.Warnings, "Daily minimum is the morning low and maximum the daytime high. Today's temperatures may be forecasts or nearby estimated actuals once those periods have passed; source markup does not identify a cutoff. Today's symbol/probability apply after weather_probability_from. Daily precipitation amount remains the source daily summary, with no assumed post-issue interval. Daily wind is unavailable; six-hour precipitation intervals and five temperature/wind instants are separate.")
	return result, nil
}
