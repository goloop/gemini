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
		greq, err := geminiRequest(req)
		if err != nil {
			yield(ai.Chunk{}, err)
			return
		}
		body, err := json.Marshal(greq)
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

		// searches is the largest count any event reported, and sourced is how
		// many sources have already been passed on. This provider repeats the
		// whole metadata block as it grows, so without the second counter the
		// same source would be reported on every event that carries it.
		var searches, sourced int
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

			if gm := gr.Candidates[0].GroundingMetadata; gm != nil {
				if n := hostedCalls(gm)[ai.HostedWebSearch]; n > searches {
					searches = n
				}
				cs := streamCitations(gm, sourced)
				if n := len(gm.GroundingChunks); n > sourced {
					sourced = n
				}
				if len(cs) > 0 {
					if !yield(ai.Chunk{
						Citations: cs,
						Raw:       json.RawMessage(data),
					}, nil) {
						return
					}
				}
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
		reports, err := req.HostedReports(
			map[ai.HostedKind]int{ai.HostedWebSearch: searches})
		if err != nil {
			yield(ai.Chunk{}, err)
			return
		}
		yield(ai.Chunk{Done: true, Usage: usage, Hosted: reports}, nil)
	}
}

// streamCitations returns the sources this provider has found beyond the
// first skip of them.
//
// They come without a byte range, unlike the ones [Client.Generate] returns.
// The provider measures a range against the finished answer, and a stream has
// no finished answer to measure against while it is still arriving; a range
// that looks like an offset into the chunk it rides on would point at the
// wrong words. A caller that needs the ranges asks for the whole answer.
func streamCitations(gm *GroundingMetadata, skip int) []ai.Citation {
	var out []ai.Citation
	for _, c := range gm.GroundingChunks[min(skip, len(gm.GroundingChunks)):] {
		if c.Web == nil {
			continue
		}
		out = append(out, ai.Citation{URL: c.Web.URI, Title: c.Web.Title})
	}
	return out
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
