package thparse

func ParseIndexPage(body []byte, pageURL string) []Trend {
	trends, _ := ParseCardList(body, "index")
	for i := range trends {
		if trends[i].SourceURL == "" {
			trends[i].SourceURL = pageURL
		}
	}
	return trends
}
