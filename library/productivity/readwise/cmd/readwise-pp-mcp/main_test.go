package main

import (
	"github.com/mvanhorn/printing-press-library/library/productivity/readwise/internal/cliutil/testenv"
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	_ = os.Unsetenv("READWISE_API_TOKEN")
	os.Exit(testenv.RunSandboxed(m))
}
