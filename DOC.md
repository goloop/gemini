# gemini - reference

The full reference for the `gemini` package: the client, the shared `goloop/ai`
model, content generation (interface and native), streaming, embeddings, token
counting and models.

Ukrainian version: **[DOC.UK.md](DOC.UK.md)**.

## Contents

- [Mental model](#mental-model)
- [Creating a client](#creating-a-client)
- [Generate and Stream](#generate-and-stream)
- [Structured output](#structured-output)
- [Hosted web search](#hosted-web-search)
- [Native generateContent](#native-generatecontent)
- [Tools and tool results](#tools-and-tool-results)
- [Images and system prompts](#images-and-system-prompts)
- [Embeddings](#embeddings)
- [Token counting](#token-counting)
- [Models](#models)
- [Options and errors](#options-and-errors)

## Mental model

`gemini.Client` implements `ai.Client`, the provider-agnostic contract from
`github.com/goloop/ai`. The shared `Generate` and `Stream` cover the common
ground - chat with tools, images and streaming - so code written against the
interface runs on any provider.

Gemini-specific power lives in native methods: the full `GenerateContent`
request with its generation config and response schema, embeddings, token
counting and model listing. Those are not part of the shared interface.

```go
import (
	"github.com/goloop/ai"
	"github.com/goloop/gemini"
)
```

## Creating a client

```go
c := gemini.New(os.Getenv("GEMINI_API_KEY"))

c = gemini.New(apiKey, gemini.WithTimeout(30*time.Second))
```

The base URL defaults to `https://generativelanguage.googleapis.com/v1beta`.
The API key is sent in the `x-goog-api-key` header. Point `WithBaseURL` at any
compatible endpoint to reuse this client against another gateway.

## Generate and Stream

```go
resp, err := c.Generate(ctx, &ai.Request{
	Model:    gemini.ModelGemini25Flash,
	System:   "You are concise.",
	Messages: []ai.Message{ai.UserText("Name three primary colors.")},
})
resp.Text()
resp.ToolCalls()
resp.Usage
```

`Stream` returns `iter.Seq2[ai.Chunk, error]`: text deltas as chunks with
`Text`, a tool call as a chunk with `ToolCall`, and a final chunk with `Done`
and `Usage`.

```go
for chunk, err := range c.Stream(ctx, req) {
	if err != nil {
		return err
	}
	fmt.Print(chunk.Text)
}
```

## Structured output

`ai.Request.Format` maps onto the provider's own `responseMimeType`, so a request for JSON
is enforced by the provider rather than merely asked for:

```go
resp, err := c.Generate(ctx, &ai.Request{
	Model:    "the-model",
	Messages: []ai.Message{ai.UserText("Draft SEO fields for this article.")},
	Format: &ai.Format{
		Type:   ai.FormatJSONSchema,
		Name:   "seo",
		Schema: schema,
	},
})

var seo SEO
err = resp.JSON(&seo)
```

`ai.FormatJSON` sets `responseMimeType` to `application/json`;
`ai.FormatJSONSchema` sends the schema as `responseJsonSchema` beside it.

The API offers two schema fields and they are mutually exclusive.
`responseSchema` takes a subset of **OpenAPI 3.0**, which agrees with JSON
Schema only for the simplest shapes - `$ref`, `oneOf`, `$defs` and
`additionalProperties` are where the two part ways. `ai.Format.Schema` is a
JSON Schema, so the driver uses `responseJsonSchema`, the field that speaks
that dialect. `GenerationConfig.ResponseSchema` remains available for callers
who build a `GenerateRequest` themselves and mean OpenAPI.


`ai.Response.Format` is `ai.FormatNative`: this provider enforces every shape
it accepts. Which models support schema mode is the provider's business - there
is no capability table here, so an unsupported pairing is reported by the
provider itself.

## Native generateContent

For Gemini-only options build a `GenerateRequest` and call `GenerateContent` or
`StreamGenerateContent`:

```go
resp, err := c.GenerateContent(ctx, gemini.ModelGemini25Flash, &gemini.GenerateRequest{
	Contents: []gemini.Content{
		{Role: "user", Parts: []gemini.Part{{Text: "List two colors."}}},
	},
	GenerationConfig: &gemini.GenerationConfig{
		Temperature:      ptr(0.2),
		MaxOutputTokens:  256,
		ResponseMIMEType: "application/json",
		ResponseSchema:   json.RawMessage(`{"type":"array","items":{"type":"string"}}`),
	},
})
resp.Text()
```

Roles are `"user"` and `"model"`; the system prompt lives in
`SystemInstruction`. A `Part` holds exactly one of `Text`, `InlineData`,
`FileData`, `FunctionCall` or `FunctionResponse`.

## Tools and tool results

Tools use the shared `ai.Tool` type; `ToolChoice` maps to Gemini's function
calling mode (`ToolAuto` -> `AUTO`, `ToolNone` -> `NONE`, `ToolRequired` ->
`ANY`).

Gemini matches a tool result to its call by function name rather than by an ID.
The driver hides this: a returned `ai.ToolUse` carries the function name as its
`ID`, so replying with an `ai.ToolResult` whose `ID` is that same value routes
the result correctly.

```go
resp, _ := c.Generate(ctx, &ai.Request{
	Model:    gemini.ModelGemini25Flash,
	Messages: []ai.Message{ai.UserText("Weather in Kyiv?")},
	Tools: []ai.Tool{{
		Name:        "get_weather",
		Description: "Get the current weather for a city.",
		Schema:      json.RawMessage(`{"type":"object","properties":{"city":{"type":"string"}}}`),
	}},
})

for _, call := range resp.ToolCalls() {
	// ... run the tool ...
	req.Messages = append(req.Messages, ai.Message{
		Role:  ai.RoleTool,
		Parts: []ai.Part{ai.ToolResult{ID: call.ID, Content: `{"tempC":21}`}},
	})
}
```

A tool result whose `Content` is a JSON object is passed through; any other
string is wrapped as `{"result": "..."}` (or `{"error": "..."}` when
`IsError` is set).

## Images and system prompts

Inline image bytes become an `inlineData` part; an `ai.Image` with a `URL`
becomes a `fileData` reference. System text from the `System` field or a
`RoleSystem` message is merged into the request's system instruction.

```go
ai.Message{Role: ai.RoleUser, Parts: []ai.Part{
	ai.Text{Text: "What is in this image?"},
	ai.Image{MIME: "image/png", Data: pngBytes},
}}
```

## Embeddings

```go
vecs, err := c.Embed(ctx, "text-embedding-004", "hello", "world")
// or a single request with task type and dimensions:
e, err := c.EmbedContent(ctx, "text-embedding-004", &gemini.EmbedRequest{
	Content:              gemini.Content{Parts: []gemini.Part{{Text: "hello"}}},
	TaskType:             "RETRIEVAL_DOCUMENT",
	OutputDimensionality: 256,
})
e.Values
```

## Token counting

```go
n, err := c.CountTokens(ctx, gemini.ModelGemini25Flash, &gemini.GenerateRequest{
	Contents: []gemini.Content{
		{Role: "user", Parts: []gemini.Part{{Text: "hello there"}}},
	},
})
```

## Models

```go
models, err := c.Models(ctx)
m, err := c.GetModel(ctx, gemini.ModelGemini25Flash)
m.InputTokenLimit
```

## Hosted web search

`ai.Request.Hosted` maps onto grounding with Google Search, sent as its own
entry of the tools list beside your function declarations:

```go
resp, err := c.Generate(ctx, &ai.Request{
	Model:    gemini.ModelGemini25Flash,
	Messages: []ai.Message{ai.UserText("What shipped this week?")},
	Hosted:   []ai.Hosted{{Kind: ai.HostedWebSearch}},
})
for _, s := range resp.Citations() {
	fmt.Println(s.URL, resp.Text()[s.StartByte:s.EndByte])
}
```

The search runs on the provider's side, so nothing new reaches a tool loop.

Sources arrive as `ai.Citation` values on the text they support, and this
provider is unusual in reporting *where*: its offsets are byte offsets, so
`StartByte` and `EndByte` cut `ai.Text.Text` exactly and you can show which
sentence each source backs. It never reports the source's own words, so
`CitedText` stays empty here. A range that does not fall inside its part is
dropped rather than reported wrong.

A stream is the exception: it carries the sources without a range, because the
provider measures a range against the finished answer and a stream has none
yet.

`ai.Response.Hosted` reports whether grounding ran and how many queries it
issued, which is the unit this provider meters.

Grounding takes no settings at all here - the provider decides how many
searches to run and over what - so a request that sets `ai.HostedWeb.MaxUses`,
either domain list or a region is `ai.ErrNoHosted` rather than a search that
quietly ignores it.

A search and a structured `ai.Format` cannot be asked for in one call: this
provider enforces a format by constraining what the model may emit, and that
does not hold together with grounding. The pair is `ai.ErrFormatWithHosted`
before the request leaves. Two calls do work: one that searches and answers in
prose, one without `Hosted` that reshapes it into the schema.

## Options and errors

Options: `WithBaseURL`, `WithHTTPClient`, `WithTimeout`, `WithMaxRetries`,
`WithHeader`.

A non-success response becomes an `*ai.APIError` with `Status`, `Type` (the
Gemini status string, such as `INVALID_ARGUMENT`), `Message` and the raw body:

```go
var apiErr *ai.APIError
if errors.As(err, &apiErr) && apiErr.Status == http.StatusTooManyRequests {
	// back off
}
```

Requests missing a model or messages fail before the network with
`ai.ErrNoModel` or `ai.ErrNoMessages`.

When Gemini blocks a prompt on safety grounds it returns HTTP 200 with no
candidates and a block reason. The driver turns that into an `*ai.APIError`
(status 400) whose message carries the block reason, so a blocked prompt is a
normal error rather than a silent empty response.
