package gemini

import (
	"context"
	"errors"
	"github.com/goloop/ai"
	"testing"
)

func TestCountTokensRejectsNilRequest(t *testing.T) {
	if _, err := New("").CountTokens(context.Background(), "test", nil); !errors.Is(err, ai.ErrNoRequest) {
		t.Fatalf("got %v", err)
	}
}
