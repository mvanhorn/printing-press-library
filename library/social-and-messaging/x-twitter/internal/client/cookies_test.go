package client

import "testing"

func TestCookieAuthArticleUserIDFallsBackToTWID(t *testing.T) {
	c := &cookieAuth{TWID: `"u%3D1234567890"`}
	if got := c.ArticleUserID(); got != "1234567890" {
		t.Fatalf("ArticleUserID() = %q, want 1234567890", got)
	}
	c.UserID = "999"
	if got := c.ArticleUserID(); got != "999" {
		t.Fatalf("ArticleUserID() should prefer user_id, got %q", got)
	}
}
