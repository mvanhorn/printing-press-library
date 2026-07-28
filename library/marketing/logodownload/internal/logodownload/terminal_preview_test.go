package logodownload

import (
	"context"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRenderTerminalPreviewRendersBrailleRows(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/logo.png" {
			t.Fatalf("unexpected request: %s", r.URL.String())
		}

		img := image.NewRGBA(image.Rect(0, 0, 24, 12))
		for y := 0; y < 12; y++ {
			for x := 0; x < 24; x++ {
				img.Set(x, y, color.RGBA{R: 255, G: 255, B: 255, A: 255})
			}
		}
		for y := 3; y < 9; y++ {
			for x := 4; x < 20; x++ {
				img.Set(x, y, color.RGBA{A: 255})
			}
		}

		w.Header().Set("Content-Type", "image/png")
		if err := png.Encode(w, img); err != nil {
			t.Fatalf("png encode failed: %v", err)
		}
	}))
	defer server.Close()

	output := RenderTerminalPreview(context.Background(), server.Client(), []LogoResult{{
		Title:    "Test Logo",
		ImageURL: server.URL + "/logo.png",
	}}, TerminalPreviewOptions{Height: 4, Width: 10, Limit: 1})

	if !strings.Contains(output, "1. Test L") {
		t.Fatalf("expected title in preview output, got:\n%s", output)
	}
	if !containsBraille(output) {
		t.Fatalf("expected braille preview output, got:\n%s", output)
	}
}

func containsBraille(value string) bool {
	for _, char := range value {
		if char >= 0x2800 && char <= 0x28ff && char != 0x2800 {
			return true
		}
	}
	return false
}
