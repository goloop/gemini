package gemini

import (
	"context"
	"errors"
	"io"
	"net/http"
	"testing"

	"github.com/goloop/ai"
)

// streamOf serves the given SSE lines and runs Stream over them, returning
// the text handed out, whether Done was reported and the final error.
func streamOf(t *testing.T, events []string) (text string, doneSeen bool, err error) {
	t.Helper()
	c, done := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		for _, line := range events {
			io.WriteString(w, line+"\n")
		}
	})
	defer done()

	for chunk, e := range c.Stream(context.Background(), &ai.Request{
		Model: "m", Messages: []ai.Message{ai.UserText("hi")},
	}) {
		if e != nil {
			return text, doneSeen, e
		}
		text += chunk.Text
		if chunk.Done {
			doneSeen = true
		}
	}
	return text, doneSeen, nil
}

// An event that is not JSON is a broken stream, not something to skip: a
// stream made of nothing but a broken event must not come back as Done.
func TestStreamBrokenEventIsAnError(t *testing.T) {
	_, doneSeen, err := streamOf(t, []string{`data: {broken`, ``})
	if err == nil {
		t.Error("a broken event was skipped without an error")
	}
	if doneSeen {
		t.Error("a stream with a broken event reported Done")
	}
}

// A stream that ends before any candidate carries a finishReason was cut
// off, and is reported as such rather than as a shorter answer.
func TestStreamCutOffAfterText(t *testing.T) {
	text, doneSeen, err := streamOf(t, []string{
		`data: {"candidates":[{"content":{"parts":[{"text":"Hel"}]}}]}`, ``,
		// connection ends here - no finishReason
	})
	if text != "Hel" {
		t.Errorf("text = %q, want the delta that did arrive", text)
	}
	if doneSeen {
		t.Error("a cut-off stream reported Done")
	}
	if !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Errorf("err = %v, want ErrUnexpectedEOF", err)
	}
}

// An empty stream is a cut-off stream too, not an empty success.
func TestStreamEmptyIsAnError(t *testing.T) {
	_, doneSeen, err := streamOf(t, nil)
	if doneSeen {
		t.Error("an empty stream reported Done")
	}
	if !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Errorf("err = %v, want ErrUnexpectedEOF", err)
	}
}

// The native stream reports a broken event as well instead of skipping it.
func TestStreamGenerateContentBrokenEventIsAnError(t *testing.T) {
	c, done := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "data: {broken\n\n")
	})
	defer done()

	var gotErr error
	for _, err := range c.StreamGenerateContent(context.Background(), "m", &GenerateRequest{}) {
		if err != nil {
			gotErr = err
			break
		}
	}
	if gotErr == nil {
		t.Error("a broken event was skipped without an error")
	}
}
