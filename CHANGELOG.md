# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

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
