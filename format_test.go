package gemini

import (
	"encoding/json"
	"strings"
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
				if string(cfg.ResponseJSONSchema) != c.schema {
					t.Errorf("responseJsonSchema = %s, want %s",
						cfg.ResponseJSONSchema, c.schema)
				}
				// The two schema fields are mutually exclusive, and only one
				// of them takes what ai.Format.Schema actually is.
				if len(cfg.ResponseSchema) != 0 {
					t.Errorf("responseSchema = %s; a JSON Schema does not go "+
						"in the OpenAPI field", cfg.ResponseSchema)
				}
			}
			if got := formatMode(c.format); got != c.mode {
				t.Errorf("formatMode = %s, want %s", got, c.mode)
			}
		})
	}
}

// TestJSONSchemaFeaturesSurvive covers why the field matters. responseSchema
// takes an OpenAPI 3.0 subset; a schema using JSON Schema's own vocabulary
// has to reach the API whole, through the field that speaks it.
func TestJSONSchemaFeaturesSurvive(t *testing.T) {
	schema := json.RawMessage(`{"type":"object","additionalProperties":false,` +
		`"properties":{"a":{"oneOf":[{"type":"string"},{"type":"null"}]}},` +
		`"$defs":{"x":{"type":"integer"}}}`)

	cfg := geminiRequest(askFor(&ai.Format{
		Type:   ai.FormatJSONSchema,
		Schema: schema,
	})).GenerationConfig
	if cfg == nil {
		t.Fatal("no generationConfig was sent")
	}
	if string(cfg.ResponseJSONSchema) != string(schema) {
		t.Errorf("schema was not sent verbatim:\n got %s\nwant %s",
			cfg.ResponseJSONSchema, schema)
	}
}

// TestNativeSchemaFieldStillWorks checks the OpenAPI field is still there for
// callers who build a GenerateRequest themselves and mean that dialect.
func TestNativeSchemaFieldStillWorks(t *testing.T) {
	gr := &GenerateRequest{
		GenerationConfig: &GenerationConfig{
			ResponseMIMEType: "application/json",
			ResponseSchema:   json.RawMessage(`{"type":"object"}`),
		},
	}
	body, err := json.Marshal(gr)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), `"responseSchema"`) {
		t.Errorf("responseSchema was dropped: %s", body)
	}
	if strings.Contains(string(body), `"responseJsonSchema"`) {
		t.Errorf("responseJsonSchema appeared unasked: %s", body)
	}
}

// TestGenerationConfigStaysAbsent checks a request that asks for nothing still
// sends no generationConfig at all, as it did before the field existed.
func TestGenerationConfigStaysAbsent(t *testing.T) {
	if gr := geminiRequest(askFor(nil)); gr.GenerationConfig != nil {
		t.Errorf("generationConfig = %+v, want nil", gr.GenerationConfig)
	}
}
