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

// ToolDecls is one entry of the tools list: either the caller's function
// declarations or one of the provider's own tools. A request that wants both
// sends two entries.
type ToolDecls struct {
	FunctionDeclarations []FunctionDeclaration `json:"functionDeclarations,omitempty"`

	// GoogleSearch turns on grounding with Google Search, which the provider
	// runs itself. It takes no settings, which is why it is an empty object.
	GoogleSearch *GoogleSearch `json:"googleSearch,omitempty"`
}

// GoogleSearch asks the provider to ground its answer in a search it runs
// itself. It has no fields: this provider decides how many searches to run and
// over what, and offers nothing to narrow them with.
type GoogleSearch struct{}

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
	Temperature      *float64 `json:"temperature,omitempty"`
	TopP             *float64 `json:"topP,omitempty"`
	MaxOutputTokens  int      `json:"maxOutputTokens,omitempty"`
	StopSequences    []string `json:"stopSequences,omitempty"`
	ResponseMIMEType string   `json:"responseMimeType,omitempty"`

	// ResponseSchema constrains the answer to a schema written in the
	// OpenAPI 3.0 subset the API defines. It is not JSON Schema, and the two
	// diverge as soon as a schema uses anything beyond type, properties and
	// required.
	ResponseSchema json.RawMessage `json:"responseSchema,omitempty"`

	// ResponseJSONSchema constrains the answer to a JSON Schema. It is what
	// ai.Format.Schema is, so that is where this driver puts it. The API
	// rejects a request that sets both this and ResponseSchema; either one
	// still needs ResponseMIMEType.
	ResponseJSONSchema json.RawMessage `json:"responseJsonSchema,omitempty"`
}

// GenerateResponse is the native generateContent (and stream chunk) response.
type GenerateResponse struct {
	Candidates     []Candidate     `json:"candidates"`
	UsageMetadata  *UsageMetadata  `json:"usageMetadata,omitempty"`
	ModelVersion   string          `json:"modelVersion,omitempty"`
	PromptFeedback *PromptFeedback `json:"promptFeedback,omitempty"`
}

// PromptFeedback reports why a prompt was rejected. BlockReason is empty for a
// prompt that was not blocked (for example "SAFETY" or "PROHIBITED_CONTENT").
type PromptFeedback struct {
	BlockReason string `json:"blockReason,omitempty"`
}

// Candidate is one generated response option.
type Candidate struct {
	Content      Content `json:"content"`
	FinishReason string  `json:"finishReason,omitempty"`
	Index        int     `json:"index"`

	// GroundingMetadata is present when the answer was grounded in a search
	// the provider ran. It is where the sources live.
	GroundingMetadata *GroundingMetadata `json:"groundingMetadata,omitempty"`
}

// GroundingMetadata reports the search behind a grounded answer: what was
// searched for, what was found, and which stretch of the answer each finding
// supports.
type GroundingMetadata struct {
	WebSearchQueries  []string           `json:"webSearchQueries,omitempty"`
	GroundingChunks   []GroundingChunk   `json:"groundingChunks,omitempty"`
	GroundingSupports []GroundingSupport `json:"groundingSupports,omitempty"`
}

// GroundingChunk is one source the search found.
type GroundingChunk struct {
	Web *GroundingWeb `json:"web,omitempty"`
}

// GroundingWeb is a web page a search found.
type GroundingWeb struct {
	URI    string `json:"uri,omitempty"`
	Title  string `json:"title,omitempty"`
	Domain string `json:"domain,omitempty"`
}

// GroundingSupport ties a stretch of the answer to the sources behind it.
// Sources are named by their position in GroundingMetadata.GroundingChunks.
type GroundingSupport struct {
	Segment               *Segment `json:"segment,omitempty"`
	GroundingChunkIndices []int    `json:"groundingChunkIndices,omitempty"`
}

// Segment is a stretch of the generated answer. StartIndex and EndIndex are
// byte offsets, which is what [ai.Citation] wants, so unlike most providers
// this one can say which sentence a source backs rather than only listing
// sources.
type Segment struct {
	PartIndex  int    `json:"partIndex,omitempty"`
	StartIndex int    `json:"startIndex,omitempty"`
	EndIndex   int    `json:"endIndex,omitempty"`
	Text       string `json:"text,omitempty"`
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
