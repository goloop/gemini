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
// # Structured output
//
// ai.Request.Format maps onto responseMimeType, with ai.FormatJSONSchema
// sending the schema as responseJsonSchema beside it, so a request for JSON is
// enforced rather than merely asked for. ai.Response.JSON decodes the reply.
// The API has a second schema field, responseSchema, which takes an OpenAPI
// 3.0 subset rather than JSON Schema; it stays available on GenerationConfig
// for callers who mean that dialect.
//
// # Hosted web search
//
// ai.Request.Hosted maps onto grounding with Google Search, which goes in the
// tools list as an entry of its own beside the caller's functions:
//
//	resp, err := c.Generate(ctx, &ai.Request{
//	    Model:    gemini.ModelGemini25Flash,
//	    Messages: []ai.Message{ai.UserText("What shipped this week?")},
//	    Hosted:   []ai.Hosted{{Kind: ai.HostedWebSearch}},
//	})
//	for _, c := range resp.Citations() { ... }
//
// The search runs on the provider's side, so nothing new reaches a tool loop.
// The sources come back as ai.Citation values on the text they support, and
// this provider is unusual in reporting where: its offsets are byte offsets,
// so Citation.StartByte and Citation.EndByte cut ai.Text.Text exactly, and a
// caller can show which sentence each source backs. It never reports the
// source's own words, so Citation.CitedText stays empty here.
//
// A stream is the exception: it carries the sources without a range. The
// provider measures a range against the finished answer, and there is no
// finished answer to measure against while it is still arriving.
//
// Grounding takes no settings at all here: the provider decides how many
// searches to run and over what. A request that sets ai.HostedWeb.MaxUses,
// either domain list or a region is ai.ErrNoHosted rather than a search that
// quietly ignores what it was told.
//
// A search and a structured Format cannot be asked for in the same call. This
// provider enforces a format by constraining what the model may emit, and that
// does not hold together with grounding, so a request that asks for both is
// ai.ErrFormatWithHosted before it leaves. Two calls do work: one that
// searches and answers in prose, one without Hosted that reshapes it into the
// schema.
//
// # Asking what this driver can do
//
// Capabilities describes this driver for the decision taken before a call:
// whether to offer a feature at all, and whether it needs one request or two.
//
//	if ai.SupportsHosted(c, ai.Hosted{Kind: ai.HostedWebSearch}) { ... }
//
// It is a hint and not a permission - support also depends on the model, the
// account and the region - so ai.ErrNoHosted and ai.ErrNoFormat remain the
// source of truth and a caller still handles them. What changes is that a
// refusal the provider only reports as a 400 now arrives as those same
// sentinels, wrapped around the original ai.APIError, so one errors.Is covers
// a limitation this driver knew in advance and one it learned over the wire.
//
// Gemini keys tool results by function name rather than by call ID, so this
// package synthesizes a unique ai.ToolUse ID per call (the function name plus a
// counter, so repeated calls to one function stay distinct) and resolves it
// back to the name on the way in. It depends only on goloop/ai and the standard
// library.
package gemini
