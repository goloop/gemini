package gemini

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
)

// headers returns the headers common to every Gemini request. The API key is
// sent in the x-goog-api-key header rather than the query string.
func (c *Client) headers() http.Header {
	h := http.Header{}
	h.Set("x-goog-api-key", c.opts.APIKey)
	h.Set("content-type", "application/json")
	return h
}

// send performs a request against a path under the base URL and returns the
// response body and status code.
func (c *Client) send(
	ctx context.Context,
	method, path string,
	body []byte,
) ([]byte, int, error) {
	resp, err := c.opts.Do(ctx, method, c.opts.BaseURL+path, body, c.headers())
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, err
	}
	return data, resp.StatusCode, nil
}

// postJSON marshals in, POSTs it to path and unmarshals the response into out.
func (c *Client) postJSON(ctx context.Context, path string, in, out any) error {
	body, err := json.Marshal(in)
	if err != nil {
		return err
	}
	data, status, err := c.send(ctx, http.MethodPost, path, body)
	if err != nil {
		return err
	}
	if status != http.StatusOK {
		return parseError(status, data)
	}
	if out == nil {
		return nil
	}
	return json.Unmarshal(data, out)
}

// getJSON GETs path and unmarshals the response into out.
func (c *Client) getJSON(ctx context.Context, path string, out any) error {
	data, status, err := c.send(ctx, http.MethodGet, path, nil)
	if err != nil {
		return err
	}
	if status != http.StatusOK {
		return parseError(status, data)
	}
	return json.Unmarshal(data, out)
}
