package cli

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/cliutil"
)

func etsyAssertBuyerRefusal(t *testing.T, out string, err error) {
	t.Helper()
	if err == nil || ExitCode(err) != 2 || !strings.Contains(err.Error(), "ETSY_MCP_ALLOW_BUYER_PII") {
		t.Fatalf("expected buyer opt-in usage error, got %v (stdout %q)", err, out)
	}
	etsyAssertNoBuyerValues(t, out)
}

func etsyAssertNoBuyerValues(t *testing.T, out string) {
	t.Helper()
	for _, value := range []string{"jordan@example.test", "Jordan Example", "12 Pine Lane", "Please gift wrap", "7770003001", "7770003002"} {
		if strings.Contains(out, value) {
			t.Fatalf("buyer value %q leaked in stdout: %s", value, out)
		}
	}
}

func TestEtsyOrdersMCPBuyerCLI(t *testing.T) {
	t.Setenv(cliutil.MCPChildEnvVar, "1")
	t.Setenv("ETSY_MCP_ALLOW_BUYER_PII", "")
	s2SetNow(t, s2Epoch)
	db := s2ReceiptFixture(t)
	for _, opt := range []string{"", "0", "no", "2", "false"} {
		t.Run("denied_"+opt, func(t *testing.T) {
			t.Setenv("ETSY_MCP_ALLOW_BUYER_PII", opt)
			for _, extra := range [][]string{nil, {"--dry-run"}} {
				args := append([]string{"orders", "unshipped", "--full", "--db", db, "--json"}, extra...)
				out, _, err := s2Run(t, args...)
				etsyAssertBuyerRefusal(t, out, err)
			}
		})
	}
	out, _, err := s2Run(t, "orders", "unshipped", "--db", db, "--json")
	s2Must(t, err)
	etsyAssertNoBuyerValues(t, out)
	var rows []map[string]any
	shopxDecodeResults(t, out, &rows)
	if len(rows) != 2 {
		t.Fatalf("expected two receipts: %s", out)
	}
	for _, opt := range []string{"1", "true", "yes", " TrUe ", " YES "} {
		t.Run("allowed_"+opt, func(t *testing.T) {
			t.Setenv("ETSY_MCP_ALLOW_BUYER_PII", opt)
			out, _, err := s2Run(t, "orders", "unshipped", "--full", "--db", db, "--json")
			s2Must(t, err)
			if !strings.Contains(out, "Jordan Example") {
				t.Fatalf("missing buyer name: %s", out)
			}
		})
	}
	t.Setenv(cliutil.MCPChildEnvVar, "")
	out, _, err = s2Run(t, "orders", "unshipped", "--full", "--db", db, "--json")
	s2Must(t, err)
	if !strings.Contains(out, "Jordan Example") {
		t.Fatalf("missing buyer name outside MCP: %s", out)
	}
}

func TestEtsyAnalyticsMCPBuyerCLI(t *testing.T) {
	t.Setenv(cliutil.MCPChildEnvVar, "1")
	t.Setenv("ETSY_MCP_ALLOW_BUYER_PII", "")
	db := s2ReceiptFixture(t)
	for _, typ := range []string{"receipts", "transactions", "users", "reviews", "listings_reviews", "RECEIPTS", "brand_new_type"} {
		for _, group := range []string{"", "buyer_user_id"} {
			args := []string{"analytics", "--type", typ, "--db", db}
			if group != "" {
				args = append(args, "--group-by", group)
			}
			out, _, err := s2Run(t, args...)
			etsyAssertBuyerRefusal(t, out, err)
		}
	}
	out, _, err := s2Run(t, "analytics", "--type", "listings", "--db", db)
	s2Must(t, err)
	if !json.Valid([]byte(out)) || !strings.Contains(out, `"count": 0`) {
		t.Fatalf("missing count: %s", out)
	}
	_, _, err = s2Run(t, "analytics", "--db", db)
	s2Must(t, err)
	for _, opt := range []string{"1", "true", "yes", " TrUe "} {
		t.Setenv("ETSY_MCP_ALLOW_BUYER_PII", opt)
		for _, extra := range [][]string{nil, {"--group-by", "buyer_user_id"}} {
			_, _, err = s2Run(t, append([]string{"analytics", "--type", "receipts", "--db", db}, extra...)...)
			s2Must(t, err)
		}
	}
	t.Setenv("ETSY_MCP_ALLOW_BUYER_PII", "")
	t.Setenv(cliutil.MCPChildEnvVar, "")
	out, _, err = s2Run(t, "analytics", "--type", "receipts", "--group-by", "buyer_user_id", "--db", db)
	s2Must(t, err)
	if !strings.Contains(out, "7770003001") {
		t.Fatalf("missing buyer group outside MCP: %s", out)
	}
}

func TestEtsyWorkflowArchiveHiddenFromMCP(t *testing.T) {
	cmd, _, err := RootCmd().Find([]string{"workflow", "archive"})
	s2Must(t, err)
	if cmd.Annotations["mcp:hidden"] != "true" {
		t.Fatal("workflow archive must be hidden from MCP")
	}
}

func TestEtsyBuyerBearingType(t *testing.T) {
	for _, typ := range []string{"receipts", "users", "reviews", "brand_new_type"} {
		if !etsyBuyerBearingType(typ) {
			t.Errorf("%q must be buyer-bearing", typ)
		}
	}
	for _, typ := range []string{"listings", "shops", "listings_inventory", "Listings ", ""} {
		if etsyBuyerBearingType(typ) {
			t.Errorf("%q must not be buyer-bearing", typ)
		}
	}
}

func TestEtsyReviewsDigestTextMCPRefusal(t *testing.T) {
	etsyFixture := shopxNewFixture(t)
	etsyFixture.put(shopxReviewTypes[0], map[string]any{
		"listing_id": 1000000001, "rating": 2,
		"created_timestamp": shopxAgo(shopxDay),
		"review":            "linen apron arrived torn",
	})
	t.Setenv(cliutil.MCPChildEnvVar, "1")
	for _, etsyOpt := range []string{"unset", "", "0"} {
		t.Run("denied_"+etsyOpt, func(t *testing.T) {
			t.Setenv("ETSY_MCP_ALLOW_BUYER_PII", etsyOpt)
			if etsyOpt == "unset" {
				if etsyErr := os.Unsetenv("ETSY_MCP_ALLOW_BUYER_PII"); etsyErr != nil {
					t.Fatal(etsyErr)
				}
			}
			for _, etsyExtra := range [][]string{nil, {"--dry-run"}} {
				etsyArgs := append([]string{"reviews", "digest", "--text"}, etsyExtra...)
				etsyOut, _, etsyErr := shopxRun(t, etsyFixture, etsyArgs...)
				if strings.Contains(etsyOut, "arrived torn") {
					t.Fatalf("REVIEWS_DIGEST_TEXT_CHECK: review text leaked: %s", etsyOut)
				}
				etsyAssertBuyerRefusal(t, etsyOut, etsyErr)
			}
			etsyOut, _, etsyErr := shopxRun(t, etsyFixture, "reviews", "digest")
			s2Must(t, etsyErr)
			if strings.Contains(etsyOut, "arrived torn") {
				t.Fatalf("REVIEWS_DIGEST_TEXT_CHECK: text in counts-only digest: %s", etsyOut)
			}
			if shopxMustObject(t, etsyOut)["total"] != float64(1) {
				t.Fatalf("REVIEWS_DIGEST_TEXT_CHECK: missing fixture review: %s", etsyOut)
			}
		})
	}
	for _, etsyMode := range []string{"opted_in", "outside_mcp"} {
		t.Run(etsyMode, func(t *testing.T) {
			t.Setenv("ETSY_MCP_ALLOW_BUYER_PII", "1")
			if etsyMode == "outside_mcp" {
				t.Setenv(cliutil.MCPChildEnvVar, "")
				t.Setenv("ETSY_MCP_ALLOW_BUYER_PII", "")
			}
			etsyOut, _, etsyErr := shopxRun(t, etsyFixture, "reviews", "digest", "--text")
			s2Must(t, etsyErr)
			if !strings.Contains(etsyOut, "linen apron arrived torn") {
				t.Fatalf("REVIEWS_DIGEST_TEXT_CHECK: missing permitted review text: %s", etsyOut)
			}
		})
	}
}

// TestEtsyReviewsDigestTextRefusedUnderMCP is the single hazard probe for the
// `reviews digest --text` refusal (no subtests, so it can be deselected alone).
func TestEtsyReviewsDigestTextRefusedUnderMCP(t *testing.T) {
	etsyFixture := shopxNewFixture(t)
	etsyFixture.put(shopxReviewTypes[0], map[string]any{
		"listing_id": 1000000001, "rating": 2,
		"created_timestamp": shopxAgo(shopxDay),
		"review":            "linen apron arrived torn",
	})
	t.Setenv(cliutil.MCPChildEnvVar, "1")
	t.Setenv("ETSY_MCP_ALLOW_BUYER_PII", "")
	etsyOut, _, etsyErr := shopxRun(t, etsyFixture, "reviews", "digest", "--text")
	if etsyErr == nil || strings.Contains(etsyOut, "arrived torn") {
		t.Fatalf("PROBE_REVIEWS_DIGEST_TEXT_LEAK: err=%v out=%s", etsyErr, etsyOut)
	}
}
