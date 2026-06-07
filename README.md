# ai-examples

From-scratch reimplementations of AI engineering concepts in Go — vectors,
embeddings, RAG, agents, security, multimodal — built from underlying first
principles. Each example pairs runnable Go code with an `.md` explainer that
captures the *why* behind the design decisions and the observed results from
running the experiment.

## Examples

| Example | Concept | Run |
|---------|---------|-----|
| example01-vectors | Hand-crafted vectors, cosine similarity, top-K retrieval, vector arithmetic | `go run ./examples/example01-vectors` |
| example02-embeddings | Real LLM-generated embeddings, cosine over 768-dim vectors, the representation-flip effect | `go run ./examples/example02-embeddings` |
| example03-context-injection | Chat completions HTTP client + with/without-context experiment showing hallucination | `go run ./examples/example03-context-injection` |
| example04-chat-streaming | SSE streaming via three Go shapes: `io.Writer`, callback, and channel | `go run ./examples/example04-chat-streaming` |
| example05-rag-motivation | Cold vs single-doc vs all-docs runs proving why retrieval is needed | `go run ./examples/example05-rag-motivation` |
| example06-vector-db | Vector search 5 ways: brute force, pgvector, HNSW, IVFFlat, Product Quantization from scratch | `go run ./examples/example06-vector-db/01-bruteforce` (and sibling stage dirs) |

## Foundation packages

Shared infrastructure under `foundation/`:

- **`foundation/vector`** — `CosineSimilarity`, `Add`, `Subtract` over `[]float64`.
- **`foundation/client`** — `PostJSON[T]` and `StreamSSE[T]` generic primitives for OpenAI-compatible HTTP APIs.
- **`foundation/openai`** — Typed structs (`Message`, `ChatRequest`, `ChatResponse`, `ChatChunk`) for the OpenAI-compatible chat wire format.

## Prerequisites

- **Go 1.25+** (generics used extensively).
- **A local OpenAI-compatible LLM server** on `http://localhost:11435` serving both a chat model (`Qwen3-8B-Q8_0`) and an embedding model (`embeddinggemma-300m-qat-Q8_0`). Any backend that implements the OpenAI HTTP contract works.
- **Postgres with the pgvector extension** for example 06 stages 2-4. The included `compose.yaml` brings up a ready-to-use container:

  ```sh
  docker compose up -d
  ```

## Notes

- Module path: `github.com/veerLakshya/ai-examples`.
- Each example directory contains a `.md` explainer that captures first-principles reasoning, observed results from real runs, and what each example seeds for later ones.
