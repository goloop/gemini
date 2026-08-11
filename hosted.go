package gemini

import (
	"fmt"

	"github.com/goloop/ai"
)

// hostedTools converts the capabilities a request asked for into this
// provider's own tool entries. They go in the same tools list as the caller's
// function declarations, each as its own entry.
//
// Grounding here takes no settings at all: this provider decides how many
// searches to run and over what. A caller who asked for a limit gets
// [ai.ErrNoHosted] rather than a search that ignores it, because the whole
// value of a constraint is that it held.
func hostedTools(req *ai.Request) ([]ToolDecls, error) {
	var out []ToolDecls

	for _, h := range req.Hosted {
		if h.Kind != ai.HostedWebSearch {
			return nil, fmt.Errorf("%w: %s", ai.ErrNoHosted, h.Kind)
		}
		if err := checkWeb(h.Web); err != nil {
			return nil, err
		}
		out = append(out, ToolDecls{GoogleSearch: &GoogleSearch{}})
	}

	return out, nil
}

// checkWeb rejects the search settings this provider has nowhere to put.
func checkWeb(w *ai.HostedWeb) error {
	if w == nil {
		return nil
	}
	switch {
	case w.MaxUses > 0:
		return fmt.Errorf("%w: search grounding has no use limit", ai.ErrNoHosted)
	case len(w.AllowDomains) > 0 || len(w.BlockDomains) > 0:
		return fmt.Errorf("%w: search grounding has no domain filter", ai.ErrNoHosted)
	case w.Region != "":
		return fmt.Errorf("%w: search grounding has no region", ai.ErrNoHosted)
	}
	return nil
}

// checkFormat rejects a request that asks for a schema and a search at once.
//
// This provider enforces a format by constraining what the model may emit, and
// that constraint and search grounding do not hold together: the request is
// either refused or answered without one of the two. Saying so before the call
// is the difference between a clear error and JSON that quietly does not match
// the schema. A caller who needs both makes two calls: one that searches and
// answers in prose, one without Hosted that reshapes it.
func checkFormat(req *ai.Request) error {
	if len(req.Hosted) == 0 || req.Format == nil {
		return nil
	}
	if req.Format.Type == ai.FormatText {
		return nil
	}
	return fmt.Errorf("%w: search grounding and a response schema cannot be "+
		"set on the same call", ai.ErrFormatWithHosted)
}

// hostedCalls reads how many searches the provider ran. It counts the queries
// it says it issued, which is the unit it meters. A grounded answer that
// reports no queries still proves a search happened, so it counts as one.
func hostedCalls(gm *GroundingMetadata) map[ai.HostedKind]int {
	if gm == nil {
		return nil
	}
	n := len(gm.WebSearchQueries)
	if n == 0 && len(gm.GroundingChunks) == 0 {
		return nil
	}
	if n == 0 {
		n = 1
	}
	return map[ai.HostedKind]int{ai.HostedWebSearch: n}
}

// citationsFor builds the sources for the text part at partIndex, which starts
// at startByte within the candidate's concatenated text.
//
// This provider reports sources and the stretch of the answer each one backs
// separately: the sources are a list, and every support names them by position
// in it. The offsets are byte offsets, which is what [ai.Citation] wants, but
// they are measured against the whole candidate, so they are rebased onto the
// part they land in. A support that does not land in this part belongs to
// another one.
func citationsFor(
	gm *GroundingMetadata,
	partIndex, startByte, length int,
) []ai.Citation {
	if gm == nil || len(gm.GroundingChunks) == 0 {
		return nil
	}

	var out []ai.Citation
	for _, s := range gm.GroundingSupports {
		start, end := 0, 0
		if s.Segment != nil {
			// PartIndex is absent on a single-part answer, where zero is also
			// the right answer, so it needs no special case.
			if s.Segment.PartIndex != partIndex {
				continue
			}
			start = s.Segment.StartIndex - startByte
			end = s.Segment.EndIndex - startByte

			// A range outside this part means the provider measured against
			// something other than assumed. Dropping the range is better than
			// reporting one that cuts the text in the wrong place, possibly
			// mid-character.
			if start < 0 || end > length || start > end {
				start, end = 0, 0
			}
		}

		for _, i := range s.GroundingChunkIndices {
			if i < 0 || i >= len(gm.GroundingChunks) {
				continue
			}
			web := gm.GroundingChunks[i].Web
			if web == nil {
				continue
			}
			// CitedText stays empty on purpose. This provider reports the
			// stretch of our own answer a source backs, not the words of the
			// source; the byte range already says that, and copying our text
			// into a field that means "what the source said" would be the
			// wrong claim in the one field a reader checks a citation with.
			out = append(out, ai.Citation{
				URL:       web.URI,
				Title:     web.Title,
				StartByte: start,
				EndByte:   end,
			})
		}
	}

	// Grounding with no support entries still found sources, and losing them
	// would leave an answer that cannot be checked. They are reported without
	// a range, which reads as "these back the whole part".
	if out == nil && len(gm.GroundingSupports) == 0 {
		for _, c := range gm.GroundingChunks {
			if c.Web == nil {
				continue
			}
			out = append(out, ai.Citation{URL: c.Web.URI, Title: c.Web.Title})
		}
	}

	return out
}
