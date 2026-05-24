// Copyright 2026 eric-jung. Licensed under Apache-2.0. See LICENSE.

package cli

import "encoding/json"

func mustMarshal(v any) json.RawMessage {
	data, _ := json.Marshal(v)
	return data
}
