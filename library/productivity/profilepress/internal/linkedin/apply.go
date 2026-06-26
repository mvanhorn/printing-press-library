package linkedin

import "fmt"

// ApplyAdapter is the seam where a future LinkedIn browser writer plugs in.
type ApplyAdapter interface {
	Apply(section, value string) error
}

type DryRunAdapter struct{}

func (DryRunAdapter) Apply(section, value string) error { return nil }

type NotImplementedAdapter struct{}

func (NotImplementedAdapter) Apply(section, value string) error {
	return fmt.Errorf("live LinkedIn apply is not implemented; rerun with --dry-run or a tested adapter")
}
