package gemini

import (
	"encoding/json"
	"testing"

	"github.com/goloop/ai"
)

func askFor(f *ai.Format) *ai.Request {
	return &ai.Request{
		Model:    "the-model",
		Messages: []ai.Message{ai.UserText("Describe this article.")},
		Format:   f,
	}
}

// TestResponseFormat checks the request carries what this provider wants: the
// MIME type of the answer, with the schema beside it rather than inside it.
func TestResponseFormat(t *testing.T) {
	schema := json.RawMessage(`{"type":"object","properties":{"a":{"type":"string"}}}`)

	cases := []struct {
		name   string
		format *ai.Format
		mime   string
		schema string
		mode   ai.FormatMode
	}{
		{"nothing asked", nil, "", "", ai.FormatNone},
		{"text", &ai.Format{Type: ai.FormatText}, "", "", ai.FormatNone},
		{"json", &ai.Format{Type: ai.FormatJSON}, "application/json", "", ai.FormatNative},
		{
			"json schema",
			&ai.Format{Type: ai.FormatJSONSchema, Schema: schema},
			"application/json", string(schema), ai.FormatNative,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			gr := geminiRequest(askFor(c.format))
			cfg := gr.GenerationConfig

			if c.mime == "" {
				if cfg != nil && cfg.ResponseMIMEType != "" {
					t.Errorf("responseMimeType = %q, want it absent", cfg.ResponseMIMEType)
				}
			} else {
				if cfg == nil {
					t.Fatal("no generationConfig was sent")
				}
				if cfg.ResponseMIMEType != c.mime {
					t.Errorf("responseMimeType = %q, want %q", cfg.ResponseMIMEType, c.mime)
				}
				if string(cfg.ResponseSchema) != c.schema {
					t.Errorf("responseSchema = %s, want %s", cfg.ResponseSchema, c.schema)
				}
			}
			if got := formatMode(c.format); got != c.mode {
				t.Errorf("formatMode = %s, want %s", got, c.mode)
			}
		})
	}
}

// TestGenerationConfigStaysAbsent checks a request that asks for nothing still
// sends no generationConfig at all, as it did before the field existed.
func TestGenerationConfigStaysAbsent(t *testing.T) {
	if gr := geminiRequest(askFor(nil)); gr.GenerationConfig != nil {
		t.Errorf("generationConfig = %+v, want nil", gr.GenerationConfig)
	}
}
