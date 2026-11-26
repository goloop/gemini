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
