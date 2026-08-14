<div align="center">

# 🖐 thing

**A minimal, self-evolving agent harness built from first principles**

![Go](https://img.shields.io/badge/Go-1.26-00ADD8?logo=go&logoColor=white&style=flat-square)
[![License: GPLv3](https://img.shields.io/badge/License-GPLv3-blue.svg?style=flat-square)](COPYING)
![CI](https://img.shields.io/badge/CI-passing-brightgreen.svg?style=flat-square)

</div>

<p align="center">
  <img src="screenshot.png" alt="thing screenshot">
</p>

---

## 🧭 Why this exists

`thing` started with a simple question: how does an agent harness actually work? I
hand-wrote the first loop, then uploaded its source code to the running agent in
base64-encoded chunks. I asked it to implement the `bash` tool and copied its answer
back into the source. That was the moment the harness began to evolve: it could now
inspect and edit its own code.

From there, I began adding the features I enjoy in other harnesses, borrowing most
heavily from [Pi](https://pi.dev), which I use every day.

No vibe-coding. The goal is to understand how agent loops work. A model can help guide
its own evolution, but nothing lands blindly.

## 🧰 Getting started

`thing` connects to an OpenAI-compatible Chat Completions endpoint. Create
`~/.config/thing/config.toml` with your model and endpoint:

```toml
model = "your-model"
endpoint = "https://your-provider.example/v1/chat/completions"
reasoning_effort = "medium" # optional
```

Then run:

```sh
./script/run
```

On first run, `thing` asks for an API token and stores it in the system keychain.

## ⚠️ Current trust model

Tool calls are not confirmed one by one, and sandboxing is not implemented yet. The
`bash` tool runs with the permissions of the process, so use `thing` only in a project
environment where you trust the model and can tolerate its changes.

## 📚 The paper trail

The `docs/adr/` directory captures the reasoning behind the project. It records the
important decisions, the alternatives considered, and the trade-offs that shaped the
harness.
