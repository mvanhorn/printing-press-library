package cli

import "testing"

func TestReplacePathParamEscapesReservedCharacters(t *testing.T) {
	got := replacePathParam("/subscriptions/by_email/{email}", "email", "user+tag@example.com")
	want := "/subscriptions/by_email/user%2Btag%40example.com"
	if got != want {
		t.Fatalf("replacePathParam() = %q, want %q", got, want)
	}
}

func TestReplacePathParamEscapesPathSeparators(t *testing.T) {
	got := replacePathParam("/resources/{id}", "id", "a/b c")
	want := "/resources/a%2Fb%20c"
	if got != want {
		t.Fatalf("replacePathParam() = %q, want %q", got, want)
	}
}
