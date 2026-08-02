# OpenAI Chat Completions JSON is the shared vocabulary; transport confined to a boundary

Despite the goal that the core be protocol- and transport-agnostic, the core's
conversation types (`Chat`, `Message`, `Tool`, `Response`, `Usage`) are the
OpenAI-compatible Chat Completions shapes, living in `internal/model` and imported by
both the core and the transport. We deliberately rejected building an abstract
intermediate representation with per-provider codecs: nearly every host we could switch
to (OpenAI, OpenRouter, vLLM, Ollama, llama.cpp) speaks the same wire format, so the
abstraction would cost ceremony for no current payoff. The genuine agnosticism we do
keep is that the core never touches HTTP — the endpoint, `http.Client`, and marshalling
live in a transport-boundary package implementing the one-method `Model` interface, so
the loop is testable with a fake that needs no network.

*Status: accepted*

*Considered Options:* abstract own-domain model + codec adapters per provider (rejected
— YAGNI for a learning harness whose hosts are all OpenAI-compatible).
