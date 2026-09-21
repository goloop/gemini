# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [1.2.1] - 2026-09-21

Patch release.

### Fixed
- `Stream` no longer turns a cut-off or broken stream into a success. An event
  that did not parse as JSON was skipped, and the end of the stream was
  reported as `Done` whether or not the generation had finished, so a stream
  made of nothing but a broken event, an empty stream, or one cut off after a
  text delta all came back as a completed answer with no error. A stream now
  ends properly only when a candidate carries a `finishReason`; anything else
  is `io.ErrUnexpectedEOF`, and an event that is not valid JSON is reported
  as an error. `StreamGenerateContent` reports a broken event the same way.

### Changed
- Depends on `ai` v1.2.1, whose `SSEEvents` bounds the size of one event, so a
  stream that never ends an event cannot make the reader grow without limit.


Minor release: image generation with Imagen. Additive.

### Added
- `GenerateImage` generates images through the Imagen predict endpoint, in the
  same shape as the other goloop image drivers: `ImageData.Bytes` decodes the
  inline result, and the request, response and error names match. Until now the
  driver did text and embeddings only.
- Imagen works in aspect ratios, not pixels, so `ImageRequest` offers both:
  `Size` ("WxH") is mapped to the nearest supported ratio for cross-provider
  code, and `AspectRatio` sets Imagen's native knob directly; `AspectRatio`
  wins when both are set. Google does not honor exact pixels, so a size snaps to
  the nearest ratio rather than being rejected.
- `ModelImagen4` and `ModelImagen3` model constants (verify against Google's
  current catalog before pinning - Google versions and retires these by date).
- `Capabilities.Images` is true: a UI can ask `ai.SupportsImages(client)` to
  decide whether to offer image generation.

## [1.1.1] - 2026-08-11

Patch release.

### Documentation
- The reference covers the driver's `Capabilities` and the translation of
  model-level 400 refusals into `ai.ErrNoHosted`/`ai.ErrNoFormat`, which until
  now lived only in the godoc.

## [1.1.0] - 2026-08-11

Minor release, on `ai` v1.1.0.

### Added
- `Capabilities` describes what this driver can be asked for: which hosted
  capabilities it runs, which `ai.HostedWeb` settings it can express, and
  whether a search survives in the same call as a structured format. It is a
  hint for the decision taken before a call, never a substitute for handling
  `ai.ErrNoHosted` - support also depends on the model, the account and the
  region. A test pins the table against what the driver actually refuses, so
  it stays honest mechanically rather than by discipline.
- `WithFormat` is reported as unavailable, which is the honest answer: the
  constraint that enforces a schema does not hold together with grounding, so
  a feature that needs both takes two calls. Grounding itself accepts no
  settings, and the table says so field by field.
- A provider refusal that means "this model cannot do that" is now translated
  into the matching sentinel, so an application degrades with one `errors.Is`
  whether the driver knew in advance or learned from a 400. The provider's own
  `ai.APIError` is wrapped, not replaced, and stays reachable with `errors.As`.
  The rules are deliberately narrow - only a 400, only a capability the caller
  asked for, only an error naming that exact feature - because mistaking a
  genuinely bad request for a missing feature would retry it forever.

## [1.0.0] - 2026-08-11

First stable release, on `ai` v1.0.0.

### Added
- `ai.Request.Hosted` maps onto grounding with Google Search, sent as its own
  entry of the tools list beside the caller's function declarations.
- Sources come back as `ai.Citation` values on the text they support. This
  provider is unusual in reporting where: its offsets are byte offsets, so
  `StartByte` and `EndByte` cut `ai.Text.Text` exactly and a caller can show
  which sentence each source backs. A range that does not fall inside its part
  is dropped rather than reported wrong.
- `ai.Response.Hosted` reports whether grounding ran and how many queries it
  issued, which is the unit this provider meters.
- A stream carries the sources once each, without a range: the provider
  measures a range against the finished answer, and a stream has none yet.
- `GroundingMetadata` and the types under it are exposed on `Candidate`.

### Changed
- Grounding takes no settings here, so a request that sets
  `ai.HostedWeb.MaxUses`, either domain list or a region is `ai.ErrNoHosted`
  rather than a search that quietly ignores what it was told.
- A search and a structured `ai.Format` cannot be asked for in one call: this
  provider enforces a format by constraining what the model may emit, and that
  does not hold together with grounding. The pair is `ai.ErrFormatWithHosted`
  before the request leaves, rather than JSON that quietly does not match.

## [0.3.0] - 2026-08-05

### Added
- `GenerationConfig.ResponseJSONSchema`, the API's `responseJsonSchema` field,
  which takes a JSON Schema. The existing `ResponseSchema` takes a subset of
  OpenAPI 3.0; the two are mutually exclusive.

### Fixed
- `ai.FormatJSONSchema` now sends the schema as `responseJsonSchema` rather
  than `responseSchema`. `ai.Format.Schema` is a JSON Schema, and the field it
  was going into speaks OpenAPI 3.0 - the dialects agree for a schema of plain
  types, properties and required fields, which is why this worked, and stop
  agreeing at `$ref`, `oneOf`, `$defs` and `additionalProperties`. A schema
  using any of those was rejected or read as something else.

## [0.2.2] - 2026-08-05

### Documentation
- The package documentation describes how `ai.Request.Format` reaches this
  provider, so it is on the first page a reader sees rather than only in the
  reference.

## [0.2.1] - 2026-08-05

Released by mistake: it carries this changelog entry and nothing else. The
documentation it describes landed in 0.2.2.

## [0.2.0] - 2026-08-05

### Added
- `ai.Request.Format` is mapped onto the provider's own `responseMimeType`, so a request
  for JSON is enforced by the provider rather than merely asked for, and
  `ai.Response.JSON` decodes the reply. `Response.Format` reports
  `ai.FormatNative`. Until now only this package's native request type could
  ask for JSON, so callers going through the provider-agnostic interface had
  to strip code fences from the reply by hand.
  `ai.FormatJSONSchema` sends the schema as `responseSchema` beside it.

### Changed
- Requires `github.com/goloop/ai` v0.4.0.

## [0.1.2] - 2026-07-10

### Documentation
- `DOC.md`/`DOC.UK.md` document blocked-prompt handling: a safety block (HTTP
  200 with no candidates) is surfaced as an `*ai.APIError`.

## [0.1.1] - 2026-07-10

### Fixed
- Repeated calls to the same function in one turn now get distinct tool-call
  IDs (the name plus a counter), so callers can tell them apart; each still
  resolves back to the function name when the results are sent in.
- A prompt blocked by a safety filter (HTTP 200 with no candidates and a
  `blockReason`) now returns an `*ai.APIError` instead of an empty response.

### Changed
- Require `goloop/ai` v0.2.0 (500 no longer retried; jittered backoff).

## [0.1.0] - 2026-07-09

Initial release, built on the `github.com/goloop/ai` interface.

### Added
- `Client` implementing `ai.Client`: `Generate` and streaming `Stream` over
  generateContent, with tool use, multimodal image input and system
  instructions.
- Native `GenerateContent` and `StreamGenerateContent` exposing the full
  generation config, tool config and response schema.
- Embeddings (`EmbedContent`, `Embed`), token counting (`CountTokens`) and
  models (`Models`, `GetModel`).
- Functional options: `WithBaseURL`, `WithHTTPClient`, `WithTimeout`,
  `WithMaxRetries`, `WithHeader`.
- Retries on 429 and 5xx with backoff; normalized `*ai.APIError` errors.
