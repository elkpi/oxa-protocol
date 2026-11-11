package responses

import (
	"encoding/json"
	"fmt"

	"github.com/elkpi/oxa-protocol/go/v3/ir"
)

const contextManagementLossDetail = "Responses context_management has no equivalent on the target face"

func decodeRequestExtras(req *ir.Request, wire *Request) ([]ir.Loss, error) {
	var losses []ir.Loss
	if wire.TopLogprobs != nil {
		if *wire.TopLogprobs < 0 {
			return nil, fmt.Errorf("responses: top_logprobs must be a non-negative integer")
		}
		v := *wire.TopLogprobs
		req.Params.TopLogprobs = &v
	}
	if len(wire.ContextManagement) == 0 {
		return losses, nil
	}
	var kept []ContextManagementEntry
	for i, entry := range wire.ContextManagement {
		if entry.Type == "" {
			return nil, fmt.Errorf("responses: context_management[%d].type is required", i)
		}
		if entry.Type != "compaction" {
			losses = append(losses, loss(
				fmt.Sprintf("context_management[%d].type", i), "type", ir.LossUnmappedValue,
				fmt.Sprintf("unknown context_management type %q", entry.Type),
			))
			continue
		}
		if entry.CompactThreshold != nil && *entry.CompactThreshold < 1000 {
			return nil, fmt.Errorf("responses: context_management[%d].compact_threshold must be at least 1000", i)
		}
		kept = append(kept, entry)
	}
	if len(kept) == 0 {
		return losses, nil
	}
	raw, err := json.Marshal(kept)
	if err != nil {
		return nil, err
	}
	req.Native = append(req.Native, ir.NativeAnnotation{
		Source: ir.NativeSourceOpenAI,
		Kind:   ir.NativeKindContextManagement,
		Value:  string(raw),
		Omission: &ir.NativeOmission{
			Field:  "context_management",
			Reason: ir.LossUnmappedField,
			Detail: contextManagementLossDetail,
		},
	})
	return losses, nil
}

func encodeRequestExtras(out *Request, req *ir.Request) ([]ir.Loss, error) {
	var losses []ir.Loss
	sendTop := req.Params.TopLogprobs != nil
	if req.Params.Logprobs != nil {
		if *req.Params.Logprobs {
			if req.Params.TopLogprobs == nil {
				losses = append(losses, loss("params.logprobs", "logprobs", ir.LossUnmappedField, "Responses has no logprobs boolean; top_logprobs is the sampling switch"))
			}
		} else {
			sendTop = false
			losses = append(losses, loss("params.logprobs", "logprobs", ir.LossUnmappedField, "Responses has no logprobs boolean; explicit false is dropped"))
		}
	}
	if sendTop {
		v := *req.Params.TopLogprobs
		out.TopLogprobs = &v
	}
	for _, ann := range req.Native {
		if ann.Source == ir.NativeSourceOpenAI && ann.Kind == ir.NativeKindContextManagement {
			var entries []ContextManagementEntry
			if err := json.Unmarshal([]byte(ann.Value), &entries); err != nil {
				return nil, fmt.Errorf("responses: context_management: %w", err)
			}
			out.ContextManagement = entries
		}
	}
	return losses, nil
}
