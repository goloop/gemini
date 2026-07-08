package gemini

import (
	"context"
	"strings"
)

// Model describes a Gemini model as reported by the API.
type Model struct {
	Name                       string   `json:"name"`
	BaseModelID                string   `json:"baseModelId,omitempty"`
	Version                    string   `json:"version,omitempty"`
	DisplayName                string   `json:"displayName,omitempty"`
	Description                string   `json:"description,omitempty"`
	InputTokenLimit            int      `json:"inputTokenLimit,omitempty"`
	OutputTokenLimit           int      `json:"outputTokenLimit,omitempty"`
	SupportedGenerationMethods []string `json:"supportedGenerationMethods,omitempty"`
}

// Models lists the models available to the account.
func (c *Client) Models(ctx context.Context) ([]Model, error) {
	var out struct {
		Models []Model `json:"models"`
	}
	if err := c.getJSON(ctx, "/models?pageSize=1000", &out); err != nil {
		return nil, err
	}
	return out.Models, nil
}

// GetModel retrieves one model by name, with or without the "models/" prefix.
func (c *Client) GetModel(ctx context.Context, model string) (*Model, error) {
	var m Model
	path := "/models/" + strings.TrimPrefix(model, "models/")
	if err := c.getJSON(ctx, path, &m); err != nil {
		return nil, err
	}
	return &m, nil
}
