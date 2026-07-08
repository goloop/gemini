package gemini

import (
	"encoding/json"
	"strings"
)

// Content is one turn of a conversation: a role ("user" or "model") and its
// parts. System text is carried separately in GenerateRequest.SystemInstruction
// and leaves Role empty.
type Content struct {
	Role  string `json:"role,omitempty"`
	Parts []Part `json:"parts"`
}

// Part is a single piece of a Content. Exactly one field is set: Text for
// plain text, InlineData for embedded bytes, FileData for a referenced file,
// FunctionCall for a model tool call, or FunctionResponse for a tool result.
type Part struct {
	Text             string            `json:"text,omitempty"`
	InlineData       *Blob             `json:"inlineData,omitempty"`
	FileData         *FileData         `json:"fileData,omitempty"`
	FunctionCall     *FunctionCall     `json:"functionCall,omitempty"`
	FunctionResponse *FunctionResponse `json:"functionResponse,omitempty"`
}

// Blob is inline binary data, such as an image, with its MIME type. Data is
// base64-encoded.
type Blob struct {
	MIMEType string `json:"mimeType"`
	Data     string `json:"data"`
}

// FileData references data by URI, such as an uploaded file or a supported
// remote resource.
type FileData struct {
	MIMEType string `json:"mimeType,omitempty"`
	FileURI  string `json:"fileUri"`
}

// FunctionCall is a request from the model to call a declared function. Args is
// the JSON arguments object.
type FunctionCall struct {
	Name string          `json:"name"`
	Args json.RawMessage `json:"args,omitempty"`
}

// FunctionResponse carries a function's result back to the model. Response is a
// JSON object.
type FunctionResponse struct {
	Name     string          `json:"name"`
	Response json.RawMessage `json:"response"`
}

// GenerateRequest is the native generateContent request body.
type GenerateRequest struct {
	Contents          []Content         `json:"contents"`
	SystemInstruction *Content          `json:"systemInstruction,omitempty"`
	Tools             []ToolDecls       `json:"tools,omitempty"`
	ToolConfig        *ToolConfig       `json:"toolConfig,omitempty"`
	GenerationConfig  *GenerationConfig `json:"generationConfig,omitempty"`
}

// ToolDecls groups the function declarations offered to the model.
type ToolDecls struct {
	FunctionDeclarations []FunctionDeclaration `json:"functionDeclarations,omitempty"`
}

// FunctionDeclaration describes a callable function. Parameters is a JSON
// Schema object describing its arguments.
type FunctionDeclaration struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Parameters  json.RawMessage `json:"parameters,omitempty"`
}

// ToolConfig controls tool calling for a request.
type ToolConfig struct {
	FunctionCallingConfig *FunctionCallingConfig `json:"functionCallingConfig,omitempty"`
}

// FunctionCallingConfig sets the tool-calling mode: "AUTO", "ANY" or "NONE".
type FunctionCallingConfig struct {
	Mode string `json:"mode,omitempty"`
}

// GenerationConfig tunes generation. Temperature and TopP are pointers so an
// explicit zero is distinct from unset.
type GenerationConfig struct {
	Temperature      *float64        `json:"temperature,omitempty"`
	TopP             *float64        `json:"topP,omitempty"`
	MaxOutputTokens  int             `json:"maxOutputTokens,omitempty"`
	StopSequences    []string        `json:"stopSequences,omitempty"`
	ResponseMIMEType string          `json:"responseMimeType,omitempty"`
	ResponseSchema   json.RawMessage `json:"responseSchema,omitempty"`
}

// GenerateResponse is the native generateContent (and stream chunk) response.
type GenerateResponse struct {
	Candidates    []Candidate    `json:"candidates"`
	UsageMetadata *UsageMetadata `json:"usageMetadata,omitempty"`
	ModelVersion  string         `json:"modelVersion,omitempty"`
}

// Candidate is one generated response option.
type Candidate struct {
	Content      Content `json:"content"`
	FinishReason string  `json:"finishReason,omitempty"`
	Index        int     `json:"index"`
}

// UsageMetadata reports token counts for a request.
type UsageMetadata struct {
	PromptTokenCount     int `json:"promptTokenCount"`
	CandidatesTokenCount int `json:"candidatesTokenCount"`
	TotalTokenCount      int `json:"totalTokenCount"`
}

// Text returns the concatenation of the text parts of the first candidate.
func (r *GenerateResponse) Text() string {
	if len(r.Candidates) == 0 {
		return ""
	}
	var b strings.Builder
	for _, p := range r.Candidates[0].Content.Parts {
		b.WriteString(p.Text)
	}
	return b.String()
}
