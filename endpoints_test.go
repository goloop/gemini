package gemini

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestEmbed(t *testing.T) {
	c, done := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, ":batchEmbedContents") {
			t.Errorf("path = %q", r.URL.Path)
		}
		io.WriteString(w, `{"embeddings":[{"values":[0.1,0.2]},{"values":[0.3,0.4]}]}`)
	})
	defer done()

	vecs, err := c.Embed(context.Background(), "text-embedding-004", "a", "b")
	if err != nil {
		t.Fatal(err)
	}
	if len(vecs) != 2 || vecs[0][0] != 0.1 || vecs[1][1] != 0.4 {
		t.Errorf("vecs = %+v", vecs)
	}
}

func TestEmbedContent(t *testing.T) {
	c, done := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, ":embedContent") {
			t.Errorf("path = %q", r.URL.Path)
		}
		io.WriteString(w, `{"embedding":{"values":[1,2,3]}}`)
	})
	defer done()

	e, err := c.EmbedContent(context.Background(), "text-embedding-004", &EmbedRequest{
		Content: Content{Parts: []Part{{Text: "hi"}}},
	})
	if err != nil || len(e.Values) != 3 {
		t.Fatalf("embed: %v %+v", err, e)
	}
}

func TestCountTokens(t *testing.T) {
	c, done := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, ":countTokens") {
			t.Errorf("path = %q", r.URL.Path)
		}
		io.WriteString(w, `{"totalTokens":7}`)
	})
	defer done()

	n, err := c.CountTokens(context.Background(), "m", &GenerateRequest{
		Contents: []Content{{Role: "user", Parts: []Part{{Text: "hello there"}}}},
	})
	if err != nil || n != 7 {
		t.Fatalf("count: %v %d", err, n)
	}
}

func TestModels(t *testing.T) {
	c, done := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/models/gemini-2.5-flash"):
			io.WriteString(w, `{"name":"models/gemini-2.5-flash","inputTokenLimit":1000000}`)
		default:
			io.WriteString(w, `{"models":[{"name":"models/gemini-2.5-flash"}]}`)
		}
	})
	defer done()

	ctx := context.Background()
	if models, err := c.Models(ctx); err != nil || len(models) != 1 {
		t.Fatalf("models: %v %+v", err, models)
	}
	m, err := c.GetModel(ctx, "gemini-2.5-flash")
	if err != nil || m.InputTokenLimit != 1000000 {
		t.Fatalf("get model: %v %+v", err, m)
	}
}

func TestGenerateContentNative(t *testing.T) {
	c, done := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"candidates":[{"content":{"parts":[{"text":"hi"}]}}]}`)
	})
	defer done()

	resp, err := c.GenerateContent(context.Background(), "m", &GenerateRequest{
		Contents: []Content{{Role: "user", Parts: []Part{{Text: "hi"}}}},
	})
	if err != nil || resp.Text() != "hi" {
		t.Fatalf("generate: %v %+v", err, resp)
	}
}

func TestStreamGenerateContentNative(t *testing.T) {
	events := []string{
		`data: {"candidates":[{"content":{"parts":[{"text":"a"}]}}]}`, ``,
		`data: {"candidates":[{"content":{"parts":[{"text":"b"}]}}]}`, ``,
	}
	c, done := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		for _, line := range events {
			io.WriteString(w, line+"\n")
		}
	})
	defer done()

	var text strings.Builder
	for gr, err := range c.StreamGenerateContent(context.Background(), "m", &GenerateRequest{
		Contents: []Content{{Role: "user", Parts: []Part{{Text: "hi"}}}},
	}) {
		if err != nil {
			t.Fatal(err)
		}
		text.WriteString(gr.Text())
	}
	if text.String() != "ab" {
		t.Errorf("text = %q", text.String())
	}
}
