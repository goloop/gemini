// Package gemini is a client for the Google Gemini API, built on the goloop/ai
// interface.
//
// The Client implements ai.Client, so Generate and Stream work the same as
// with any other goloop AI provider. On top of that it exposes Gemini's native
// endpoints and their full options: generateContent, streamGenerateContent,
// embeddings, token counting and model listing.
//
//	c := gemini.New(os.Getenv("GEMINI_API_KEY"))
//	resp, err := c.Generate(ctx, &ai.Request{
//	    Model:    gemini.ModelGemini25Flash,
//	    Messages: []ai.Message{ai.UserText("Say hello in one word.")},
//	})
//
// Gemini keys tool results by function name rather than by call ID, so this
// package synthesizes a unique ai.ToolUse ID per call (the function name plus a
// counter, so repeated calls to one function stay distinct) and resolves it
// back to the name on the way in. It depends only on goloop/ai and the standard
// library.
package gemini
