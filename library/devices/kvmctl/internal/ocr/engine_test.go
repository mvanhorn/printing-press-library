package ocr

import (
	"errors"
	"os/exec"
	"testing"
)

func TestCommandEngineFromEnvironmentRequiresExplicitCommand(t *testing.T) {
	t.Setenv("KVMCTL_OCR_COMMAND", "")
	_, err := CommandEngineFromEnvironment()
	if !errors.Is(err, ErrOCRUnavailable) {
		t.Fatalf("error = %v, want OCR unavailable", err)
	}
}

func TestCommandEngineRecognizesTesseractTSVFromStandardInput(t *testing.T) {
	tesseract, err := exec.LookPath("tesseract")
	if err != nil {
		t.Skip("tesseract is not installed")
	}
	t.Setenv("KVMCTL_OCR_COMMAND", tesseract)
	t.Setenv("KVMCTL_OCR_PROTOCOL", "")
	engine, err := CommandEngineFromEnvironment()
	if err != nil {
		t.Fatal(err)
	}

	image := makePNG(t, 31, 17)
	width, height, words, err := engine.Recognize(image)
	if err != nil {
		t.Fatal(err)
	}
	if width != 31 || height != 17 {
		t.Fatalf("dimensions = %dx%d, want 31x17", width, height)
	}
	if words == nil {
		t.Fatal("words must be a non-nil empty slice when no text is recognized")
	}
}

func TestParseJSONResponseRejectsMissingWordConfidence(t *testing.T) {
	_, _, _, err := parseJSONResponse([]byte(`{"width":10,"height":10,"words":[{"text":"ok","x":1,"y":2,"width":3,"height":4}]}`))
	if !errors.Is(err, ErrOCRFailed) {
		t.Fatalf("error = %v, want OCR failure", err)
	}
}
