package client

import "sync"

var globalQueryParams = map[string]string{}

var globalQueryParamsMu sync.RWMutex

// SetGlobalQueryParam sets a query parameter added to every request. An empty
// value removes the parameter.
func SetGlobalQueryParam(key, value string) {
	globalQueryParamsMu.Lock()
	defer globalQueryParamsMu.Unlock()
	if value == "" {
		delete(globalQueryParams, key)
		return
	}
	globalQueryParams[key] = value
}

func globalQueryParamsSnapshot() map[string]string {
	globalQueryParamsMu.RLock()
	defer globalQueryParamsMu.RUnlock()
	snapshot := make(map[string]string, len(globalQueryParams))
	for key, value := range globalQueryParams {
		snapshot[key] = value
	}
	return snapshot
}
