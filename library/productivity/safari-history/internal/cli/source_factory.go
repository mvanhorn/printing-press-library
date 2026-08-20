package cli

import (
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/productivity/safari-history/internal/source"
	safariSource "github.com/mvanhorn/printing-press-library/library/productivity/safari-history/internal/source/safari"
)

func resolveSource(name string) (source.Source, error) {
	switch name {
	case "", "safari":
		return safariSource.New(), nil
	default:
		return nil, fmt.Errorf("unsupported source: %s", name)
	}
}
