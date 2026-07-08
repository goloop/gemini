[![deps.dev](https://img.shields.io/badge/deps.dev-insights-4c8dbc)](https://deps.dev/go/github.com%2Fgoloop%2Fgemini) [![License](https://img.shields.io/badge/license-MIT-brightgreen)](https://github.com/goloop/gemini/blob/master/LICENSE) [![License](https://img.shields.io/badge/godoc-YES-green)](https://pkg.go.dev/github.com/goloop/gemini) [![Stay with Ukraine](https://img.shields.io/static/v1?label=Stay%20with&message=Ukraine%20♥&color=ffD700&labelColor=0057B8&style=flat)](https://u24.gov.ua/)


# gemini

`gemini` is a Go client for the Google Gemini API. It implements the
`github.com/goloop/ai` interface, so it looks and works like every other goloop
AI provider, and exposes Gemini's native endpoints with their full options on
top.

## Features

- Content generation: `Generate` for a single response, `Stream` for
  token-by-token output through `iter.Seq2`.
- Tool use (function calling), multimodal image input and system instructions.
- Native `GenerateContent` and `StreamGenerateContent` with the full option set
  (generation config, response schema, tool config).
- Embeddings, token counting and model listing.
- Retries on 429 and 5xx with backoff; normalized, typed API errors.
- Depends only on `github.com/goloop/ai` and the standard library.

## Installation

```sh
go get github.com/goloop/gemini
```

## Quick start

```go
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/goloop/ai"
	"github.com/goloop/gemini"
)

func main() {
	c := gemini.New(os.Getenv("GEMINI_API_KEY"))

	resp, err := c.Generate(context.Background(), &ai.Request{
		Model:    gemini.ModelGemini25Flash,
		Messages: []ai.Message{ai.UserText("Say hello in one word.")},
	})
	if err != nil {
		panic(err)
	}
	fmt.Println(resp.Text())
}
```

## Streaming

```go
for chunk, err := range c.Stream(ctx, req) {
	if err != nil {
		break
	}
	fmt.Print(chunk.Text)
	if chunk.Done && chunk.Usage != nil {
		fmt.Printf("\n[%d in / %d out]\n",
			chunk.Usage.InputTokens, chunk.Usage.OutputTokens)
	}
}
```

## Tools, images and system prompts

Tools, images and system prompts use the same shared `ai` types as every other
provider (see the [reference](DOC.md)). Gemini keys tool results by function
name, so the driver uses the function name as the tool-call ID and resolves it
back automatically.

For Gemini-only options such as a JSON response schema, build a native
`GenerateRequest`:

```go
resp, _ := c.GenerateContent(ctx, gemini.ModelGemini25Flash, &gemini.GenerateRequest{
	Contents: []gemini.Content{{Role: "user", Parts: []gemini.Part{{Text: "List two colors."}}}},
	GenerationConfig: &gemini.GenerationConfig{
		ResponseMIMEType: "application/json",
	},
})
```

## Native endpoints

```go
c.Embed(ctx, "text-embedding-004", "hello", "world")
c.CountTokens(ctx, gemini.ModelGemini25Flash, req)
c.Models(ctx)
c.GetModel(ctx, gemini.ModelGemini25Flash)
```

## Documentation

Full reference: **[DOC.md](DOC.md)** (Ukrainian: **[DOC.UK.md](DOC.UK.md)**).

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md).

## License

MIT - see [LICENSE](LICENSE).
