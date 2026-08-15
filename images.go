package gemini

import (
	"context"
	"encoding/base64"
	"errors"
	"strconv"
	"strings"
)

// Imagen model identifiers. Any model string is accepted; a constant is just a
// documented name to reach for. Google versions these by date and retires old
// ones, so verify against the current catalog before pinning one for a release.
const (
	ModelImagen4 = "imagen-4.0-generate-001"
	ModelImagen3 = "imagen-3.0-generate-002"
)

// Errors reported for an image request or its result. The names match the
// other goloop image drivers, so application code reads the same way whichever
// provider is behind it.
var (
	// ErrNoImageRequest is returned by GenerateImage for a nil request.
	ErrNoImageRequest = errors.New("gemini: image request is nil")

	// ErrNoImagePrompt is returned by GenerateImage when the prompt is empty.
	// Imagen has nothing to draw from without one.
	ErrNoImagePrompt = errors.New("gemini: image request has no prompt")

	// ErrNoImageBytes is returned by ImageData.Bytes when there are no image
	// bytes to decode. Imagen answers with inline base64, so this only happens
	// on an empty result.
	ErrNoImageBytes = errors.New("gemini: no image bytes")
)

// ImageRequest is an image generation request.
//
// The field names match the other goloop image drivers where the meaning is
// the same (Model, Prompt, N). Imagen works in aspect ratios rather than exact
// pixels, so two size knobs are offered: Size takes a "WxH" string for
// cross-provider code and is mapped to the nearest aspect ratio, while
// AspectRatio sets Imagen's native knob directly. When both are set,
// AspectRatio wins.
type ImageRequest struct {
	Model  string `json:"-"`
	Prompt string `json:"-"`

	// N is how many images to generate. Zero means one.
	N int `json:"-"`

	// AspectRatio is Imagen's native ratio, one of "1:1", "3:4", "4:3",
	// "9:16", "16:9". Empty leaves it to the provider.
	AspectRatio string `json:"-"`

	// Size is a "WxH" pixel string (for example "1536x1024"). Google does not
	// honor exact pixels, so it is mapped to the nearest supported aspect
	// ratio. AspectRatio, when set, takes precedence.
	Size string `json:"-"`
}

// ImageData is one generated image. Imagen returns bytes inline, so Bytes
// decodes them directly.
type ImageData struct {
	B64JSON  string `json:"bytesBase64Encoded"`
	MIMEType string `json:"mimeType,omitempty"`
}

// ImageResponse is an image generation response.
type ImageResponse struct {
	Data []ImageData

	// Usage mirrors the field on the other drivers. Imagen's predict endpoint
	// does not report token usage, so this is always nil here; it exists so
	// accounting code reads the same across providers.
	Usage *ImageUsage
}

// ImageUsage reports the tokens an image request consumed, in the shape the
// other goloop image drivers use. Imagen's predict endpoint reports none, so
// this is unused today; it keeps the type present for uniform accounting code.
type ImageUsage struct {
	TotalTokens  int `json:"total_tokens"`
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// Bytes returns the decoded image. Imagen delivers bytes inline, so this always
// has something to decode unless the result was empty ([ErrNoImageBytes]).
func (d ImageData) Bytes() ([]byte, error) {
	if d.B64JSON == "" {
		return nil, ErrNoImageBytes
	}
	return base64.StdEncoding.DecodeString(d.B64JSON)
}

// predictRequest is the wire shape of an Imagen predict call.
type predictRequest struct {
	Instances  []predictInstance `json:"instances"`
	Parameters predictParameters `json:"parameters"`
}

type predictInstance struct {
	Prompt string `json:"prompt"`
}

type predictParameters struct {
	SampleCount int    `json:"sampleCount,omitempty"`
	AspectRatio string `json:"aspectRatio,omitempty"`
}

// predictResponse is the wire shape of the predict result.
type predictResponse struct {
	Predictions []ImageData `json:"predictions"`
}

// GenerateImage generates images from a text prompt with Imagen, through the
// predict endpoint. Read each image with [ImageData.Bytes].
//
// The request is fitted first: Size is mapped to the nearest aspect ratio Imagen
// supports, unless AspectRatio names one directly. A nil or promptless request
// is refused before the network.
func (c *Client) GenerateImage(
	ctx context.Context,
	req *ImageRequest,
) (*ImageResponse, error) {
	if req == nil {
		return nil, ErrNoImageRequest
	}
	if strings.TrimSpace(req.Prompt) == "" {
		return nil, ErrNoImagePrompt
	}

	wire := predictRequest{
		Instances: []predictInstance{{Prompt: req.Prompt}},
		Parameters: predictParameters{
			SampleCount: req.N,
			AspectRatio: aspectRatioFor(req),
		},
	}

	var out predictResponse
	path := "/models/" + req.Model + ":predict"
	if err := c.postJSON(ctx, path, &wire, &out); err != nil {
		return nil, err
	}
	return &ImageResponse{Data: out.Predictions}, nil
}

// aspectRatioFor resolves the ratio to send: AspectRatio wins when set,
// otherwise Size is mapped to the nearest supported ratio, otherwise nothing
// is sent and the provider chooses.
func aspectRatioFor(req *ImageRequest) string {
	if req.AspectRatio != "" {
		return req.AspectRatio
	}
	if req.Size != "" {
		return nearestAspectRatio(req.Size)
	}
	return ""
}

// supportedAspectRatios is the set Imagen accepts, each with its numeric value
// for nearest-match. Google does not honor exact pixels, so a "WxH" size is
// snapped to whichever of these is closest rather than rejected: refusing a
// perfectly reasonable "1024x1024" would be hostile.
var supportedAspectRatios = []struct {
	name  string
	ratio float64
}{
	{"1:1", 1.0},
	{"3:4", 3.0 / 4.0},
	{"4:3", 4.0 / 3.0},
	{"9:16", 9.0 / 16.0},
	{"16:9", 16.0 / 9.0},
}

// nearestAspectRatio maps a "WxH" string to the closest supported ratio. An
// unparseable size falls back to no ratio (empty), letting the provider decide
// rather than erroring on a typo in a cosmetic field.
func nearestAspectRatio(size string) string {
	w, h, ok := parseSize(size)
	if !ok || h == 0 {
		return ""
	}
	target := float64(w) / float64(h)

	best := supportedAspectRatios[0]
	bestDist := abs(target - best.ratio)
	for _, r := range supportedAspectRatios[1:] {
		if d := abs(target - r.ratio); d < bestDist {
			best, bestDist = r, d
		}
	}
	return best.name
}

// parseSize splits a "WxH" string into its dimensions.
func parseSize(size string) (w, h int, ok bool) {
	parts := strings.SplitN(strings.ToLower(size), "x", 2)
	if len(parts) != 2 {
		return 0, 0, false
	}
	w, err1 := strconv.Atoi(strings.TrimSpace(parts[0]))
	h, err2 := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err1 != nil || err2 != nil {
		return 0, 0, false
	}
	return w, h, true
}

func abs(f float64) float64 {
	if f < 0 {
		return -f
	}
	return f
}
