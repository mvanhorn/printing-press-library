package cli

import "github.com/mvanhorn/printing-press-library/library/health/suppco/internal/provider"

func newSuppCoProvider(flags *rootFlags) (*provider.Service, error) {
	c, err := flags.newClient()
	if err != nil {
		return nil, err
	}
	return provider.New(c), nil
}
