package gemini

import (
	"encoding/json"

	"github.com/goloop/ai"
)

// parseError turns a non-success response body into an *ai.APIError. Gemini
// reports errors as {"error":{"code","message","status"}}; status maps to the
// APIError Type and code, when numeric, is kept as a string.
func parseError(status int, body []byte) error {
	var w struct {
		Error struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
			Status  string `json:"status"`
		} `json:"error"`
	}
	_ = json.Unmarshal(body, &w)

	return &ai.APIError{
		Status:  status,
		Type:    w.Error.Status,
		Message: w.Error.Message,
		Raw:     append(json.RawMessage(nil), body...),
	}
}
