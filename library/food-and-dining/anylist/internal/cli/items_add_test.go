package cli

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestItemsAddBaseWriteRequiresExplicitApply(t *testing.T) {
	t.Parallel()
	var output bytes.Buffer
	cmd := newItemsAddCmd(&rootFlags{asJSON: true})
	cmd.SetOut(&output)
	cmd.SetArgs([]string{"--list", "disposable-list", "--item", "disposable-item"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("base preview returned error: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(output.Bytes(), &got); err != nil {
		t.Fatalf("preview was not JSON: %v (%q)", err, output.String())
	}
	if got["dry_run"] != true || got["apply"] != false {
		t.Fatalf("unexpected base write gate: %#v", got)
	}
}

func TestItemsAddMetadataRequiresExplicitApply(t *testing.T) {
	t.Parallel()

	cmd := newItemsAddCmd(&rootFlags{})
	cmd.SetArgs([]string{"--list", "disposable-list", "--item", "disposable-item", "--category", "Produce"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("metadata preview returned error: %v", err)
	}
}

func TestItemsAddDryRunKeepsMetadataPreviewOffline(t *testing.T) {
	t.Parallel()

	cmd := newItemsAddCmd(&rootFlags{dryRun: true})
	cmd.SetArgs([]string{
		"--list", "disposable-list",
		"--item", "disposable-item",
		"--category", "Produce",
		"--price", "7.31",
	})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("dry-run returned error: %v", err)
	}
}

func TestItemsAddMetadataPreviewIncludesStoreAndApplyGate(t *testing.T) {
	var output bytes.Buffer
	cmd := newItemsAddCmd(&rootFlags{asJSON: true})
	cmd.SetOut(&output)
	cmd.SetArgs([]string{
		"--list", "disposable-list",
		"--item", "disposable-item",
		"--store", "Paris Walmart",
		"--price", "4.25",
	})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("metadata preview returned error: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(output.Bytes(), &got); err != nil {
		t.Fatalf("preview was not JSON: %v (%q)", err, output.String())
	}
	if got["dry_run"] != true || got["apply"] != false {
		t.Fatalf("unexpected preview gate: %#v", got)
	}
	if got["store"] != "Paris Walmart" {
		t.Fatalf("store was not preserved in preview: %#v", got["store"])
	}
}

func TestItemsAddPriceStoreRequiresPrice(t *testing.T) {
	cmd := newItemsAddCmd(&rootFlags{})
	cmd.SetArgs([]string{
		"--list", "disposable-list",
		"--item", "disposable-item",
		"--price-store", "store-id",
	})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected --price-store without --price to fail")
	}
}
