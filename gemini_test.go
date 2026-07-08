package gemini

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/goloop/ai"
)

func newTestClient(t *testing.T, h http.HandlerFunc) (*Client, func()) {
	t.Helper()
	srv := httptest.NewServer(h)
	c := New("key", WithBaseURL(srv.URL), WithMaxRetries(0))
	return c, srv.Close
}

func TestGenerateText(t *testing.T) {
	c, done := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("x-goog-api-key") != "key" {
			t.Errorf("api key = %q", r.Header.Get("x-goog-api-key"))
		}
		if !strings.HasSuffix(r.URL.Path, ":generateContent") {
			t.Errorf("path = %q", r.URL.Path)
		}
		var req GenerateRequest
		body, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(body, &req); err != nil {
			t.Fatal(err)
		}
		if req.SystemInstruction == nil ||
			req.SystemInstruction.Parts[0].Text != "be brief" {
			t.Errorf("system = %+v", req.SystemInstruction)
		}
		if len(req.Contents) != 1 || req.Contents[0].Role != "user" {
			t.Errorf("contents = %+v", req.Contents)
		}
		io.WriteString(w, `{"candidates":[{"content":{"role":"model",`+
			`"parts":[{"text":"hello"}]},"finishReason":"STOP"}],`+
			`"usageMetadata":{"promptTokenCount":3,"candidatesTokenCount":2,"totalTokenCount":5}}`)
	})
	defer done()

	resp, err := c.Generate(context.Background(), &ai.Request{
		Model:    "m",
		System:   "be brief",
		Messages: []ai.Message{ai.UserText("hi")},
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Text() != "hello" {
		t.Errorf("text = %q", resp.Text())
	}
	if resp.Usage.InputTokens != 3 || resp.Usage.OutputTokens != 2 {
		t.Errorf("usage = %+v", resp.Usage)
	}
	if resp.StopReason != "STOP" {
		t.Errorf("stop = %q", resp.StopReason)
	}
}

func TestGenerateToolUse(t *testing.T) {
	c, done := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		var req GenerateRequest
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &req)
		if req.ToolConfig == nil ||
			req.ToolConfig.FunctionCallingConfig.Mode != "ANY" {
			t.Errorf("tool config = %+v", req.ToolConfig)
		}
		if len(req.Tools) != 1 || req.Tools[0].FunctionDeclarations[0].Name != "get_weather" {
			t.Errorf("tools = %+v", req.Tools)
		}
		io.WriteString(w, `{"candidates":[{"content":{"role":"model",`+
			`"parts":[{"functionCall":{"name":"get_weather","args":{"city":"Kyiv"}}}]},`+
			`"finishReason":"STOP"}]}`)
	})
	defer done()

	resp, err := c.Generate(context.Background(), &ai.Request{
		Model:      "m",
		Messages:   []ai.Message{ai.UserText("weather?")},
		Tools:      []ai.Tool{{Name: "get_weather"}},
		ToolChoice: ai.ToolRequired,
	})
	if err != nil {
		t.Fatal(err)
	}
	calls := resp.ToolCalls()
	if len(calls) != 1 || calls[0].Name != "get_weather" || calls[0].ID != "get_weather" {
		t.Fatalf("calls = %+v", calls)
	}
	if string(calls[0].Input) != `{"city":"Kyiv"}` {
		t.Errorf("input = %s", calls[0].Input)
	}
}

func TestStream(t *testing.T) {
	events := []string{
		`data: {"candidates":[{"content":{"parts":[{"text":"Hel"}]}}]}`, ``,
		`data: {"candidates":[{"content":{"parts":[{"text":"lo"}]},"finishReason":"STOP"}],` +
			`"usageMetadata":{"promptTokenCount":5,"candidatesTokenCount":2,"totalTokenCount":7}}`, ``,
	}
	c, done := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("alt") != "sse" {
			t.Errorf("alt = %q", r.URL.Query().Get("alt"))
		}
		for _, line := range events {
			io.WriteString(w, line+"\n")
		}
	})
	defer done()

	var text strings.Builder
	var usage ai.Usage
	var doneSeen bool
	for chunk, err := range c.Stream(context.Background(), &ai.Request{
		Model: "m", Messages: []ai.Message{ai.UserText("hi")},
	}) {
		if err != nil {
			t.Fatal(err)
		}
		text.WriteString(chunk.Text)
		if chunk.Done {
			doneSeen = true
			if chunk.Usage != nil {
				usage = *chunk.Usage
			}
		}
	}
	if text.String() != "Hello" || !doneSeen {
		t.Errorf("text = %q done = %v", text.String(), doneSeen)
	}
	if usage.InputTokens != 5 || usage.OutputTokens != 2 {
		t.Errorf("usage = %+v", usage)
	}
}

func TestStreamToolCall(t *testing.T) {
	events := []string{
		`data: {"candidates":[{"content":{"parts":[{"functionCall":` +
			`{"name":"lookup","args":{"q":42}}}]}}]}`, ``,
	}
	c, done := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		for _, line := range events {
			io.WriteString(w, line+"\n")
		}
	})
	defer done()

	var call *ai.ToolUse
	for chunk, err := range c.Stream(context.Background(), &ai.Request{
		Model: "m", Messages: []ai.Message{ai.UserText("hi")},
	}) {
		if err != nil {
			t.Fatal(err)
		}
		if chunk.ToolCall != nil {
			call = chunk.ToolCall
		}
	}
	if call == nil || call.Name != "lookup" || string(call.Input) != `{"q":42}` {
		t.Fatalf("call = %+v", call)
	}
}

func TestError(t *testing.T) {
	c, done := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		io.WriteString(w, `{"error":{"code":400,"message":"bad","status":"INVALID_ARGUMENT"}}`)
	})
	defer done()

	_, err := c.Generate(context.Background(), &ai.Request{
		Model: "m", Messages: []ai.Message{ai.UserText("hi")},
	})
	var apiErr *ai.APIError
	if !errors.As(err, &apiErr) || apiErr.Status != 400 || apiErr.Message != "bad" {
		t.Fatalf("err = %v", err)
	}
	if apiErr.Type != "INVALID_ARGUMENT" {
		t.Errorf("type = %q", apiErr.Type)
	}
}

func TestContentsMapping(t *testing.T) {
	req := &ai.Request{
		Messages: []ai.Message{
			{Role: ai.RoleUser, Parts: []ai.Part{
				ai.Text{Text: "look"},
				ai.Image{MIME: "image/png", Data: []byte{1, 2, 3}},
			}},
			{Role: ai.RoleAssistant, Parts: []ai.Part{
				ai.ToolUse{ID: "get_weather", Name: "get_weather", Input: json.RawMessage(`{}`)},
			}},
			{Role: ai.RoleTool, Parts: []ai.Part{
				ai.ToolResult{ID: "get_weather", Content: "42"},
			}},
		},
	}
	got := geminiContents(req)
	if len(got) != 3 {
		t.Fatalf("contents = %+v", got)
	}
	if got[0].Role != "user" || got[0].Parts[1].InlineData == nil {
		t.Errorf("user = %+v", got[0])
	}
	if got[0].Parts[1].InlineData.MIMEType != "image/png" {
		t.Errorf("inline mime = %q", got[0].Parts[1].InlineData.MIMEType)
	}
	if got[1].Role != "model" || got[1].Parts[0].FunctionCall == nil {
		t.Errorf("assistant = %+v", got[1])
	}
	fr := got[2].Parts[0].FunctionResponse
	if fr == nil || fr.Name != "get_weather" || string(fr.Response) != `{"result":"42"}` {
		t.Errorf("tool = %+v", fr)
	}
}

func TestValidate(t *testing.T) {
	c := New("key")
	_, err := c.Generate(context.Background(), &ai.Request{Model: "m"})
	if !errors.Is(err, ai.ErrNoMessages) {
		t.Errorf("want ErrNoMessages, got %v", err)
	}
}
