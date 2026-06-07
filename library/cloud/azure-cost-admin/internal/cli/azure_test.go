package cli

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestRedactAzureSensitiveValues(t *testing.T) {
	subscriptionID := strings.Join([]string{"12345678", "1234", "1234", "1234", "123456789abc"}, "-")
	email := "admin" + "@" + "example.com"
	token := strings.Join([]string{"abc", "def", "ghi"}, ".")
	input := strings.Join([]string{
		"subscription " + subscriptionID,
		"user " + email,
		"/subscriptions/" + subscriptionID + "/resourceGroups/prod/providers/Microsoft.Compute/virtualMachines/vm1",
		"Bearer " + token,
	}, "\n")

	got := redactAzureText(input)

	for _, secret := range []string{
		subscriptionID,
		email,
		token,
		"Microsoft.Compute/virtualMachines/vm1",
	} {
		if strings.Contains(got, secret) {
			t.Fatalf("redacted output still contains %q: %s", secret, got)
		}
	}
}

func TestParseCostRowsMapsColumnsAndTotals(t *testing.T) {
	var response costQueryResponse
	err := json.Unmarshal([]byte(`{
	  "properties": {
	    "columns": [
	      {"name":"Cost","type":"Number"},
	      {"name":"Currency","type":"String"},
	      {"name":"ServiceName","type":"String"}
	    ],
	    "rows": [
	      [12.5, "USD", "Storage"],
	      [5.0, "USD", "Compute"]
	    ]
	  }
	}`), &response)
	if err != nil {
		t.Fatalf("unmarshal cost response: %v", err)
	}

	rows := parseCostRows(response)

	if len(rows) != 2 {
		t.Fatalf("got %d rows, want 2", len(rows))
	}
	if rows[0].Cost != 12.5 || rows[0].Currency != "USD" || rows[0].Group != "Storage" {
		t.Fatalf("unexpected first row: %+v", rows[0])
	}
}

func TestBuildCostQuerySupportsActualCostGrouping(t *testing.T) {
	body, err := buildCostQuery(costQueryOptions{
		Timeframe: "MonthToDate",
		From:      "",
		To:        "",
		GroupBy:   "ServiceName",
	})
	if err != nil {
		t.Fatalf("buildCostQuery failed: %v", err)
	}

	bodyText := string(body)
	for _, expected := range []string{`"type":"ActualCost"`, `"timeframe":"MonthToDate"`, `"name":"Cost"`, `"name":"ServiceName"`} {
		if !strings.Contains(bodyText, expected) {
			t.Fatalf("query body missing %s: %s", expected, bodyText)
		}
	}
}

func TestBuildMissingTagQueryEscapesTagNameAndLimitsResults(t *testing.T) {
	query := buildMissingTagQuery("owner", "rg-data", 25)

	for _, expected := range []string{
		`Resources`,
		`isnull(tags['owner'])`,
		`resourceGroup == 'rg-data'`,
		`take 25`,
	} {
		if !strings.Contains(query, expected) {
			t.Fatalf("query missing %q: %s", expected, query)
		}
	}
}

func TestQueryMissingTagsUsesResourceGraphQueryFlag(t *testing.T) {
	runner := &recordingRunner{output: []byte(`{"data":[]}`)}
	app := defaultApp()
	app.runner = runner

	_, err := app.queryMissingTags(context.Background(), "sub-name", "owner", "", 5)
	if err != nil {
		t.Fatalf("queryMissingTags failed: %v", err)
	}

	joined := strings.Join(runner.args, " ")
	if !strings.Contains(joined, "--graph-query") {
		t.Fatalf("Resource Graph call did not use --graph-query: %s", joined)
	}
	if strings.Contains(joined, " --query ") {
		t.Fatalf("Resource Graph call used Azure CLI global --query flag: %s", joined)
	}
}

func TestSelectJSONFieldsProjectsSlices(t *testing.T) {
	selected, err := selectJSONFields([]costRow{
		{Group: "Storage", Cost: 12.5, Currency: "USD"},
	}, "group,cost")
	if err != nil {
		t.Fatalf("selectJSONFields failed: %v", err)
	}

	rows, ok := selected.([]any)
	if !ok || len(rows) != 1 {
		t.Fatalf("unexpected selected shape: %#v", selected)
	}
	row, ok := rows[0].(map[string]any)
	if !ok {
		t.Fatalf("unexpected row shape: %#v", rows[0])
	}
	if _, ok := row["currency"]; ok {
		t.Fatalf("unselected field was present: %#v", row)
	}
	if row["group"] != "Storage" || row["cost"] != 12.5 {
		t.Fatalf("selected fields missing: %#v", row)
	}
}

func TestParseRetailPriceRows(t *testing.T) {
	rows, err := parseRetailPriceResponse([]byte(`{
	  "Items": [
	    {
	      "serviceName": "Virtual Machines",
	      "skuName": "D2s v5",
	      "armRegionName": "eastus",
	      "retailPrice": 0.096,
	      "unitOfMeasure": "1 Hour",
	      "currencyCode": "USD"
	    }
	  ]
	}`))
	if err != nil {
		t.Fatalf("parseRetailPriceResponse failed: %v", err)
	}

	if len(rows) != 1 {
		t.Fatalf("got %d rows, want 1", len(rows))
	}
	if rows[0].ServiceName != "Virtual Machines" || rows[0].RetailPrice != 0.096 {
		t.Fatalf("unexpected row: %+v", rows[0])
	}
}

type recordingRunner struct {
	output []byte
	args   []string
}

func (r *recordingRunner) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	r.args = append([]string{name}, args...)
	return r.output, nil
}
