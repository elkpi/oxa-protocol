package responses

import (
	"testing"

	"github.com/elkpi/oxa-protocol/go/v3/ir"
)

func TestCompactionItemRequestRoundTrip(t *testing.T) {
	wire := &Request{
		Model: "gpt-5.4",
		Input: Input{Items: []InputItem{
			{Type: ItemTypeMessage, Role: RoleUser, Content: "before"},
			{Type: ItemTypeCompaction, ID: "cmp_1", EncryptedContent: "cipher", CreatedBy: "system"},
			{Type: ItemTypeMessage, Role: RoleUser, Content: "after"},
		}},
	}
	req, losses, err := DecodeRequest(wire)
	if err != nil {
		t.Fatal(err)
	}
	if len(losses) != 0 {
		t.Fatalf("losses: %+v", losses)
	}
	if len(req.CompactionItems) != 1 || req.CompactionItems[0].AfterMessage != 0 || req.CompactionItems[0].CreatedBy != "system" {
		t.Fatalf("items: %+v", req.CompactionItems)
	}
	out, losses, err := EncodeRequest(req)
	if err != nil {
		t.Fatal(err)
	}
	if len(losses) != 0 {
		t.Fatalf("encode losses: %+v", losses)
	}
	if len(out.Input.Items) != 3 || out.Input.Items[1].Type != ItemTypeCompaction || out.Input.Items[1].EncryptedContent != "cipher" {
		t.Fatalf("wire: %+v", out.Input.Items)
	}
}

func TestCompactionItemResponseRoundTrip(t *testing.T) {
	wire := &Response{
		ID: "resp_1", Object: ObjectResponse, Status: StatusCompleted, Model: "gpt-5.4",
		Output: []OutputItem{
			{Type: ItemTypeCompaction, ID: "cmp_1", EncryptedContent: "cipher"},
			{Type: ItemTypeMessage, ID: "msg_1", Status: StatusCompleted, Role: RoleAssistant, Content: []OutputContent{{Type: PartTypeOutputText, Text: "hi"}}},
		},
	}
	resp, losses, err := DecodeResponse(wire)
	if err != nil {
		t.Fatal(err)
	}
	if len(losses) != 0 {
		t.Fatalf("losses: %+v", losses)
	}
	if _, ok := resp.Content[0].(ir.CompactionItemBlock); !ok {
		t.Fatalf("content: %#v", resp.Content)
	}
	out, losses, err := EncodeResponse(resp)
	if err != nil {
		t.Fatal(err)
	}
	if len(losses) != 0 {
		t.Fatalf("encode losses: %+v", losses)
	}
	if out.Output[0].Type != ItemTypeCompaction || out.Output[0].EncryptedContent != "cipher" {
		t.Fatalf("output: %+v", out.Output)
	}
}
