package client

import "testing"

func TestParseMarketplacePhotoUploadResponse(t *testing.T) {
	t.Parallel()

	body := []byte(`for (;;);{"__ar":1,"payload":{"photoID":"12345","width":640,"height":480,"imageSrc":"https://example.invalid/tokenized"}}`)
	got, err := parseMarketplacePhotoUploadResponse(body)
	if err != nil {
		t.Fatalf("parseMarketplacePhotoUploadResponse returned error: %v", err)
	}
	if got.PhotoID != "12345" {
		t.Fatalf("PhotoID = %q, want 12345", got.PhotoID)
	}
	if got.Width != 640 || got.Height != 480 {
		t.Fatalf("dimensions = %dx%d, want 640x480", got.Width, got.Height)
	}
}

func TestParseMarketplacePhotoUploadResponseRequiresPhotoID(t *testing.T) {
	t.Parallel()

	if _, err := parseMarketplacePhotoUploadResponse([]byte(`{"payload":{}}`)); err == nil {
		t.Fatalf("expected missing photoID error")
	}
}

func TestSummarizeMarketplacePhotoUploadResponseCompactsErrors(t *testing.T) {
	t.Parallel()

	body := []byte(`{"error":1357001,"errorSummary":"Log in to continue","errorDescription":"Please log in to your account.","payload":{"__dialog":{"body":"Please log in to continue.","buttons":[{"href":"https://example.invalid/token"}]}}}`)
	got := summarizeMarketplacePhotoUploadResponse(body)
	want := `{"dialog_body":"Please log in to continue.","error":1357001,"errorDescription":"Please log in to your account.","errorSummary":"Log in to continue"}`
	if got != want {
		t.Fatalf("summary = %s, want %s", got, want)
	}
}
