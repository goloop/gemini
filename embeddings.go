package gemini

import "context"

// EmbedRequest is the native embedContent request body.
type EmbedRequest struct {
	Content              Content `json:"content"`
	TaskType             string  `json:"taskType,omitempty"`
	Title                string  `json:"title,omitempty"`
	OutputDimensionality int     `json:"outputDimensionality,omitempty"`
}

// Embedding is a single embedding vector.
type Embedding struct {
	Values []float64 `json:"values"`
}

// EmbedContent embeds a single request with the given model.
func (c *Client) EmbedContent(
	ctx context.Context,
	model string,
	req *EmbedRequest,
) (*Embedding, error) {
	var out struct {
		Embedding Embedding `json:"embedding"`
	}
	path := "/models/" + model + ":embedContent"
	if err := c.postJSON(ctx, path, req, &out); err != nil {
		return nil, err
	}
	return &out.Embedding, nil
}

// Embed embeds one or more plain-text inputs and returns their vectors in
// order, using a single batchEmbedContents call.
func (c *Client) Embed(
	ctx context.Context,
	model string,
	texts ...string,
) ([][]float64, error) {
	type item struct {
		Model   string  `json:"model"`
		Content Content `json:"content"`
	}
	reqs := make([]item, len(texts))
	for i, t := range texts {
		reqs[i] = item{
			Model:   "models/" + model,
			Content: Content{Parts: []Part{{Text: t}}},
		}
	}

	var out struct {
		Embeddings []Embedding `json:"embeddings"`
	}
	body := struct {
		Requests []item `json:"requests"`
	}{Requests: reqs}
	path := "/models/" + model + ":batchEmbedContents"
	if err := c.postJSON(ctx, path, body, &out); err != nil {
		return nil, err
	}

	vecs := make([][]float64, len(out.Embeddings))
	for i, e := range out.Embeddings {
		vecs[i] = e.Values
	}
	return vecs, nil
}
