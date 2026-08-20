package cli

import (
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/productivity/chrome-history/internal/source"
	chromeSource "github.com/mvanhorn/printing-press-library/library/productivity/chrome-history/internal/source/chrome"
)

func resolveSource(name string) (source.Source, error) {
	switch name {
	case "", "chrome":
		return chromeSource.New(), nil
	default:
		return nil, fmt.Errorf("unsupported source: %s", name)
	}
}
