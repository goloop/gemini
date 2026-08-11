package gemini

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/goloop/ai"
)

// mustGeminiRequest builds the native request or fails the test.
func mustGeminiRequest(t *testing.T, req *ai.Request) *GenerateRequest {
	t.Helper()
	gr, err := geminiRequest(req)
	if err != nil {
		t.Fatalf("geminiRequest() error = %v", err)
	}
	return gr
}

func askToSearch(h ...ai.Hosted) *ai.Request {
	return &ai.Request{
		Model:    "the-model",
		Messages: []ai.Message{ai.UserText("What happened today?")},
		Hosted:   h,
	}
}

func TestHostedWebSearchReachesTheRequest(t *testing.T) {
	gr := mustGeminiRequest(t, askToSearch(ai.Hosted{Kind: ai.HostedWebSearch}))

	if len(gr.Tools) != 1 || gr.Tools[0].GoogleSearch == nil {
		t.Fatalf("Tools = %+v, want the provider's search tool", gr.Tools)
	}

	// The tool is an empty object on the wire, not an absent one.
	body, err := json.Marshal(gr.Tools[0])
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != `{"googleSearch":{}}` {
		t.Errorf("tool = %s", body)
	}
}

// The caller's functions and the provider's search are separate entries of the
// same list, and the tool choice keeps referring to the caller's.
func TestHostedIsItsOwnToolEntry(t *testing.T) {
	req := askToSearch(ai.Hosted{Kind: ai.HostedWebSearch})
	req.Tools = []ai.Tool{{Name: "lookup"}}

	gr := mustGeminiRequest(t, req)
	if len(gr.Tools) != 2 {
		t.Fatalf("Tools = %+v, want two entries", gr.Tools)
	}
	if len(gr.Tools[0].FunctionDeclarations) != 1 {
		t.Errorf("first entry = %+v, want the caller's functions", gr.Tools[0])
	}
	if gr.Tools[1].GoogleSearch == nil {
		t.Errorf("second entry = %+v, want the search tool", gr.Tools[1])
	}
}

func TestNoHostedLeavesTheRequestAlone(t *testing.T) {
	if gr := mustGeminiRequest(t, askToSearch()); gr.Tools != nil {
		t.Errorf("Tools = %+v, want none", gr.Tools)
	}
}

// This provider offers nothing to narrow a search with, so a caller who asked
// for a narrower one is told, rather than given a wider one.
func TestHostedRejectsSettingsThisProviderHasNoPlaceFor(t *testing.T) {
	tests := []struct {
		name string
		web  ai.HostedWeb
	}{
		{"use limit", ai.HostedWeb{MaxUses: 2}},
		{"allowed domains", ai.HostedWeb{AllowDomains: []string{"a.example"}}},
		{"blocked domains", ai.HostedWeb{BlockDomains: []string{"b.example"}}},
		{"region", ai.HostedWeb{Region: "UA"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			web := tt.web
			_, err := geminiRequest(askToSearch(ai.Hosted{
				Kind: ai.HostedWebSearch,
				Web:  &web,
			}))
			if !errors.Is(err, ai.ErrNoHosted) {
				t.Errorf("geminiRequest() error = %v, want ErrNoHosted", err)
			}
		})
	}
}

// A schema and a search do not hold together here, and saying so beats letting
// the provider answer with JSON that does not match.
func TestHostedWithAFormatIsRefused(t *testing.T) {
	for _, f := range []*ai.Format{
		{Type: ai.FormatJSON},
		{Type: ai.FormatJSONSchema, Schema: json.RawMessage(`{"type":"object"}`)},
	} {
		req := askToSearch(ai.Hosted{Kind: ai.HostedWebSearch})
		req.Format = f
		if _, err := geminiRequest(req); !errors.Is(err, ai.ErrFormatWithHosted) {
			t.Errorf("geminiRequest() error = %v, want ErrFormatWithHosted", err)
		}
	}
}

// Asking for plain text alongside a search is not asking for a format at all.
func TestHostedWithTextFormatIsFine(t *testing.T) {
	req := askToSearch(ai.Hosted{Kind: ai.HostedWebSearch})
	req.Format = &ai.Format{Type: ai.FormatText}
	if _, err := geminiRequest(req); err != nil {
		t.Errorf("geminiRequest() error = %v", err)
	}
}

// groundedReply is the shape a grounded answer takes: sources in one list, and
// the stretch of the answer each supports in another that points into it.
const groundedReply = `{
  "modelVersion": "the-model",
  "candidates": [{
    "content": {"role": "model", "parts": [{"text": "Сьогодні тепло."}]},
    "finishReason": "STOP",
    "groundingMetadata": {
      "webSearchQueries": ["погода сьогодні", "прогноз"],
      "groundingChunks": [
        {"web": {"uri": "https://example.org/a", "title": "A"}},
        {"web": {"uri": "https://example.org/b", "title": "B"}}
      ],
      "groundingSupports": [
        {"segment": {"startIndex": 0, "endIndex": 16, "text": "Сьогодні"},
         "groundingChunkIndices": [0, 1]}
      ]
    }
  }],
  "usageMetadata": {"promptTokenCount": 4, "candidatesTokenCount": 6}
}`

func TestGenerateReportsGroundingAndCitations(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(groundedReply))
		}))
	defer srv.Close()

	c := New("k", WithBaseURL(srv.URL))
	resp, err := c.Generate(context.Background(),
		askToSearch(ai.Hosted{Kind: ai.HostedWebSearch}))
	if err != nil {
		t.Fatal(err)
	}

	// Two queries were issued, which is what this provider meters.
	want := ai.HostedReport{
		Kind: ai.HostedWebSearch, Mode: ai.HostedNative, Calls: 2,
	}
	if len(resp.Hosted) != 1 || resp.Hosted[0] != want {
		t.Errorf("Hosted = %+v, want %+v", resp.Hosted, want)
	}

	cs := resp.Citations()
	if len(cs) != 2 {
		t.Fatalf("Citations() = %+v, want two", cs)
	}
	if cs[0].URL != "https://example.org/a" || cs[1].URL != "https://example.org/b" {
		t.Errorf("citations = %+v", cs)
	}

	// The range is a byte range into the answer, and "Сьогодні" is sixteen
	// bytes: a provider counting in something else would put the boundary
	// mid-character.
	if cs[0].StartByte != 0 || cs[0].EndByte != 16 {
		t.Errorf("range = %d..%d, want 0..16", cs[0].StartByte, cs[0].EndByte)
	}
	text := resp.Text()
	if text[cs[0].StartByte:cs[0].EndByte] != "Сьогодні" {
		t.Errorf("the range does not cut the answer where it should: %q",
			text[cs[0].StartByte:cs[0].EndByte])
	}

	// This provider reports the stretch of our own answer, never the source's
	// own words, so the field that means the source's words stays empty.
	if cs[0].CitedText != "" {
		t.Errorf("CitedText = %q, want it empty for this provider", cs[0].CitedText)
	}
}

// A range that does not fall inside the part it is attached to is dropped
// rather than reported: a wrong range points at the wrong words, which is
// worse than none.
func TestOutOfRangeSupportLosesItsRange(t *testing.T) {
	gm := &GroundingMetadata{
		GroundingChunks: []GroundingChunk{
			{Web: &GroundingWeb{URI: "https://example.org"}},
		},
		GroundingSupports: []GroundingSupport{{
			Segment:               &Segment{StartIndex: 0, EndIndex: 500},
			GroundingChunkIndices: []int{0},
		}},
	}

	cs := citationsFor(gm, 0, 0, 10)
	if len(cs) != 1 {
		t.Fatalf("citations = %+v, want one", cs)
	}
	if cs[0].StartByte != 0 || cs[0].EndByte != 0 {
		t.Errorf("range = %d..%d, want it dropped", cs[0].StartByte, cs[0].EndByte)
	}
	if cs[0].URL != "https://example.org" {
		t.Errorf("the source itself was lost: %+v", cs[0])
	}
}

// Sources with no supports at all still reach the caller: an answer nobody can
// check is the thing this exists to prevent.
func TestGroundingWithoutSupportsStillReportsSources(t *testing.T) {
	gm := &GroundingMetadata{
		GroundingChunks: []GroundingChunk{
			{Web: &GroundingWeb{URI: "https://example.org", Title: "A"}},
		},
	}
	cs := citationsFor(gm, 0, 0, 10)
	if len(cs) != 1 || cs[0].URL != "https://example.org" {
		t.Errorf("citations = %+v", cs)
	}
}

func TestGenerateReportsASkippedSearch(t *testing.T) {
	const reply = `{"candidates":[{"content":{"parts":[{"text":"I know."}]},
	  "finishReason":"STOP"}]}`

	srv := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(reply))
		}))
	defer srv.Close()

	c := New("k", WithBaseURL(srv.URL))

	resp, err := c.Generate(context.Background(),
		askToSearch(ai.Hosted{Kind: ai.HostedWebSearch}))
	if err != nil {
		t.Fatal(err)
	}
	want := ai.HostedReport{Kind: ai.HostedWebSearch, Mode: ai.HostedSkipped}
	if len(resp.Hosted) != 1 || resp.Hosted[0] != want {
		t.Errorf("Hosted = %+v, want %+v", resp.Hosted, want)
	}

	_, err = c.Generate(context.Background(), askToSearch(ai.Hosted{
		Kind:   ai.HostedWebSearch,
		Policy: ai.HostedRequired,
	}))
	if !errors.Is(err, ai.ErrHostedRequired) {
		t.Errorf("Generate() error = %v, want ErrHostedRequired", err)
	}
}

func TestStreamReportsSourcesOnceAndCountsSearches(t *testing.T) {
	// The provider repeats the whole metadata block as it grows, so the same
	// source arrives more than once.
	const events = `data: {"candidates":[{"content":{"parts":[{"text":"Сьогодні"}]},` +
		`"groundingMetadata":{"webSearchQueries":["a"],` +
		`"groundingChunks":[{"web":{"uri":"https://example.org/a","title":"A"}}]}}]}

data: {"candidates":[{"content":{"parts":[{"text":" тепло."}]},` +
		`"groundingMetadata":{"webSearchQueries":["a","b"],` +
		`"groundingChunks":[{"web":{"uri":"https://example.org/a","title":"A"}},` +
		`{"web":{"uri":"https://example.org/b","title":"B"}}]}}],` +
		`"usageMetadata":{"promptTokenCount":1,"candidatesTokenCount":2}}

`

	srv := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("content-type", "text/event-stream")
			_, _ = w.Write([]byte(events))
		}))
	defer srv.Close()

	c := New("k", WithBaseURL(srv.URL))

	var cites []ai.Citation
	var done ai.Chunk
	for chunk, err := range c.Stream(context.Background(),
		askToSearch(ai.Hosted{Kind: ai.HostedWebSearch})) {
		if err != nil {
			t.Fatal(err)
		}
		cites = append(cites, chunk.Citations...)
		if chunk.Done {
			done = chunk
		}
	}

	if len(cites) != 2 {
		t.Fatalf("citations = %+v, want each source once", cites)
	}
	if cites[0].URL != "https://example.org/a" || cites[1].URL != "https://example.org/b" {
		t.Errorf("citations = %+v", cites)
	}
	for _, c := range cites {
		if c.StartByte != 0 || c.EndByte != 0 {
			t.Errorf("a streamed citation carries a range: %+v", c)
		}
	}

	want := ai.HostedReport{
		Kind: ai.HostedWebSearch, Mode: ai.HostedNative, Calls: 2,
	}
	if len(done.Hosted) != 1 || done.Hosted[0] != want {
		t.Errorf("Hosted on the final chunk = %+v, want %+v", done.Hosted, want)
	}
}
