package gemini

import (
	"testing"

	"github.com/goloop/ai"
)

func TestToolCallKeepsSignatureOnReplay(t *testing.T) {
	part := Part{FunctionCall: &FunctionCall{Name: "lookup"}, ThoughtSignature: "signed-state"}
	call := toolUse(part, map[string]int{})
	history := &ai.Request{Messages: []ai.Message{{Role: ai.RoleAssistant, Parts: []ai.Part{call}}}}
	got := geminiContents(history)[0].Parts[0]
	if got.ThoughtSignature != part.ThoughtSignature || got.FunctionCall.Name != "lookup" {
		t.Fatalf("replayed call = %+v", got)
	}
}

func TestParallelToolCallsRetainSignatures(t *testing.T) {
	names := map[string]int{}
	first := toolUse(Part{FunctionCall: &FunctionCall{Name: "lookup"}, ThoughtSignature: "signed"}, names)
	second := toolUse(Part{FunctionCall: &FunctionCall{Name: "lookup"}}, names)
	if first.ID == second.ID {
		t.Fatalf("duplicate call ID %q", first.ID)
	}
	history := &ai.Request{Messages: []ai.Message{
		{Role: ai.RoleAssistant, Parts: []ai.Part{first, second}},
		{Role: ai.RoleTool, Parts: []ai.Part{
			ai.ToolResult{ID: first.ID, Content: "first result"},
			ai.ToolResult{ID: second.ID, Content: "second result"},
		}},
	}}
	got := geminiContents(history)
	if len(got) != 2 || len(got[0].Parts) != 2 || got[0].Parts[0].ThoughtSignature != "signed" ||
		got[0].Parts[1].ThoughtSignature != "" || len(got[1].Parts) != 2 ||
		got[1].Parts[0].FunctionResponse.Name != "lookup" || got[1].Parts[1].FunctionResponse.Name != "lookup" {
		t.Fatalf("replayed history = %+v", got)
	}
}
