package gemini

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/goloop/ai"
)

// imageClient builds a client whose predict calls hit the test handler.
func imageClient(t *testing.T, h http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return New("k", WithBaseURL(srv.URL))
}

// A prompt maps to the predict wire shape, the size maps to the nearest aspect
// ratio, and the inline bytes decode - the whole happy path.
func TestGenerateImage(t *testing.T) {
	var sent predictRequest
	var path string
	c := imageClient(t, func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &sent)
		io.WriteString(w, `{"predictions":[{"bytesBase64Encoded":"aGk=","mimeType":"image/png"}]}`)
	})

	resp, err := c.GenerateImage(context.Background(), &ImageRequest{
		Model: ModelImagen4, Prompt: "a watercolor cat", N: 2, Size: "1536x1024",
	})
	if err != nil {
		t.Fatal(err)
	}

	if !strings.HasSuffix(path, "/models/"+ModelImagen4+":predict") {
		t.Errorf("path = %q, want the predict endpoint", path)
	}
	if len(sent.Instances) != 1 || sent.Instances[0].Prompt != "a watercolor cat" {
		t.Errorf("instances = %+v", sent.Instances)
	}
	if sent.Parameters.SampleCount != 2 {
		t.Errorf("sampleCount = %d, want 2", sent.Parameters.SampleCount)
	}
	// 1536x1024 is 3:2, whose nearest supported ratio is 4:3.
	if sent.Parameters.AspectRatio != "4:3" {
		t.Errorf("aspectRatio = %q, want 4:3 (nearest to 3:2)", sent.Parameters.AspectRatio)
	}

	png, err := resp.Data[0].Bytes()
	if err != nil || string(png) != "hi" {
		t.Errorf("bytes = %q, %v", png, err)
	}
}

// AspectRatio, when set, wins over Size: the native knob is exact, the size is
// a convenience.
func TestAspectRatioWinsOverSize(t *testing.T) {
	var sent predictRequest
	c := imageClient(t, func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &sent)
		io.WriteString(w, `{"predictions":[{"bytesBase64Encoded":"aGk="}]}`)
	})

	_, err := c.GenerateImage(context.Background(), &ImageRequest{
		Model: ModelImagen4, Prompt: "x", Size: "1024x1024", AspectRatio: "16:9",
	})
	if err != nil {
		t.Fatal(err)
	}
	if sent.Parameters.AspectRatio != "16:9" {
		t.Errorf("aspectRatio = %q, want the explicit 16:9", sent.Parameters.AspectRatio)
	}
}

// The size-to-ratio mapping snaps to the nearest supported ratio, and an
// unparseable size falls back to nothing rather than erroring on a cosmetic
// field.
func TestNearestAspectRatio(t *testing.T) {
	cases := []struct {
		size, want string
	}{
		{"1024x1024", "1:1"},
		{"1536x1024", "4:3"}, // 3:2 -> nearest is 4:3
		{"1024x1536", "3:4"}, // 2:3 -> nearest is 3:4
		{"1920x1080", "16:9"},
		{"1080x1920", "9:16"},
		{"800x600", "4:3"},
		{"not-a-size", ""}, // falls back, provider decides
		{"", ""},
	}
	for _, c := range cases {
		if got := nearestAspectRatio(c.size); got != c.want {
			t.Errorf("nearestAspectRatio(%q) = %q, want %q", c.size, got, c.want)
		}
	}
}

// A nil or promptless request is refused before the network, with a clear
// sentinel.
func TestGenerateImageValidation(t *testing.T) {
	c := New("k")
	if _, err := c.GenerateImage(context.Background(), nil); !errors.Is(err, ErrNoImageRequest) {
		t.Errorf("nil request err = %v, want ErrNoImageRequest", err)
	}
	if _, err := c.GenerateImage(context.Background(),
		&ImageRequest{Model: ModelImagen4, Prompt: "  "}); !errors.Is(err, ErrNoImagePrompt) {
		t.Errorf("empty prompt err = %v, want ErrNoImagePrompt", err)
	}
}

// Bytes decodes inline base64 and reports an empty result rather than
// returning empty bytes silently.
func TestImageDataBytes(t *testing.T) {
	got, err := ImageData{B64JSON: "aGk="}.Bytes()
	if err != nil || string(got) != "hi" {
		t.Fatalf("Bytes() = %q, %v", got, err)
	}
	if _, err := (ImageData{}).Bytes(); !errors.Is(err, ErrNoImageBytes) {
		t.Errorf("empty Bytes() err = %v, want ErrNoImageBytes", err)
	}
}

// A predict error is classified the same way generateContent errors are: a
// non-200 becomes an ai.APIError through the shared parseError.
func TestGenerateImageError(t *testing.T) {
	c := imageClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		io.WriteString(w, `{"error":{"code":400,"message":"bad model","status":"INVALID_ARGUMENT"}}`)
	})

	_, err := c.GenerateImage(context.Background(),
		&ImageRequest{Model: "imagen-nope", Prompt: "x"})
	var apiErr *ai.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("err = %v, want an *ai.APIError", err)
	}
	if apiErr.Status != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", apiErr.Status)
	}
}

// The driver reports image support so a UI can offer the control.
func TestGeminiReportsImageSupport(t *testing.T) {
	if !ai.SupportsImages(New("k")) {
		t.Error("gemini should report image support")
	}
}
