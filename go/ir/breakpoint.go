package ir

import "encoding/json"

const (
	CacheControlTypeLossDetail = "cache_control.type is not represented by prompt_cache_breakpoint"
	CacheControlTTLLossDetail  = "cache_control.ttl has no equivalent prompt cache ttl"
	BreakpointModeLossDetail   = "prompt_cache_breakpoint.mode is not represented by cache_control"
	BreakpointTTLLossDetail    = "prompt_cache_breakpoint.ttl has no equivalent cache_control ttl"
	ExplicitBreakpointWire     = `{"mode":"explicit"}`
	EphemeralCacheControlWire  = `{"type":"ephemeral"}`
)

// OpenAIBreakpointWire returns the prompt_cache_breakpoint object to emit.
// An existing OpenAI breakpoint wins. An Anthropic cache_control on the same
// host becomes an explicit breakpoint without a ttl.
func OpenAIBreakpointWire(native []NativeAnnotation) json.RawMessage {
	for _, ann := range native {
		if ann.Source == NativeSourceOpenAI && ann.Kind == NativeKindPromptCacheBreakpoint {
			return json.RawMessage(ann.Value)
		}
	}
	for _, ann := range native {
		if ann.Source == NativeSourceAnthropic && ann.Kind == NativeKindCacheControl {
			return json.RawMessage(ExplicitBreakpointWire)
		}
	}
	return nil
}

func HasOpenAIBreakpoint(native []NativeAnnotation) bool {
	return len(OpenAIBreakpointWire(native)) > 0
}

func CacheControlTranslationLosses(value, path string) []Loss {
	losses := []Loss{{
		Path: path, Field: "type", Reason: LossUnmappedField, Detail: CacheControlTypeLossDetail,
	}}
	var obj map[string]json.RawMessage
	if json.Unmarshal([]byte(value), &obj) == nil {
		if _, ok := obj["ttl"]; ok {
			losses = append(losses, Loss{
				Path: path, Field: "ttl", Reason: LossUnmappedField, Detail: CacheControlTTLLossDetail,
			})
		}
	}
	return losses
}

func BreakpointTranslationLosses(value, path string) []Loss {
	var obj map[string]json.RawMessage
	if json.Unmarshal([]byte(value), &obj) != nil {
		return nil
	}
	var losses []Loss
	if _, ok := obj["mode"]; ok {
		losses = append(losses, Loss{
			Path: path, Field: "mode", Reason: LossUnmappedField, Detail: BreakpointModeLossDetail,
		})
	}
	if _, ok := obj["ttl"]; ok {
		losses = append(losses, Loss{
			Path: path, Field: "ttl", Reason: LossUnmappedField, Detail: BreakpointTTLLossDetail,
		})
	}
	return losses
}
