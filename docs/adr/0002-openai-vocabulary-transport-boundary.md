# OpenAI Chat Completions JSON is the shared vocabulary; transport confined to a boundary

- Status: accepted

## Context and Problem Statement

The goal is that the core be protocol- and transport-agnostic. Nearly every host we
could switch to (OpenAI, OpenRouter, vLLM, Ollama, llama.cpp) speaks the same
OpenAI-compatible Chat Completions wire format, so building an abstract intermediate
representation with per-provider codecs would cost ceremony for no current payoff.

## Decision

The core's conversation types (`Chat`, `Message`, `Tool`, `Response`, `Usage`) are the
OpenAI-compatible Chat Completions shapes, living in `internal/model` and imported by
both the core and the transport. The genuine agnosticism we keep is that the core never
touches HTTP: the endpoint, `http.Client`, and marshalling live in a transport-boundary
package implementing the one-method `Model` interface, so the loop is testable with a
fake that needs no network.

## Consequences

- The loop is testable without a network (fake `Model`), and no HTTP leaks into the
  core.
- An abstract own-domain model plus codec adapters per provider was rejected — YAGNI for
  a learning harness whose hosts are all OpenAI-compatible.
