package cli

import (
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/wordpress/internal/wpupload"
)

func TestClassifyWordPressUploadError(t *testing.T) {
	tests := []struct {
		name    string
		err     error
		code    int
		message string
	}{
		{name: "empty", err: wpupload.ErrEmptyFile, code: 2, message: "file was empty"},
		{name: "hash mismatch", err: &wpupload.APIError{StatusCode: 412, Code: "rest_upload_hash_mismatch"}, code: 5, message: "retry"},
		{name: "quota verbatim", err: &wpupload.APIError{StatusCode: 500, Code: "rest_upload_limited_space", Message: "Storage quota exhausted"}, code: 5, message: "Storage quota exhausted"},
		{name: "capability", err: &wpupload.APIError{StatusCode: 403, Code: "rest_cannot_create"}, code: 4, message: "upload_files"},
		{name: "unknown unauthorized", err: &wpupload.APIError{StatusCode: http.StatusUnauthorized, Message: "no"}, code: 4, message: "no"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := classifyWordPressUploadError(tt.err)
			if err == nil {
				t.Fatal("expected error")
			}
			if got := ExitCode(err); got != tt.code {
				t.Fatalf("exit code = %d, want %d", got, tt.code)
			}
			if !strings.Contains(err.Error(), tt.message) {
				t.Fatalf("error = %q, want substring %q", err, tt.message)
			}
		})
	}

	if got := classifyWordPressUploadError(errors.New("transport")); ExitCode(got) != 5 {
		t.Fatalf("transport exit code = %d", ExitCode(got))
	}
}
