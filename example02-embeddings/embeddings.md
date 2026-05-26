# Example 02 — Real embeddings, from first principles

In example 01 we *hand-crafted* 6-dimensional vectors. Here the vectors come
from a real model: we send **text** to an embedding model and get back **768
learned numbers**. The cosine math from example 01 (`foundation/vector`) is
reused unchanged — only the *source* of the vectors changed.

---

## 1. What actually changed

| | Example 01 | Example 02 |
|---|---|---|
| dimensions | 6 | 768 |
| who chose the features | you, by hand | the model, learned from training text |
| are dimensions interpretable? | yes (`Authority`, `Animal`…) | no — dim #40 is `-0.4267` and means nothing nameable |
| similarity math | `CosineSimilarity` | **same `CosineSimilarity`** |

The whole point of example 01 was to make this moment unremarkable: the math is
identical, the numbers are just produced differently.

---

## 2. The client — the universal "call a model" shape

`embedText` is ~40 lines and follows the pattern behind *every* model/API call:

```
build request struct → json.Marshal → bytes.NewReader → http POST
                                                          ↓
return data[0].Embedding ← json.Decode ← read response body
```

Key Go points:
- **`io.Writer` vs `io.Reader`.** `json.Marshal` gives `[]byte`; `bytes.NewReader`
  turns it into an `io.Reader` that the HTTP body can be read from. `json.Decoder`
  reads the response stream (an `io.Reader`) back into a struct.
- **Errors, not panics.** A 500, a network failure, empty data — these are things
  *the world does to you*, so they return `error` (contrast: example 01's
  dimension mismatch was a programmer bug → `panic`).
- The backend is **swappable**: the same client works against Kronk (`:11435`),
  Ollama (`:11434`), vLLM, or OpenAI's cloud — only the URL changes. The OpenAI
  HTTP contract is the stable interface; the server behind it is plumbing.

The contract:
```
POST /v1/embeddings  {"model":"embeddinggemma-300m-qat-Q8_0","input":"<text>"}
            →        {"data":[{"embedding":[ ... 768 floats ... ]}]}
```

---

## 3. Why exactly 768?

768 is the model's **embedding dimension** (its hidden size) — a fixed
architectural constant of EmbeddingGemma-300M. Every input, short or long, comes
back as exactly 768 floats.

- **Why fixed-length?** The model reads a *variable* number of tokens, then
  **pools** them into one fixed-size vector. It must be fixed because
  `CosineSimilarity` can only compare equal-length vectors. (We build that
  pooling step ourselves in Module 3.)
- **Why 768 specifically?** It's the transformer's `d_model`. `768 = 12 × 64`
  (12 attention heads × 64 dims) — the BERT-base lineage. It's a capacity/cost
  trade-off: bigger models use 1024 / 1536 / 3072+.
- **`300M` (params) ≠ `768` (dimensions).** Params = how much the model knows;
  dimensions = how wide its output vector is.

EmbeddingGemma is also trained with **Matryoshka** representation learning, so the
768-vector can be truncated to 512/256/128 and still work — dimensionality made
tunable (example 01's "more dims = richer, fewer = faster/brittler" on one model).

---

## 4. The experiments — what real embeddings taught us

We compared four language descriptions + a control ("chocolate chip cookies").

### a) Topic dominates

With templated descriptions, the four languages clustered tightly (**0.73–0.78**)
while cookies sat far off (**~0.50**). Embeddings are dominated by overall
topic/structure; fine distinctions are a small perturbation on top. This is why
real RAG needs **similarity thresholds** (example 09) and **rerankers**
(example 21) — relevant and merely-on-topic docs often score within 0.05.

### b) Representation determines similarity (the big one)

The same five items, represented three ways, gave **three different similarity
structures**:

```
toy (6 hand features)    → Go ≈ Haskell    (shared compiled/static/GC features)
templated sentences      → Go ≈ Haskell    (shared sentence structure)
bare names (real model)  → Rust ≈ Python   (co-occur constantly in real text)
```

Templated vs bare-name rankings literally *flipped*:

| Pair | Templated | Bare names |
|------|-----------|------------|
| Rust ↔ Python | 0.731 (lowest) | **0.852 (highest)** |
| Go ↔ Haskell | 0.777 (highest) | 0.694 (lowest) |

Bare names make the model fall back on *distributional* similarity — how the
languages actually co-occur in its training corpus. Rust and Python are discussed
together constantly (Rust-based Python tooling: ruff, uv, polars; "Rust vs
Python"), so their name-vectors are close. Go and Haskell rarely co-occur.

**Conclusion: "similarity" is not a property of the things — it's a property of
how you represent them.** There is no single true answer to "is Go more like Rust
or Haskell?" It depends entirely on what you encode. This is the thesis the whole
vectors/embeddings foundation was building toward.

### c) Polysemy and context

The bare word `"Cookies"` scored ~0.71–0.77 against *programming languages* —
because "cookies" also means HTTP/browser cookies. Switching to `"chocolate chip
cookies"` disambiguated it to food and dropped the scores. Word sense is resolved
by context; one bare word has almost none.

### d) The trade-off

Templated text → clean topic separation (0.50 vs 0.75) but compressed within-topic
detail. Bare names → rich within-topic spread (0.69–0.85) but a fuzzy topic
boundary (cookies crept up to ~0.62–0.67). The input text shapes both axes — no
free lunch. **Query phrasing is part of retrieval quality.**

---

## 5. Code walkthrough

| Piece | Role |
|-------|------|
| `ReqBody` / `EmbedResponse` | request and (minimal) response JSON structs |
| `embedText(ctx, model, text)` | the client: marshal → POST → decode → `data[0].Embedding` |
| `item{Name, Text, Emb}` | holds a label, its input text, and the resulting vector |
| experiment 1 | embed templated descriptions, pairwise `vector.CosineSimilarity` |
| experiment 2 | embed bare names, pairwise — shows the representation flip |

Note: experiment 2 re-embeds each name multiple times inside the nested loop —
embedding calls are expensive, and caching/batching them is exactly what
example 11 (rag-perf) optimizes. Fine for a 5-item demo.

---

## 6. Run it

```sh
go run ./example02-embeddings    # needs the Kronk server on :11435
```

Try: swap the templated descriptions for bare names and watch the matrix change;
sum the squares of one embedding to check whether the model returns
unit-normalized vectors (if ≈ 1.0, cosine ≈ dot product).
