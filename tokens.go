package gemini

import (
	"context"
	"github.com/goloop/ai"
)

// CountTokens reports how many tokens the given content would consume for a
// model, without generating a response.
func (c *Client) CountTokens(
	ctx context.Context,
	model string,
	req *GenerateRequest,
) (int, error) {
	if req == nil {
		return 0, ai.ErrNoRequest
	}
	body := struct {
		Contents          []Content   `json:"contents,omitempty"`
		SystemInstruction *Content    `json:"systemInstruction,omitempty"`
		Tools             []ToolDecls `json:"tools,omitempty"`
	}{
		Contents:          req.Contents,
		SystemInstruction: req.SystemInstruction,
		Tools:             req.Tools,
	}

	var out struct {
		TotalTokens int `json:"totalTokens"`
	}
	path := "/models/" + model + ":countTokens"
	if err := c.postJSON(ctx, path, body, &out); err != nil {
		return 0, err
	}
	return out.TotalTokens, nil
}
