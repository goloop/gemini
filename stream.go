package gemini

import (
	"context"
	"encoding/json"
	"io"
	"iter"
	"net/http"

	"github.com/goloop/ai"
)

// Stream implements [ai.Client]. It maps the request onto
// streamGenerateContent and yields text deltas, completed tool calls and a
// final chunk carrying usage.
func (c *Client) Stream(
	ctx context.Context,
	req *ai.Request,
) iter.Seq2[ai.Chunk, error] {
	return func(yield func(ai.Chunk, error) bool) {
		if err := req.Validate(); err != nil {
			yield(ai.Chunk{}, err)
			return
		}
		body, err := json.Marshal(geminiRequest(req))
		if err != nil {
			yield(ai.Chunk{}, err)
			return
		}

		resp, err := c.openStream(ctx, req.Model, body)
		if err != nil {
			yield(ai.Chunk{}, err)
			return
		}
		defer resp.Body.Close()

		var usage *ai.Usage
		seen := map[string]int{}
		for data, err := range ai.SSEEvents(resp.Body) {
			if err != nil {
				yield(ai.Chunk{}, err)
				return
			}
			var gr GenerateResponse
			if json.Unmarshal([]byte(data), &gr) != nil {
				continue
			}
			if gr.UsageMetadata != nil {
				usage = &ai.Usage{
					InputTokens:  gr.UsageMetadata.PromptTokenCount,
					OutputTokens: gr.UsageMetadata.CandidatesTokenCount,
				}
			}
			if err := blockedError(&gr); err != nil {
				yield(ai.Chunk{}, err)
				return
			}
			if len(gr.Candidates) == 0 {
				continue
			}
			for _, p := range gr.Candidates[0].Content.Parts {
				var chunk ai.Chunk
				switch {
				case p.FunctionCall != nil:
					chunk.ToolCall = &ai.ToolUse{
						ID:    toolCallID(p.FunctionCall.Name, seen),
						Name:  p.FunctionCall.Name,
						Input: p.FunctionCall.Args,
					}
				case p.Text != "":
					chunk.Text = p.Text
				default:
					continue
				}
				chunk.Raw = json.RawMessage(data)
				if !yield(chunk, nil) {
					return
				}
			}
		}
		yield(ai.Chunk{Done: true, Usage: usage}, nil)
	}
}

// StreamGenerateContent sends a native streaming request and yields each
// response chunk as it arrives.
func (c *Client) StreamGenerateContent(
	ctx context.Context,
	model string,
	req *GenerateRequest,
) iter.Seq2[*GenerateResponse, error] {
	return func(yield func(*GenerateResponse, error) bool) {
		body, err := json.Marshal(req)
		if err != nil {
			yield(nil, err)
			return
		}
		resp, err := c.openStream(ctx, model, body)
		if err != nil {
			yield(nil, err)
			return
		}
		defer resp.Body.Close()

		for data, err := range ai.SSEEvents(resp.Body) {
			if err != nil {
				yield(nil, err)
				return
			}
			var gr GenerateResponse
			if json.Unmarshal([]byte(data), &gr) != nil {
				continue
			}
			if !yield(&gr, nil) {
				return
			}
		}
	}
}

// openStream opens the SSE streamGenerateContent connection for a model.
func (c *Client) openStream(
	ctx context.Context,
	model string,
	body []byte,
) (*http.Response, error) {
	url := c.opts.BaseURL + "/models/" + model + ":streamGenerateContent?alt=sse"
	resp, err := c.opts.Do(ctx, http.MethodPost, url, body, c.headers())
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		data, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, parseError(resp.StatusCode, data)
	}
	return resp, nil
}
