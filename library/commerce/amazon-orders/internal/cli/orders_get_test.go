package cli

import (
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/commerce/amazon-orders/internal/parser"
)

func TestValidateOrderDetailMatchesRequestedID(t *testing.T) {
	const requested = "702-5010515-8774615"
	if err := validateOrderDetailMatchesRequestedID(requested, &parser.OrderDetail{OrderID: requested}); err != nil {
		t.Fatalf("matching order ID returned error: %v", err)
	}
}

func TestValidatePageContainsRequestedOrderID(t *testing.T) {
	const orderID = "702-5010515-8774615"
	if err := validatePageContainsRequestedOrderID(orderID, []byte(`<html>Order # 702-5010515-8774615</html>`)); err != nil {
		t.Fatalf("validatePageContainsRequestedOrderID() unexpected error: %v", err)
	}
	if err := validatePageContainsRequestedOrderID(orderID, []byte(`<html>Order # 111-1111111-1111111</html>`)); err == nil {
		t.Fatal("validatePageContainsRequestedOrderID() = nil, want mismatch error")
	}
}

func TestValidateOrderDetailRejectsMismatchedID(t *testing.T) {
	err := validateOrderDetailMatchesRequestedID("702-5010515-8774615", &parser.OrderDetail{OrderID: "144-5062705-8396341"})
	if err == nil {
		t.Fatal("mismatched order ID returned nil error")
	}
	if !strings.Contains(err.Error(), "did not match requested order") {
		t.Fatalf("error = %v, want requested-order mismatch", err)
	}
}

func TestValidateOrderDetailRejectsMissingID(t *testing.T) {
	err := validateOrderDetailMatchesRequestedID("702-5010515-8774615", &parser.OrderDetail{})
	if err == nil {
		t.Fatal("missing order ID returned nil error")
	}
	if !strings.Contains(err.Error(), "did not include an order ID") {
		t.Fatalf("error = %v, want missing-order-ID message", err)
	}
}
