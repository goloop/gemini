package gemini

import "github.com/goloop/ai"

// DefaultBaseURL is the Gemini API base URL, including the version segment.
const DefaultBaseURL = "https://generativelanguage.googleapis.com/v1beta"

// Convenience model identifiers. Any model string is accepted; use Models to
// discover what the account can call.
const (
	ModelGemini25Pro       = "gemini-2.5-pro"
	ModelGemini25Flash     = "gemini-2.5-flash"
	ModelGemini25FlashLite = "gemini-2.5-flash-lite"
	ModelGemini20Flash     = "gemini-2.0-flash"
	ModelTextEmbedding004  = "text-embedding-004"
)

// Client is a Gemini API client. It implements [ai.Client] and adds the
// provider's native endpoints.
type Client struct {
	opts ai.Options
}

var _ ai.Client = (*Client)(nil)

// New returns a Client for the given API key. Shared options (WithBaseURL,
// WithHTTPClient, WithTimeout, WithMaxRetries, WithHeader) configure it.
func New(apiKey string, opts ...Option) *Client {
	s := settings{}
	for _, o := range opts {
		o(&s)
	}

	o := ai.NewOptions(apiKey, s.aiOpts...)
	if o.BaseURL == "" {
		o.BaseURL = DefaultBaseURL
	}

	return &Client{opts: o}
}
