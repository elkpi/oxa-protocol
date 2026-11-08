package vectest

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/elkpi/oxa-protocol/go/v3/anthropic/messages"
	"github.com/elkpi/oxa-protocol/go/v3/ir"
	"github.com/elkpi/oxa-protocol/go/v3/openai/chatcompletions"
	"github.com/elkpi/oxa-protocol/go/v3/openai/responses"
	jev "github.com/elkpi/oxa-protocol/go/v3/systemone/jev"
)

// fakeCrossConverter is a programmable Converter for harness tests.
type fakeCrossConverter struct {
	face         string
	decodedReq   *ir.Request
	encodedReq   []byte
	decodedResp  *ir.Response
	encodedResp  []byte
	decodeLosses []ir.Loss
	encodeLosses []ir.Loss
	reqCalls     int
	respCalls    int
}

func (f *fakeCrossConverter) Face() string { return f.face }

func (f *fakeCrossConverter) DecodeRequestWire(json.RawMessage) (*ir.Request, []ir.Loss, error) {
	f.reqCalls++
	return f.decodedReq, f.decodeLosses, nil
}

func (f *fakeCrossConverter) DecodeResponseWire(json.RawMessage) (*ir.Response, []ir.Loss, error) {
	f.respCalls++
	return f.decodedResp, f.decodeLosses, nil
}

func (f *fakeCrossConverter) EncodeRequestIR(*ir.Request) (json.RawMessage, []ir.Loss, error) {
	return f.encodedReq, f.encodeLosses, nil
}

func (f *fakeCrossConverter) EncodeResponseIR(*ir.Response) (json.RawMessage, []ir.Loss, error) {
	return f.encodedResp, f.encodeLosses, nil
}

func writeCrossVector(t *testing.T, root, fileName string, v Vector) {
	t.Helper()
	dir := filepath.Join(root, "vectors", "cross", "nonstream")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("make cross vector directory: %v", err)
	}
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal vector: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, fileName), raw, 0o644); err != nil {
		t.Fatalf("write vector: %v", err)
	}
}

func crossTestVector(name, source, target string, request bool) Vector {
	v := Vector{
		Name:        "cross.nonstream." + name,
		Mode:        "nonstream",
		Conversion:  "protocol-to-protocol",
		Input:       json.RawMessage(`{"kind":"input"}`),
		ExpectedOut: json.RawMessage(`{"kind":"output"}`),
	}
	v.Source.Protocol = source
	v.Target.Protocol = target
	// Matches the fake converters' decode+encode losses so the harness's
	// concatenation and unordered-set comparison are both exercised.
	v.ExpectedLosses = []Loss{
		{Path: "in", Field: "f", Reason: "unmapped-field"},
		{Path: "params", Field: "g", Reason: "unmapped-field"},
	}
	if request {
		v.Tags = []string{"request"}
	} else {
		v.Tags = []string{"response"}
	}
	return v
}

func TestRunCrossComposesMatchedVectors(t *testing.T) {
	root := setupStreamVectorRepo(t)
	alpha := &fakeCrossConverter{
		face:         "alpha",
		decodedReq:   &ir.Request{Model: "m"},
		decodedResp:  &ir.Response{Model: "m"},
		decodeLosses: []ir.Loss{{Path: "in", Field: "f", Reason: ir.LossUnmappedField}},
	}
	beta := &fakeCrossConverter{
		face:         "beta",
		encodedReq:   []byte(`{"kind":"output"}`),
		encodedResp:  []byte(`{"kind":"output"}`),
		encodeLosses: []ir.Loss{{Path: "params", Field: "g", Reason: ir.LossUnmappedField}},
	}
	writeCrossVector(t, root, "alpha-to-beta-request.json", crossTestVector("alpha-to-beta-request", "alpha", "beta", true))
	writeCrossVector(t, root, "alpha-to-beta-response.json", crossTestVector("alpha-to-beta-response", "alpha", "beta", false))
	// Mismatched pair must be skipped by RunCross(t, alpha, beta).
	writeCrossVector(t, root, "beta-to-alpha-request.json", crossTestVector("beta-to-alpha-request", "beta", "alpha", true))

	RunCross(t, alpha, beta)

	if alpha.reqCalls != 1 || alpha.respCalls != 1 {
		t.Fatalf("alpha decode calls = (%d, %d), want (1, 1)", alpha.reqCalls, alpha.respCalls)
	}
}

func TestCrossVectorsForFiltersByPair(t *testing.T) {
	vectors := []Vector{
		crossTestVector("a-to-b-request", "alpha", "beta", true),
		crossTestVector("b-to-a-request", "beta", "alpha", true),
		crossTestVector("a-to-c-request", "alpha", "gamma", true),
	}
	alpha := &fakeCrossConverter{face: "alpha"}
	beta := &fakeCrossConverter{face: "beta"}
	matched := crossVectorsFor(alpha, beta, vectors)
	if len(matched) != 1 || matched[0].Name != "cross.nonstream.a-to-b-request" {
		t.Fatalf("crossVectorsFor() = %+v, want exactly a-to-b-request", matched)
	}
	if got := crossVectorsFor(alpha, &fakeCrossConverter{face: "delta"}, vectors); len(got) != 0 {
		t.Fatalf("crossVectorsFor() with unknown target = %+v, want empty", got)
	}
}

// crossChatCompletions binds the Chat Completions face to the cross runner.
type crossChatCompletions struct{}

func (crossChatCompletions) Face() string { return "chatcompletions" }

func (crossChatCompletions) DecodeRequestWire(w json.RawMessage) (*ir.Request, []ir.Loss, error) {
	return chatcompletions.DecodeRequestJSON(w)
}

func (crossChatCompletions) DecodeResponseWire(w json.RawMessage) (*ir.Response, []ir.Loss, error) {
	return chatcompletions.DecodeResponseJSON(w)
}

func (crossChatCompletions) EncodeRequestIR(req *ir.Request) (json.RawMessage, []ir.Loss, error) {
	out, losses, err := chatcompletions.EncodeRequest(req)
	if err != nil {
		return nil, nil, err
	}
	raw, err := json.Marshal(out)
	return raw, losses, err
}

func (crossChatCompletions) EncodeResponseIR(resp *ir.Response) (json.RawMessage, []ir.Loss, error) {
	out, losses, err := chatcompletions.EncodeResponse(resp)
	if err != nil {
		return nil, nil, err
	}
	raw, err := json.Marshal(out)
	return raw, losses, err
}

// crossResponses binds the Responses face to the cross runner.
type crossResponses struct{}

func (crossResponses) Face() string { return "responses" }

func (crossResponses) DecodeRequestWire(w json.RawMessage) (*ir.Request, []ir.Loss, error) {
	return responses.DecodeRequestJSON(w)
}

func (crossResponses) DecodeResponseWire(w json.RawMessage) (*ir.Response, []ir.Loss, error) {
	return responses.DecodeResponseJSON(w)
}

func (crossResponses) EncodeRequestIR(req *ir.Request) (json.RawMessage, []ir.Loss, error) {
	out, losses, err := responses.EncodeRequest(req)
	if err != nil {
		return nil, nil, err
	}
	raw, err := json.Marshal(out)
	return raw, losses, err
}

func (crossResponses) EncodeResponseIR(resp *ir.Response) (json.RawMessage, []ir.Loss, error) {
	out, losses, err := responses.EncodeResponse(resp)
	if err != nil {
		return nil, nil, err
	}
	raw, err := json.Marshal(out)
	return raw, losses, err
}

// crossAnthropic binds the Anthropic Messages face to the cross runner.
type crossAnthropic struct{}

func (crossAnthropic) Face() string { return "anthropic" }

func (crossAnthropic) DecodeRequestWire(w json.RawMessage) (*ir.Request, []ir.Loss, error) {
	return messages.DecodeRequestJSON(w)
}

func (crossAnthropic) DecodeResponseWire(w json.RawMessage) (*ir.Response, []ir.Loss, error) {
	return messages.DecodeResponseJSON(w)
}

func (crossAnthropic) EncodeRequestIR(req *ir.Request) (json.RawMessage, []ir.Loss, error) {
	out, losses, err := messages.EncodeRequest(req)
	if err != nil {
		return nil, nil, err
	}
	raw, err := json.Marshal(out)
	return raw, losses, err
}

func (crossAnthropic) EncodeResponseIR(resp *ir.Response) (json.RawMessage, []ir.Loss, error) {
	out, losses, err := messages.EncodeResponse(resp)
	if err != nil {
		return nil, nil, err
	}
	raw, err := json.Marshal(out)
	return raw, losses, err
}

// crossJev binds the SystemOne Jev face to the cross runner.
type crossJev struct{}

func (crossJev) Face() string { return "jev" }

func (crossJev) DecodeRequestWire(w json.RawMessage) (*ir.Request, []ir.Loss, error) {
	return jev.DecodeRequestJSON(w)
}

func (crossJev) DecodeResponseWire(w json.RawMessage) (*ir.Response, []ir.Loss, error) {
	return jev.DecodeResponseJSON(w)
}

func (crossJev) EncodeRequestIR(req *ir.Request) (json.RawMessage, []ir.Loss, error) {
	out, losses, err := jev.EncodeRequest(req)
	if err != nil {
		return nil, nil, err
	}
	raw, err := json.Marshal(out)
	return raw, losses, err
}

func (crossJev) EncodeResponseIR(resp *ir.Response) (json.RawMessage, []ir.Loss, error) {
	out, losses, err := jev.EncodeResponse(resp)
	if err != nil {
		return nil, nil, err
	}
	raw, err := json.Marshal(out)
	return raw, losses, err
}

// fakeCrossStreamConverter is a programmable StreamConverter for harness
// tests: each native event decodes to one fixed IR text delta carrying the
// native position, and each IR event encodes to one fixed native object.
type fakeCrossStreamConverter struct {
	face        string
	decodeLoss  ir.Loss
	encodeLoss  ir.Loss
	decodeCalls int
	applied     []ir.Event
	encodeRaw   string
}

func (f *fakeCrossStreamConverter) Face() string { return f.face }

func (f *fakeCrossStreamConverter) DecodeNativeEvent(json.RawMessage) ([]ir.Event, error) {
	f.decodeCalls++
	return []ir.Event{ir.ContentBlockDelta{
		Index: 0,
		Delta: ir.TextDelta{Text: fmt.Sprintf("delta-%d", f.decodeCalls)},
	}}, nil
}

func (f *fakeCrossStreamConverter) FlushDecoder() ([]ir.Event, error) {
	return []ir.Event{ir.MessageDone{}}, nil
}

func (f *fakeCrossStreamConverter) DecoderLosses() []ir.Loss {
	if f.decodeLoss.Reason == "" {
		return nil
	}
	return []ir.Loss{f.decodeLoss}
}

func (f *fakeCrossStreamConverter) ApplyIREvent(ev ir.Event) ([]json.RawMessage, []ir.Loss, error) {
	f.applied = append(f.applied, ev)
	raw := json.RawMessage(f.encodeRaw)
	var losses []ir.Loss
	if f.encodeLoss.Reason != "" {
		losses = []ir.Loss{f.encodeLoss}
	}
	return []json.RawMessage{raw}, losses, nil
}

func writeCrossStreamVector(t *testing.T, root, fileName string, v Vector) {
	t.Helper()
	dir := filepath.Join(root, "vectors", "cross", "stream")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("make cross stream vector directory: %v", err)
	}
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal vector: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, fileName), raw, 0o644); err != nil {
		t.Fatalf("write vector: %v", err)
	}
}

func crossStreamTestVector(name, source, target string) Vector {
	// The fake source reports one decode loss; the fake target reports one
	// loss per applied IR event (3 events), so the unordered-set comparison
	// exercises the decode+encode concatenation.
	return Vector{
		Name:        "cross.stream." + name,
		Mode:        "stream",
		Conversion:  "protocol-to-protocol",
		Source:      Endpoint{Protocol: source},
		Target:      Endpoint{Protocol: target},
		Input:       json.RawMessage(`{"events":[{"a":1},{"a":2}]}`),
		ExpectedOut: json.RawMessage(`{"events":[{"n":1},{"n":1},{"n":1}]}`),
		ExpectedLosses: []Loss{
			{Path: "src", Field: "f", Reason: "unmapped-field"},
			{Path: "tgt", Field: "g", Reason: "unmapped-field"},
			{Path: "tgt", Field: "g", Reason: "unmapped-field"},
			{Path: "tgt", Field: "g", Reason: "unmapped-field"},
		},
		Tags: []string{"request"},
	}
}

// TestRunCrossStreamComposesMatchedVectors verifies the cross-stream
// composition: source native events decode to IR events, the IR event
// sequence encodes into target native events, and the losses concatenate as
// an unordered set.
func TestRunCrossStreamComposesMatchedVectors(t *testing.T) {
	root := setupStreamVectorRepo(t)
	source := &fakeCrossStreamConverter{face: "alpha", decodeLoss: ir.Loss{Path: "src", Field: "f", Reason: ir.LossUnmappedField}, encodeRaw: `{"n":1}`}
	target := &fakeCrossStreamConverter{face: "beta", encodeLoss: ir.Loss{Path: "tgt", Field: "g", Reason: ir.LossUnmappedField}, encodeRaw: `{"n":1}`}
	writeCrossStreamVector(t, root, "alpha-to-beta-text.json", crossStreamTestVector("alpha-to-beta-text", "alpha", "beta"))
	// Mismatched pair must be skipped.
	writeCrossStreamVector(t, root, "beta-to-alpha-text.json", crossStreamTestVector("beta-to-alpha-text", "beta", "alpha"))

	RunCrossStream(t, source, target)

	// Two native events + the flushed MessageDone = 3 applied IR events.
	if len(target.applied) != 3 {
		t.Fatalf("target applied %d IR events, want 3", len(target.applied))
	}
	if _, ok := target.applied[2].(ir.MessageDone); !ok {
		t.Fatalf("third applied event = %T, want ir.MessageDone", target.applied[2])
	}
}

// TestCrossVectors runs the six ordered face pairs over the
// cross/nonstream vectors through the real implementations.
func TestCrossVectors(t *testing.T) {
	faces := map[string]Converter{
		"chatcompletions": crossChatCompletions{},
		"responses":       crossResponses{},
		"anthropic":       crossAnthropic{},
		"jev":             crossJev{},
	}
	for _, pair := range [][2]string{
		{"chatcompletions", "responses"},
		{"chatcompletions", "anthropic"},
		{"chatcompletions", "jev"},
		{"responses", "chatcompletions"},
		{"responses", "anthropic"},
		{"responses", "responses"},
		{"anthropic", "chatcompletions"},
		{"anthropic", "responses"},
		{"anthropic", "anthropic"},
		{"jev", "chatcompletions"},
		{"jev", "responses"},
		{"jev", "anthropic"},
	} {
		t.Run(pair[0]+"-to-"+pair[1], func(t *testing.T) {
			RunCross(t, faces[pair[0]], faces[pair[1]])
		})
	}
}

// crossStreamAdapter binds one face's streaming decoder and encoder to the
// StreamConverter surface through closures, since the three faces' encoder
// Apply methods return different concrete types.
type crossStreamAdapter struct {
	face   string
	reset  func()
	decode func(json.RawMessage) ([]ir.Event, error)
	flush  func() ([]ir.Event, error)
	losses func() []ir.Loss
	apply  func(ir.Event) ([]json.RawMessage, []ir.Loss, error)
}

func (a *crossStreamAdapter) Face() string       { return a.face }
func (a *crossStreamAdapter) ResetStreamVector() { a.reset() }
func (a *crossStreamAdapter) DecodeNativeEvent(raw json.RawMessage) ([]ir.Event, error) {
	return a.decode(raw)
}
func (a *crossStreamAdapter) FlushDecoder() ([]ir.Event, error) { return a.flush() }
func (a *crossStreamAdapter) DecoderLosses() []ir.Loss          { return a.losses() }
func (a *crossStreamAdapter) ApplyIREvent(ev ir.Event) ([]json.RawMessage, []ir.Loss, error) {
	return a.apply(ev)
}

// marshalNativeEvents converts whatever a face encoder returns into the raw
// JSON events the stream runners compare.
func marshalNativeEvents[T any](events []T) ([]json.RawMessage, error) {
	out := make([]json.RawMessage, 0, len(events))
	for _, event := range events {
		raw, err := json.Marshal(event)
		if err != nil {
			return nil, err
		}
		out = append(out, raw)
	}
	return out, nil
}

func newCrossStreamAdapterCC() *crossStreamAdapter {
	dec := chatcompletions.NewStreamDecoder()
	enc := chatcompletions.NewStreamEncoder()
	return &crossStreamAdapter{
		face: "chatcompletions",
		reset: func() {
			dec = chatcompletions.NewStreamDecoder()
			enc = chatcompletions.NewStreamEncoder()

		},
		decode: func(raw json.RawMessage) ([]ir.Event, error) { return dec.FeedJSON(raw) },
		flush:  func() ([]ir.Event, error) { return dec.Flush() },
		losses: func() []ir.Loss { return dec.Losses() },
		apply: func(ev ir.Event) ([]json.RawMessage, []ir.Loss, error) {
			chunks, losses, err := enc.Apply(ev)
			if err != nil {
				return nil, nil, err
			}
			out, err := marshalNativeEvents(chunks)
			return out, losses, err
		},
	}
}

func newCrossStreamAdapterRE() *crossStreamAdapter {
	dec := responses.NewStreamDecoder()
	enc := responses.NewStreamEncoder()
	return &crossStreamAdapter{
		face: "responses",
		reset: func() {
			dec = responses.NewStreamDecoder()
			enc = responses.NewStreamEncoder()

		},
		decode: func(raw json.RawMessage) ([]ir.Event, error) { return dec.FeedJSON(raw) },
		flush:  func() ([]ir.Event, error) { return dec.Flush() },
		losses: func() []ir.Loss { return dec.Losses() },
		apply: func(ev ir.Event) ([]json.RawMessage, []ir.Loss, error) {
			events, losses, err := enc.Apply(ev)
			if err != nil {
				return nil, nil, err
			}
			out, err := marshalNativeEvents(events)
			return out, losses, err
		},
	}
}

func newCrossStreamAdapterAN() *crossStreamAdapter {
	dec := messages.NewStreamDecoder()
	enc := messages.NewStreamEncoder()
	return &crossStreamAdapter{
		face: "anthropic",
		reset: func() {
			dec = messages.NewStreamDecoder()
			enc = messages.NewStreamEncoder()

		},
		decode: func(raw json.RawMessage) ([]ir.Event, error) { return dec.FeedJSON(raw) },
		flush:  func() ([]ir.Event, error) { return dec.Flush() },
		losses: func() []ir.Loss { return dec.Losses() },
		apply: func(ev ir.Event) ([]json.RawMessage, []ir.Loss, error) {
			events, losses, err := enc.Apply(ev)
			if err != nil {
				return nil, nil, err
			}
			out, err := marshalNativeEvents(events)
			return out, losses, err
		},
	}
}

// TestCrossVectorsStream runs the nine ordered pairs over the cross/stream
// vectors through the real streaming implementations. Jev has no streaming
// face (spec/17) and is excluded.
func TestCrossVectorsStream(t *testing.T) {
	faces := map[string]*crossStreamAdapter{
		"chatcompletions": newCrossStreamAdapterCC(),
		"responses":       newCrossStreamAdapterRE(),
		"anthropic":       newCrossStreamAdapterAN(),
	}
	for _, pair := range [][2]string{
		{"chatcompletions", "responses"},
		{"chatcompletions", "anthropic"},
		{"responses", "chatcompletions"},
		{"responses", "anthropic"},
		{"responses", "responses"},
		{"chatcompletions", "chatcompletions"},
		{"anthropic", "chatcompletions"},
		{"anthropic", "responses"},
		{"anthropic", "anthropic"},
	} {
		t.Run(pair[0]+"-to-"+pair[1], func(t *testing.T) {
			RunCrossStream(t, faces[pair[0]], faces[pair[1]])
		})
	}
}
