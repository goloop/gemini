package gemini

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/goloop/ai"
)

// Generate implements [ai.Client]. It maps the request onto generateContent and
// returns the first candidate as an [ai.Response].
func (c *Client) Generate(
	ctx context.Context,
	req *ai.Request,
) (*ai.Response, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}
	gr, raw, err := c.generateContent(ctx, req.Model, geminiRequest(req))
	if err != nil {
		return nil, err
	}
	if err := blockedError(gr); err != nil {
		return nil, err
	}
	return geminiToResponse(req.Model, gr, raw), nil
}

// blockedError returns an [ai.APIError] when a response carries no candidates
// because the prompt was blocked by a safety filter. Such a response has HTTP
// 200 with an empty candidate list, so without this check Generate would return
// an empty result with no explanation.
func blockedError(gr *GenerateResponse) error {
	if len(gr.Candidates) > 0 || gr.PromptFeedback == nil ||
		gr.PromptFeedback.BlockReason == "" {
		return nil
	}
	return &ai.APIError{
		Status:  http.StatusBadRequest,
		Type:    "blocked",
		Message: "prompt blocked: " + gr.PromptFeedback.BlockReason,
	}
}

// toolCallID synthesizes a unique ID for a Gemini function call. Gemini does
// not supply call IDs, so callers cannot otherwise tell apart two calls to the
// same function in one turn. The per-name counter makes the ID unique; the
// reverse lookup in geminiContents still resolves it back to the function name.
func toolCallID(name string, seen map[string]int) string {
	id := name + "-" + strconv.Itoa(seen[name])
	seen[name]++
	return id
}

// GenerateContent sends a native generateContent request for the given model.
func (c *Client) GenerateContent(
	ctx context.Context,
	model string,
	req *GenerateRequest,
) (*GenerateResponse, error) {
	gr, _, err := c.generateContent(ctx, model, req)
	return gr, err
}

// generateContent posts req and returns the decoded response and its raw body.
func (c *Client) generateContent(
	ctx context.Context,
	model string,
	req *GenerateRequest,
) (*GenerateResponse, []byte, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, nil, err
	}
	path := "/models/" + model + ":generateContent"
	data, status, err := c.send(ctx, http.MethodPost, path, body)
	if err != nil {
		return nil, nil, err
	}
	if status != http.StatusOK {
		return nil, nil, parseError(status, data)
	}
	var gr GenerateResponse
	if err := json.Unmarshal(data, &gr); err != nil {
		return nil, nil, err
	}
	return &gr, data, nil
}

// geminiRequest maps an ai.Request onto a native GenerateRequest.
func geminiRequest(req *ai.Request) *GenerateRequest {
	return &GenerateRequest{
		Contents:          geminiContents(req),
		SystemInstruction: geminiSystem(req),
		Tools:             geminiTools(req),
		ToolConfig:        geminiToolConfig(req),
		GenerationConfig:  geminiGenConfig(req),
	}
}

// geminiContents maps conversation messages to Gemini contents. System
// messages are handled separately. Gemini matches tool results to calls by
// function name, so tool-call IDs are resolved back to their names.
func geminiContents(req *ai.Request) []Content {
	names := map[string]string{}
	for _, m := range req.Messages {
		for _, p := range m.Parts {
			if tu, ok := p.(ai.ToolUse); ok {
				names[tu.ID] = tu.Name
			}
		}
	}

	var out []Content
	for _, m := range req.Messages {
		if m.Role == ai.RoleSystem {
			continue
		}
		role := "user"
		if m.Role == ai.RoleAssistant {
			role = "model"
		}

		var parts []Part
		for _, p := range m.Parts {
			switch v := p.(type) {
			case ai.Text:
				parts = append(parts, Part{Text: v.Text})
			case ai.Image:
				if len(v.Data) > 0 {
					parts = append(parts, Part{InlineData: &Blob{
						MIMEType: v.MIME,
						Data:     base64.StdEncoding.EncodeToString(v.Data),
					}})
				} else if v.URL != "" {
					parts = append(parts, Part{FileData: &FileData{
						MIMEType: v.MIME, FileURI: v.URL,
					}})
				}
			case ai.ToolUse:
				parts = append(parts, Part{FunctionCall: &FunctionCall{
					Name: v.Name, Args: v.Input,
				}})
			case ai.ToolResult:
				name := names[v.ID]
				if name == "" {
					name = v.ID
				}
				parts = append(parts, Part{FunctionResponse: &FunctionResponse{
					Name: name, Response: functionResponsePayload(v),
				}})
			}
		}
		out = append(out, Content{Role: role, Parts: parts})
	}
	return out
}

// functionResponsePayload wraps a tool result in the JSON object Gemini
// expects. A JSON-object Content is passed through; anything else is sent as a
// string under "result" (or "error" when the tool failed).
func functionResponsePayload(tr ai.ToolResult) json.RawMessage {
	key := "result"
	if tr.IsError {
		key = "error"
	}
	if trimmed := strings.TrimSpace(tr.Content); strings.HasPrefix(trimmed, "{") &&
		json.Valid([]byte(trimmed)) {
		b, _ := json.Marshal(map[string]json.RawMessage{
			key: json.RawMessage(trimmed),
		})
		return b
	}
	b, _ := json.Marshal(map[string]string{key: tr.Content})
	return b
}

// geminiSystem collects the system prompt and any system messages into a
// single system instruction, or nil when there is none.
func geminiSystem(req *ai.Request) *Content {
	var texts []string
	if req.System != "" {
		texts = append(texts, req.System)
	}
	for _, m := range req.Messages {
		if m.Role != ai.RoleSystem {
			continue
		}
		for _, p := range m.Parts {
			if t, ok := p.(ai.Text); ok {
				texts = append(texts, t.Text)
			}
		}
	}
	if len(texts) == 0 {
		return nil
	}
	return &Content{Parts: []Part{{Text: strings.Join(texts, "\n")}}}
}

// geminiTools maps ai tools to a single function-declaration group.
func geminiTools(req *ai.Request) []ToolDecls {
	if len(req.Tools) == 0 {
		return nil
	}
	decls := make([]FunctionDeclaration, len(req.Tools))
	for i, t := range req.Tools {
		decls[i] = FunctionDeclaration{
			Name:        t.Name,
			Description: t.Description,
			Parameters:  t.Schema,
		}
	}
	return []ToolDecls{{FunctionDeclarations: decls}}
}

// geminiToolConfig maps the tool-choice strategy, or nil when no tools are set.
func geminiToolConfig(req *ai.Request) *ToolConfig {
	if len(req.Tools) == 0 {
		return nil
	}
	var mode string
	switch req.ToolChoice {
	case ai.ToolAuto:
		mode = "AUTO"
	case ai.ToolNone:
		mode = "NONE"
	case ai.ToolRequired:
		mode = "ANY"
	}
	return &ToolConfig{FunctionCallingConfig: &FunctionCallingConfig{Mode: mode}}
}

// geminiGenConfig maps generation parameters, or nil when none are set.
func geminiGenConfig(req *ai.Request) *GenerationConfig {
	g := &GenerationConfig{
		Temperature:   req.Temperature,
		TopP:          req.TopP,
		StopSequences: req.Stop,
	}
	if req.MaxTokens > 0 {
		g.MaxOutputTokens = req.MaxTokens
	}
	if g.Temperature == nil && g.TopP == nil &&
		g.MaxOutputTokens == 0 && len(g.StopSequences) == 0 {
		return nil
	}
	return g
}

// geminiToResponse maps a native response onto an ai.Response. Function calls
// use their name as the tool-call ID, since Gemini keys results by name.
func geminiToResponse(model string, gr *GenerateResponse, raw []byte) *ai.Response {
	resp := &ai.Response{Model: model, Raw: append(json.RawMessage(nil), raw...)}
	if len(gr.Candidates) > 0 {
		cand := gr.Candidates[0]
		resp.StopReason = cand.FinishReason
		seen := map[string]int{}
		for _, p := range cand.Content.Parts {
			switch {
			case p.FunctionCall != nil:
				resp.Parts = append(resp.Parts, ai.ToolUse{
					ID:    toolCallID(p.FunctionCall.Name, seen),
					Name:  p.FunctionCall.Name,
					Input: p.FunctionCall.Args,
				})
			case p.Text != "":
				resp.Parts = append(resp.Parts, ai.Text{Text: p.Text})
			}
		}
	}
	if gr.UsageMetadata != nil {
		resp.Usage = ai.Usage{
			InputTokens:  gr.UsageMetadata.PromptTokenCount,
			OutputTokens: gr.UsageMetadata.CandidatesTokenCount,
		}
	}
	return resp
}
