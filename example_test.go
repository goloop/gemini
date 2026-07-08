package gemini_test

import (
	"encoding/json"
	"fmt"

	"github.com/goloop/ai"
	"github.com/goloop/gemini"
)

func ExampleNew() {
	c := gemini.New("AIza...")
	_ = c // use c.Generate, c.Stream, c.GenerateContent, ...
	fmt.Println(gemini.ModelGemini25Flash)
	// Output: gemini-2.5-flash
}

// ExampleClient_Generate builds a request. Sending it needs a real API key, so
// this example only shows the shape.
func ExampleClient_Generate() {
	req := &ai.Request{
		Model: gemini.ModelGemini25Flash,
		Messages: []ai.Message{
			ai.UserText("Name the capital of France."),
		},
	}
	fmt.Println(req.Model, len(req.Messages))
	// Output: gemini-2.5-flash 1
}

// ExampleTool shows a tool definition passed with a request.
func ExampleTool() {
	tool := ai.Tool{
		Name:        "get_weather",
		Description: "Get the current weather for a city.",
		Schema:      json.RawMessage(`{"type":"object","properties":{"city":{"type":"string"}}}`),
	}
	fmt.Println(tool.Name)
	// Output: get_weather
}
