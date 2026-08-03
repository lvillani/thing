# OpenAI Chat Completions JSON is the shared vocabulary; transport confined to a boundary

- Status: accepted

## Context and Problem Statement

The goal is a core that is independent of protocol and transport. But nearly every host
we could switch to (OpenAI, OpenRouter, vLLM, Ollama, llama.cpp) speaks the same
OpenAI-compatible Chat Completions wire format. Building an abstract intermediate
representation with per-provider codecs would add ceremony for no current payoff.

## Decision

The core's conversation types (`Chat`, `Message`, `Tool`, `Response`, `Usage`) are the
OpenAI-compatible Chat Completions shapes. They live in `internal/model` and are
imported by both the core and the transport. The agnosticism we keep is that the core
never touches HTTP: the endpoint, `http.Client`, and marshalling live in a
transport-boundary package that implements the one-method `Model` interface. This makes
the loop testable with a fake that needs no network.

## Consequences

- The loop is testable without a network, using a fake `Model`.
- HTTP never leaks into the core.
- Rejected: an abstract own-domain model plus codec adapters per provider. It is YAGNI
  for a learning harness whose hosts are all OpenAI-compatible.
