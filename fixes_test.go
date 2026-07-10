package gemini

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/goloop/ai"
)

// TestToolCallIDsUnique verifies two calls to the same function in one turn get
// distinct IDs, and that those IDs resolve back to the function name on the way
// out (Gemini matches tool results by name).
func TestToolCallIDsUnique(t *testing.T) {
	c, done := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"candidates":[{"content":{"parts":[`+
			`{"functionCall":{"name":"search","args":{"q":"a"}}},`+
			`{"functionCall":{"name":"search","args":{"q":"b"}}}]}}]}`)
	})
	defer done()

	resp, err := c.Generate(context.Background(), &ai.Request{
		Model:    "m",
		Messages: []ai.Message{ai.UserText("hi")},
	})
	if err != nil {
		t.Fatal(err)
	}
	calls := resp.ToolCalls()
	if len(calls) != 2 {
		t.Fatalf("calls = %d, want 2", len(calls))
	}
	if calls[0].ID == calls[1].ID {
		t.Fatalf("IDs not unique: %q", calls[0].ID)
	}

	// Feed both results back; each must resolve to the function name "search".
	contents := geminiContents(&ai.Request{
		Messages: []ai.Message{
			{Role: ai.RoleAssistant, Parts: []ai.Part{calls[0], calls[1]}},
			{Role: ai.RoleTool, Parts: []ai.Part{
				ai.ToolResult{ID: calls[0].ID, Content: "ra"},
				ai.ToolResult{ID: calls[1].ID, Content: "rb"},
			}},
		},
	})
	var names []string
	for _, ct := range contents {
		for _, p := range ct.Parts {
			if p.FunctionResponse != nil {
				names = append(names, p.FunctionResponse.Name)
			}
		}
	}
	if len(names) != 2 || names[0] != "search" || names[1] != "search" {
		t.Fatalf("resolved names = %v, want [search search]", names)
	}
}

// TestPromptBlocked verifies a safety-blocked prompt (HTTP 200, no candidates,
// a blockReason) surfaces as an error instead of an empty response.
func TestPromptBlocked(t *testing.T) {
	c, done := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"candidates":[],"promptFeedback":{"blockReason":"SAFETY"}}`)
	})
	defer done()

	_, err := c.Generate(context.Background(), &ai.Request{
		Model:    "m",
		Messages: []ai.Message{ai.UserText("hi")},
	})
	var apiErr *ai.APIError
	if !errors.As(err, &apiErr) || apiErr.Status != http.StatusBadRequest {
		t.Fatalf("err = %v, want APIError 400", err)
	}
	if !strings.Contains(apiErr.Message, "SAFETY") {
		t.Errorf("message = %q, want it to mention SAFETY", apiErr.Message)
	}
}
