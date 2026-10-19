package parser

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// The transactions page is a Next.js app: the first page of rows, the cursor
// for the next page and the token for the get-transactions XHR are all in the
// __NEXT_DATA__ script. The legacy apx-transactions-* DOM no longer exists.

var nextDataRegex = regexp.MustCompile(`(?s)<script id="__NEXT_DATA__"[^>]*>(.*?)</script>`)

type rawTransaction struct {
	FormattedAmount string `json:"formattedAmount"`
	FormattedDate   string `json:"formattedDate"`
	OrderData       []struct {
		OrderDisplayString string `json:"orderDisplayString"`
	} `json:"orderData"`
	StatementDescriptor string `json:"statementDescriptor"`
	StatusInfo          struct {
		Label string `json:"label"`
	} `json:"statusInfo"`
	PaymentMethodDisplayStringData struct {
		PaymentMethodName   string `json:"paymentMethodName"`
		PaymentMethodNumber struct {
			LastDigits string `json:"lastDigits"`
		} `json:"paymentMethodNumber"`
	} `json:"paymentMethodDisplayStringData"`
}

func parseTransactionsNextData(htmlBytes []byte) (*TransactionsPage, bool) {
	m := nextDataRegex.FindSubmatch(htmlBytes)
	if m == nil {
		return nil, false
	}
	var nd struct {
		Props struct {
			PageProps struct {
				Token string `json:"token"`
				State struct {
					TransactionResponseState *struct {
						Visible struct {
							TransactionList []rawTransaction `json:"transactionList"`
						} `json:"visibleTransactionResponse"`
						HasMore              bool   `json:"hasMore"`
						LastEvaluatedPageKey string `json:"lastEvaluatedPageKey"`
					} `json:"transactionResponseState"`
				} `json:"state"`
			} `json:"pageProps"`
		} `json:"props"`
	}
	if json.Unmarshal(m[1], &nd) != nil || nd.Props.PageProps.State.TransactionResponseState == nil {
		return nil, false
	}
	st := nd.Props.PageProps.State.TransactionResponseState
	page := &TransactionsPage{Transactions: convertRawTransactions(st.Visible.TransactionList), Token: nd.Props.PageProps.Token}
	if st.HasMore {
		page.NextPageKey = st.LastEvaluatedPageKey
	}
	return page, true
}

// ParseTransactionsXHR parses one get-transactions XHR response (older pages).
func ParseTransactionsXHR(body []byte) (*TransactionsPage, error) {
	var r struct {
		ResponseCode    string `json:"responseCode"`
		DisplayResponse *struct {
			TransactionsList []rawTransaction `json:"transactionsList"`
			LastEvaluatedKey string           `json:"lastEvaluatedKey"`
		} `json:"displayResponse"`
	}
	if err := json.Unmarshal(body, &r); err != nil {
		return nil, err
	}
	// A JSON error can arrive with HTTP 200; don't mistake it for the end of history.
	if r.ResponseCode != "success" || r.DisplayResponse == nil {
		return nil, fmt.Errorf("get-transactions returned responseCode %q without transactions", r.ResponseCode)
	}
	return &TransactionsPage{
		Transactions: convertRawTransactions(r.DisplayResponse.TransactionsList),
		NextPageKey:  r.DisplayResponse.LastEvaluatedKey,
	}, nil
}

func convertRawTransactions(raw []rawTransaction) []Transaction {
	out := make([]Transaction, 0, len(raw))
	for _, r := range raw {
		t := Transaction{
			DateRaw:       r.FormattedDate,
			PaymentMethod: r.PaymentMethodDisplayStringData.PaymentMethodName,
			PaymentLast4:  r.PaymentMethodDisplayStringData.PaymentMethodNumber.LastDigits,
			Merchant:      r.StatementDescriptor,
			Description:   r.StatementDescriptor,
			Status:        r.StatusInfo.Label,
		}
		if d, err := time.Parse("Jan 2, 2006", r.FormattedDate); err == nil {
			t.Date = d.Format("2006-01-02")
		} else if pt := ParseDate(r.FormattedDate); !pt.IsZero() {
			t.Date = pt.Format("2006-01-02")
		}
		amt := strings.NewReplacer("$", "", ",", "", "+", "", " ", "").Replace(r.FormattedAmount)
		if v, err := strconv.ParseFloat(amt, 64); err == nil {
			t.Amount = v // keeps the sign: "-$32.55" charge, "+$24.97" refund
		} else {
			t.Amount = ExtractMoney(r.FormattedAmount)
		}
		for _, o := range r.OrderData {
			id := strings.TrimSpace(strings.TrimPrefix(o.OrderDisplayString, "Order #"))
			if id != "" {
				t.OrderIDs = append(t.OrderIDs, id)
			}
		}
		if len(t.OrderIDs) > 0 {
			t.OrderID = t.OrderIDs[0]
		}
		out = append(out, t)
	}
	return out
}
