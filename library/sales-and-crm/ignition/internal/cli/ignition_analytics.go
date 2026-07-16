package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/ignition/internal/client"
)

const (
	analyticsPageSize = 200
	analyticsMaxPages = 25

	proposalSearchQuery = `query pagedProposals($searchType: SearchType!, $textFilters: [SearchQueryTextFilterInput!], $pagination: PaginationInput) { search { pagedQuery(type: $searchType, textFilters: $textFilters, pagination: $pagination) { results { nodes { ... on ProposalResult { id name status client { id name } __typename } __typename } } totalCount __typename } __typename } }`
	invoiceSearchQuery  = `query pagedInvoices($searchType: SearchType!, $textFilters: [SearchQueryTextFilterInput!], $pagination: PaginationInput) { search { pagedQuery(type: $searchType, textFilters: $textFilters, pagination: $pagination) { results { nodes { ... on InvoiceResult { id externalNumber ledgerName billedOn collectionOn amountWithTax { format } paymentStatus { displayName } paymentProgress { status } client { id name } __typename } __typename } } totalCount __typename } __typename } }`
	billingSearchQuery  = `query pagedBillingItems($searchType: SearchType!, $textFilters: [SearchQueryTextFilterInput!], $pagination: PaginationInput) { search { pagedQuery(type: $searchType, textFilters: $textFilters, pagination: $pagination) { results { nodes { ... on BillingItemResult { id amount { format } amountWithTax { format } billingItemStatus billingStrategy client { id name } date itemPrice { description displayName type } serviceName __typename } __typename } } totalCount __typename } __typename } }`
)

type searchClientRef struct {
	ID   string `json:"id,omitempty"`
	Name string `json:"name,omitempty"`
}

type searchMoney struct {
	Format string `json:"format,omitempty"`
}

type searchPaymentStatus struct {
	DisplayName string `json:"displayName,omitempty"`
}

type searchPaymentProgress struct {
	Status string `json:"status,omitempty"`
}

type searchItemPrice struct {
	Description string `json:"description,omitempty"`
	DisplayName string `json:"displayName,omitempty"`
	Type        string `json:"type,omitempty"`
}

type searchNode struct {
	ID                string                 `json:"id,omitempty"`
	Name              string                 `json:"name,omitempty"`
	Status            string                 `json:"status,omitempty"`
	ExternalNumber    string                 `json:"externalNumber,omitempty"`
	LedgerName        string                 `json:"ledgerName,omitempty"`
	BilledOn          string                 `json:"billedOn,omitempty"`
	CollectionOn      string                 `json:"collectionOn,omitempty"`
	Amount            *searchMoney           `json:"amount,omitempty"`
	AmountWithTax     *searchMoney           `json:"amountWithTax,omitempty"`
	PaymentStatus     *searchPaymentStatus   `json:"paymentStatus,omitempty"`
	PaymentProgress   *searchPaymentProgress `json:"paymentProgress,omitempty"`
	BillingItemStatus string                 `json:"billingItemStatus,omitempty"`
	BillingStrategy   string                 `json:"billingStrategy,omitempty"`
	Date              string                 `json:"date,omitempty"`
	ItemPrice         *searchItemPrice       `json:"itemPrice,omitempty"`
	ServiceName       string                 `json:"serviceName,omitempty"`
	Client            *searchClientRef       `json:"client,omitempty"`
	TypeName          string                 `json:"__typename,omitempty"`
}

func fetchSearchIndex(ctx context.Context, c *client.Client, searchType, query, opName string) ([]searchNode, error) {
	var all []searchNode
	for page := 1; page <= analyticsMaxPages; page++ {
		body := map[string]any{
			"operationName": opName,
			"query":         query,
			"variables": map[string]any{
				"searchType":  searchType,
				"textFilters": []any{},
				"pagination": map[string]any{
					"pageNumber": page,
					"pageSize":   analyticsPageSize,
				},
			},
		}
		data, _, err := c.PostWithParams(ctx, "/graphql", map[string]string{}, body)
		if err != nil {
			return nil, err
		}
		nodes, totalCount, err := decodeSearchNodes(data)
		if err != nil {
			return nil, err
		}
		if len(nodes) == 0 {
			break
		}
		all = append(all, nodes...)
		if page*analyticsPageSize >= totalCount {
			break
		}
	}
	return all, nil
}

func decodeSearchNodes(data json.RawMessage) ([]searchNode, int, error) {
	var envelope struct {
		Data struct {
			Search struct {
				PagedQuery struct {
					Results struct {
						Nodes []json.RawMessage `json:"nodes"`
					} `json:"results"`
					TotalCount int `json:"totalCount"`
				} `json:"pagedQuery"`
			} `json:"search"`
		} `json:"data"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		return nil, 0, fmt.Errorf("parsing search index response: %w", err)
	}
	nodes := make([]searchNode, 0, len(envelope.Data.Search.PagedQuery.Results.Nodes))
	for _, raw := range envelope.Data.Search.PagedQuery.Results.Nodes {
		node, err := decodeSearchNode(raw)
		if err != nil {
			return nil, 0, err
		}
		nodes = append(nodes, node)
	}
	return nodes, envelope.Data.Search.PagedQuery.TotalCount, nil
}

func decodeSearchNode(raw json.RawMessage) (searchNode, error) {
	var node searchNode
	if err := json.Unmarshal(raw, &node); err != nil {
		return searchNode{}, fmt.Errorf("parsing search node: %w", err)
	}
	return node, nil
}

func parseMoneyFormat(s string) float64 {
	var numeric []byte
	seenDot := false
	for i := 0; i < len(s); i++ {
		ch := s[i]
		switch {
		case ch >= '0' && ch <= '9':
			numeric = append(numeric, ch)
		case ch == '.' && !seenDot:
			numeric = append(numeric, ch)
			seenDot = true
		case ch == '-' && len(numeric) == 0:
			numeric = append(numeric, ch)
		}
	}
	if len(numeric) == 0 || string(numeric) == "-" || string(numeric) == "." || string(numeric) == "-." {
		return 0
	}
	value, err := strconv.ParseFloat(string(numeric), 64)
	if err != nil {
		return 0
	}
	return value
}

func nodeAmount(node searchNode) float64 {
	if node.TypeName == "InvoiceResult" || node.PaymentProgress != nil || node.PaymentStatus != nil || node.ExternalNumber != "" {
		if node.AmountWithTax == nil {
			return 0
		}
		return parseMoneyFormat(node.AmountWithTax.Format)
	}
	if node.Amount == nil {
		return 0
	}
	return parseMoneyFormat(node.Amount.Format)
}

func nodeStatus(node searchNode) string {
	if node.TypeName == "InvoiceResult" || node.PaymentProgress != nil || node.PaymentStatus != nil || node.ExternalNumber != "" {
		if node.PaymentProgress != nil && node.PaymentProgress.Status != "" {
			return node.PaymentProgress.Status
		}
		if node.PaymentStatus != nil {
			return node.PaymentStatus.DisplayName
		}
		return ""
	}
	if node.TypeName == "BillingItemResult" || node.BillingItemStatus != "" || node.ServiceName != "" || node.Amount != nil {
		return node.BillingItemStatus
	}
	return node.Status
}

func nodeDisplayID(node searchNode) string {
	if node.ExternalNumber != "" {
		return node.ExternalNumber
	}
	return node.ID
}
