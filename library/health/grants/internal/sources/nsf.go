package sources

import (
	"fmt"
	"net/url"
)

// NSF Awards API — awarded NSF grants. Keyless.
const nsfAwardsURL = "https://api.nsf.gov/services/v1/awards.json"

type NSFAward struct {
	ID             string `json:"id"`
	Title          string `json:"title"`
	FundsObligated string `json:"fundsObligatedAmt"`
	Awardee        string `json:"awardeeName"`
	StartDate      string `json:"startDate"`
	ExpDate        string `json:"expDate"`
}

type nsfResp struct {
	Response struct {
		Award []NSFAward `json:"award"`
	} `json:"response"`
}

// SearchNSF returns awarded NSF grants for a keyword (max 25/page per API).
func SearchNSF(keyword string, rows int) ([]NSFAward, error) {
	if rows > 25 {
		rows = 25
	}
	q := url.Values{}
	q.Set("keyword", keyword)
	q.Set("rpp", fmt.Sprint(rows))
	q.Set("printFields", "id,title,fundsObligatedAmt,awardeeName,startDate,expDate")
	var resp nsfResp
	if err := getJSON(nsfAwardsURL+"?"+q.Encode(), &resp); err != nil {
		return nil, fmt.Errorf("NSF awards: %w", err)
	}
	return resp.Response.Award, nil
}
