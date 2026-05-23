// Copyright 2026 dstevens. Licensed under Apache-2.0. See LICENSE.
package client

import (
	"context"
	"encoding/json"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/cmux/internal/cmuxclient"
)

// cmuxDispatch is the bridge from the generated HTTP-style do() into the
// cmuxclient subprocess shim. It exists in its own file so regen does not
// clobber the surgical do() edit in client.go.
func cmuxDispatch(method, path string, params map[string]string) (json.RawMessage, error) {
	return cmuxclient.Dispatch(context.Background(), method, path, params)
}
