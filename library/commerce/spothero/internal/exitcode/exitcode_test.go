package exitcode

import (
	"fmt"
	"testing"
)

func TestConstructorsCarryDocumentedCodes(t *testing.T) {
	for want, err := range map[int]error{
		Usage: Usagef("u"), NotFound: NotFoundf("n"), Auth: Authf("a"), API: APIf("x"), Transient: Transientf("t"),
	} {
		var ex *Error
		if !As(err, &ex) || ex.Code != want {
			t.Errorf("want code %d, got %v", want, err)
		}
	}
}

func TestAsFindsWrappedError(t *testing.T) {
	var ex *Error
	if !As(fmt.Errorf("wrapped: %w", Authf("no")), &ex) || ex.Code != Auth {
		t.Fatalf("not found through wrap: %v", ex)
	}
}
