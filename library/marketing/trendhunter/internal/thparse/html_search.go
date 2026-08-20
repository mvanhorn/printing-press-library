package thparse

func ParseSearchResults(body []byte, query string) ([]Trend, error) {
	trends, err := ParseCardList(body, "search")
	if err != nil {
		return nil, err
	}
	for i := range trends {
		if trends[i].Description == "" {
			trends[i].Description = query
		}
	}
	return trends, nil
}
