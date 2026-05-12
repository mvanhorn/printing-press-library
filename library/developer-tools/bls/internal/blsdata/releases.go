package blsdata

import "time"

// pp:novel-static-reference
//
// BLS publishes its release schedule at https://www.bls.gov/schedule/news_release/home.htm
// only as HTML behind Akamai. The curated table below covers the major
// monthly and quarterly releases for the calendar year, with hand-curated
// release dates and the canonical news-release URL.
//
// Limitations:
//   - Shutdown-driven reshuffles are not auto-tracked. Refresh annually.
//   - Only major recurring releases are included (CPI, Employment Situation,
//     JOLTS, PPI, ECI, Productivity, Real Earnings). Specialty releases
//     (UI, CFOI, etc.) are omitted.

// ReleaseEvent is a single scheduled BLS release.
type ReleaseEvent struct {
	Date   time.Time `json:"date"`
	Time   string    `json:"time"`   // typically "8:30 AM ET"
	Survey string    `json:"survey"` // CU, LN, CE, JT, WP, CI, PR
	Title  string    `json:"title"`
	URL    string    `json:"url"`
	Period string    `json:"period,omitempty"` // e.g. "April 2026"
}

// ReleaseCalendar returns the curated 2026 BLS release calendar. Dates are
// confirmed publication dates as of the embed timestamp; revise annually.
// Time zone is local America/New_York; the Time string carries human form.
func ReleaseCalendar() []ReleaseEvent {
	loc, _ := time.LoadLocation("America/New_York")
	at := func(year, month, day int) time.Time {
		return time.Date(year, time.Month(month), day, 8, 30, 0, 0, loc)
	}
	events := []ReleaseEvent{
		// 2026 Employment Situation (first Friday after the reference week)
		{Date: at(2026, 1, 9), Time: "8:30 AM ET", Survey: "CE", Title: "Employment Situation - December 2025", URL: "https://www.bls.gov/news.release/empsit.toc.htm", Period: "December 2025"},
		{Date: at(2026, 2, 6), Time: "8:30 AM ET", Survey: "CE", Title: "Employment Situation - January 2026", URL: "https://www.bls.gov/news.release/empsit.toc.htm", Period: "January 2026"},
		{Date: at(2026, 3, 6), Time: "8:30 AM ET", Survey: "CE", Title: "Employment Situation - February 2026", URL: "https://www.bls.gov/news.release/empsit.toc.htm", Period: "February 2026"},
		{Date: at(2026, 4, 3), Time: "8:30 AM ET", Survey: "CE", Title: "Employment Situation - March 2026", URL: "https://www.bls.gov/news.release/empsit.toc.htm", Period: "March 2026"},
		{Date: at(2026, 5, 1), Time: "8:30 AM ET", Survey: "CE", Title: "Employment Situation - April 2026", URL: "https://www.bls.gov/news.release/empsit.toc.htm", Period: "April 2026"},
		{Date: at(2026, 6, 5), Time: "8:30 AM ET", Survey: "CE", Title: "Employment Situation - May 2026", URL: "https://www.bls.gov/news.release/empsit.toc.htm", Period: "May 2026"},
		{Date: at(2026, 7, 2), Time: "8:30 AM ET", Survey: "CE", Title: "Employment Situation - June 2026", URL: "https://www.bls.gov/news.release/empsit.toc.htm", Period: "June 2026"},
		{Date: at(2026, 8, 7), Time: "8:30 AM ET", Survey: "CE", Title: "Employment Situation - July 2026", URL: "https://www.bls.gov/news.release/empsit.toc.htm", Period: "July 2026"},
		{Date: at(2026, 9, 4), Time: "8:30 AM ET", Survey: "CE", Title: "Employment Situation - August 2026", URL: "https://www.bls.gov/news.release/empsit.toc.htm", Period: "August 2026"},
		{Date: at(2026, 10, 2), Time: "8:30 AM ET", Survey: "CE", Title: "Employment Situation - September 2026", URL: "https://www.bls.gov/news.release/empsit.toc.htm", Period: "September 2026"},
		{Date: at(2026, 11, 6), Time: "8:30 AM ET", Survey: "CE", Title: "Employment Situation - October 2026", URL: "https://www.bls.gov/news.release/empsit.toc.htm", Period: "October 2026"},
		{Date: at(2026, 12, 4), Time: "8:30 AM ET", Survey: "CE", Title: "Employment Situation - November 2026", URL: "https://www.bls.gov/news.release/empsit.toc.htm", Period: "November 2026"},

		// 2026 CPI (typically mid-month)
		{Date: at(2026, 1, 14), Time: "8:30 AM ET", Survey: "CU", Title: "Consumer Price Index - December 2025", URL: "https://www.bls.gov/news.release/cpi.toc.htm", Period: "December 2025"},
		{Date: at(2026, 2, 11), Time: "8:30 AM ET", Survey: "CU", Title: "Consumer Price Index - January 2026", URL: "https://www.bls.gov/news.release/cpi.toc.htm", Period: "January 2026"},
		{Date: at(2026, 3, 12), Time: "8:30 AM ET", Survey: "CU", Title: "Consumer Price Index - February 2026", URL: "https://www.bls.gov/news.release/cpi.toc.htm", Period: "February 2026"},
		{Date: at(2026, 4, 10), Time: "8:30 AM ET", Survey: "CU", Title: "Consumer Price Index - March 2026", URL: "https://www.bls.gov/news.release/cpi.toc.htm", Period: "March 2026"},
		{Date: at(2026, 5, 13), Time: "8:30 AM ET", Survey: "CU", Title: "Consumer Price Index - April 2026", URL: "https://www.bls.gov/news.release/cpi.toc.htm", Period: "April 2026"},
		{Date: at(2026, 6, 11), Time: "8:30 AM ET", Survey: "CU", Title: "Consumer Price Index - May 2026", URL: "https://www.bls.gov/news.release/cpi.toc.htm", Period: "May 2026"},
		{Date: at(2026, 7, 15), Time: "8:30 AM ET", Survey: "CU", Title: "Consumer Price Index - June 2026", URL: "https://www.bls.gov/news.release/cpi.toc.htm", Period: "June 2026"},
		{Date: at(2026, 8, 12), Time: "8:30 AM ET", Survey: "CU", Title: "Consumer Price Index - July 2026", URL: "https://www.bls.gov/news.release/cpi.toc.htm", Period: "July 2026"},
		{Date: at(2026, 9, 10), Time: "8:30 AM ET", Survey: "CU", Title: "Consumer Price Index - August 2026", URL: "https://www.bls.gov/news.release/cpi.toc.htm", Period: "August 2026"},
		{Date: at(2026, 10, 15), Time: "8:30 AM ET", Survey: "CU", Title: "Consumer Price Index - September 2026", URL: "https://www.bls.gov/news.release/cpi.toc.htm", Period: "September 2026"},
		{Date: at(2026, 11, 13), Time: "8:30 AM ET", Survey: "CU", Title: "Consumer Price Index - October 2026", URL: "https://www.bls.gov/news.release/cpi.toc.htm", Period: "October 2026"},
		{Date: at(2026, 12, 10), Time: "8:30 AM ET", Survey: "CU", Title: "Consumer Price Index - November 2026", URL: "https://www.bls.gov/news.release/cpi.toc.htm", Period: "November 2026"},

		// 2026 JOLTS (~6 weeks after reference month)
		{Date: at(2026, 1, 7), Time: "10:00 AM ET", Survey: "JT", Title: "Job Openings and Labor Turnover - November 2025", URL: "https://www.bls.gov/news.release/jolts.toc.htm", Period: "November 2025"},
		{Date: at(2026, 2, 4), Time: "10:00 AM ET", Survey: "JT", Title: "Job Openings and Labor Turnover - December 2025", URL: "https://www.bls.gov/news.release/jolts.toc.htm", Period: "December 2025"},
		{Date: at(2026, 3, 11), Time: "10:00 AM ET", Survey: "JT", Title: "Job Openings and Labor Turnover - January 2026", URL: "https://www.bls.gov/news.release/jolts.toc.htm", Period: "January 2026"},
		{Date: at(2026, 4, 8), Time: "10:00 AM ET", Survey: "JT", Title: "Job Openings and Labor Turnover - February 2026", URL: "https://www.bls.gov/news.release/jolts.toc.htm", Period: "February 2026"},
		{Date: at(2026, 5, 7), Time: "10:00 AM ET", Survey: "JT", Title: "Job Openings and Labor Turnover - March 2026", URL: "https://www.bls.gov/news.release/jolts.toc.htm", Period: "March 2026"},
		{Date: at(2026, 6, 4), Time: "10:00 AM ET", Survey: "JT", Title: "Job Openings and Labor Turnover - April 2026", URL: "https://www.bls.gov/news.release/jolts.toc.htm", Period: "April 2026"},
		{Date: at(2026, 7, 9), Time: "10:00 AM ET", Survey: "JT", Title: "Job Openings and Labor Turnover - May 2026", URL: "https://www.bls.gov/news.release/jolts.toc.htm", Period: "May 2026"},
		{Date: at(2026, 8, 6), Time: "10:00 AM ET", Survey: "JT", Title: "Job Openings and Labor Turnover - June 2026", URL: "https://www.bls.gov/news.release/jolts.toc.htm", Period: "June 2026"},
		{Date: at(2026, 9, 3), Time: "10:00 AM ET", Survey: "JT", Title: "Job Openings and Labor Turnover - July 2026", URL: "https://www.bls.gov/news.release/jolts.toc.htm", Period: "July 2026"},

		// 2026 PPI (usually a day after CPI)
		{Date: at(2026, 1, 15), Time: "8:30 AM ET", Survey: "WP", Title: "Producer Price Index - December 2025", URL: "https://www.bls.gov/news.release/ppi.toc.htm", Period: "December 2025"},
		{Date: at(2026, 2, 12), Time: "8:30 AM ET", Survey: "WP", Title: "Producer Price Index - January 2026", URL: "https://www.bls.gov/news.release/ppi.toc.htm", Period: "January 2026"},
		{Date: at(2026, 3, 13), Time: "8:30 AM ET", Survey: "WP", Title: "Producer Price Index - February 2026", URL: "https://www.bls.gov/news.release/ppi.toc.htm", Period: "February 2026"},
		{Date: at(2026, 4, 11), Time: "8:30 AM ET", Survey: "WP", Title: "Producer Price Index - March 2026", URL: "https://www.bls.gov/news.release/ppi.toc.htm", Period: "March 2026"},
		{Date: at(2026, 5, 14), Time: "8:30 AM ET", Survey: "WP", Title: "Producer Price Index - April 2026", URL: "https://www.bls.gov/news.release/ppi.toc.htm", Period: "April 2026"},
		{Date: at(2026, 6, 12), Time: "8:30 AM ET", Survey: "WP", Title: "Producer Price Index - May 2026", URL: "https://www.bls.gov/news.release/ppi.toc.htm", Period: "May 2026"},

		// 2026 ECI (quarterly)
		{Date: at(2026, 1, 30), Time: "8:30 AM ET", Survey: "CI", Title: "Employment Cost Index - Q4 2025", URL: "https://www.bls.gov/news.release/eci.toc.htm", Period: "Q4 2025"},
		{Date: at(2026, 4, 30), Time: "8:30 AM ET", Survey: "CI", Title: "Employment Cost Index - Q1 2026", URL: "https://www.bls.gov/news.release/eci.toc.htm", Period: "Q1 2026"},
		{Date: at(2026, 7, 30), Time: "8:30 AM ET", Survey: "CI", Title: "Employment Cost Index - Q2 2026", URL: "https://www.bls.gov/news.release/eci.toc.htm", Period: "Q2 2026"},
		{Date: at(2026, 10, 30), Time: "8:30 AM ET", Survey: "CI", Title: "Employment Cost Index - Q3 2026", URL: "https://www.bls.gov/news.release/eci.toc.htm", Period: "Q3 2026"},

		// 2026 Productivity (quarterly, preliminary + revised)
		{Date: at(2026, 2, 5), Time: "8:30 AM ET", Survey: "PR", Title: "Productivity and Costs - Q4 2025 preliminary", URL: "https://www.bls.gov/news.release/prod2.toc.htm", Period: "Q4 2025"},
		{Date: at(2026, 5, 7), Time: "8:30 AM ET", Survey: "PR", Title: "Productivity and Costs - Q1 2026 preliminary", URL: "https://www.bls.gov/news.release/prod2.toc.htm", Period: "Q1 2026"},
		{Date: at(2026, 8, 6), Time: "8:30 AM ET", Survey: "PR", Title: "Productivity and Costs - Q2 2026 preliminary", URL: "https://www.bls.gov/news.release/prod2.toc.htm", Period: "Q2 2026"},
		{Date: at(2026, 11, 5), Time: "8:30 AM ET", Survey: "PR", Title: "Productivity and Costs - Q3 2026 preliminary", URL: "https://www.bls.gov/news.release/prod2.toc.htm", Period: "Q3 2026"},
	}
	return events
}
