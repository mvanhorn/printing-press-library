package cli

import "fmt"

const unsupportedChatCompletionsStreamError = "stream:true is not supported by this command: the API returns server-sent events that this CLI cannot decode into JSON/table output; omit stream or set stream:false for a normal JSON response"

func guardNonSSEChatCompletionsStream(body any, dryRun bool) error {
	bodyMap, ok := body.(map[string]any)
	if !ok {
		return fmt.Errorf("request body must be a JSON object to enforce stream safety, got %T", body)
	}
	streamValue, ok := bodyMap["stream"]
	if !ok || streamValue == nil {
		bodyMap["stream"] = false
		return nil
	}
	stream, ok := streamValue.(bool)
	if !ok {
		return fmt.Errorf("stream must be a JSON boolean or null, got JSON %T", streamValue)
	}
	if stream && !dryRun {
		return fmt.Errorf(unsupportedChatCompletionsStreamError)
	}
	return nil
}
