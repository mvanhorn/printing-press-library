package vrbo

import "errors"

// ErrDisabled is returned by every VRBO entry point. The VRBO source layer is
// kept in the tree for future re-enablement, but every CLI entry point checks
// this sentinel before any HTTP call. The previous fake-data fallback that
// stamped queried cities onto hardcoded Tahoe listings has been removed.
var ErrDisabled = errors.New("vrbo is temporarily disabled — pending Akamai workaround")

// IsDisabled reports whether err is ErrDisabled.
func IsDisabled(err error) bool { return errors.Is(err, ErrDisabled) }
