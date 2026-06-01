# Example 03 — Context injection, from first principles

Example 02 turned text into vectors. This one turns text into **more text** —
we call the *generation* primitive (chat completions) — and uncover the central
mechanism for making the model answer questions about things it doesn't
already know: **write the facts into the prompt.**

---

## 1. The two memories an LLM has

An LLM is, mechanically, a single function: **take a sequence of tokens,
produce a probability distribution over the next token.** That's all it does.
When it answers you, it can draw on exactly **two** sources:

| | Parametric memory | Prompt (context window) |
|---|---|---|
| What it is | knowledge compressed into the weights during training | the tokens you send *this* request |
| Size | huge, but fixed | small, but yours to write |
| Freshness | frozen at training cutoff | now |
| Private data? | never seen | put it there |
| Updatable? | no (would require retraining) | every call |
| Verifiable? | no — model can't cite a source | yes — you wrote it |

If a fact is in neither, the model does not have it. And critically: it
usually **doesn't know that it doesn't know**, so it fills the gap with a
confident, plausible fabrication. That's **hallucination**.

---

## 2. The fix: the prompt is writable working memory

You can't easily change weights. But the prompt changes on every call — for
free. So:

> If the model lacks a fact, **put the fact in the prompt**. Then the model
> doesn't need to *remember* it — it just *reads* it.

The analogy that makes it click: parametric memory is what you know by heart;
the prompt is an **open book on the desk**. Context injection is putting the
right page in front of the model before you ask the question. A closed-book
exam becomes an open-book one.

Mechanically, injection bundles the fact, the question, and a constraint
instruction into one prompt:

```
Answer the question using ONLY the context below.
If the answer isn't in the context, say you don't know.

Context:
"""
<the relevant facts>
"""

Question: <the user's question>
```

The model attends to the context tokens while generating the answer. That's
also why **prompt-injection attacks work** (example 23) — attention can't
inherently tell instructions from pasted data.

---

## 3. Tokens — what the model actually reads

The model never sees words or characters directly. Text is chopped into
**tokens** — chunks from a fixed vocabulary (~50k entries) — before anything
runs.

Why subwords instead of words or characters?

| Unit | Vocabulary | Sequence length | Verdict |
|------|-----------|-----------------|---------|
| characters | tiny | very long | expensive, each unit means little |
| words | huge, open-ended | short | constant "unknown word" for names, typos, code |
| subwords | medium | medium | ✅ the sweet spot — what all modern models use |

`dog` = 1 token. `tokenization` ≈ 2 (`token` + `ization`). ` the` (with the
leading space) = 1 token. Rule of thumb in English: **1 token ≈ ¾ word ≈ 4
characters** — so 100 tokens ≈ 75 words.

This matters because **the context window is measured in tokens**. "8k
context" = 8192 tokens for *prompt + response combined*. Every fact you inject
spends from that budget. That finite budget is exactly what later forces RAG:
you can't paste your whole knowledge base, so you must **retrieve the few
most relevant chunks and inject those.**

---

## 4. The chat completions contract

Same OpenAI-compatible shape as the embedding endpoint in example 02 — only
the URL and JSON change:

```
POST /v1/chat/completions
{
  "model": "<model>",
  "messages": [
    { "role": "user", "content": "<prompt>" }
  ]
}
            →
{
  "choices": [
    { "message": { "role": "assistant", "content": "<generated text>" },
      "finish_reason": "stop" }
  ]
}
```

- `messages[]` — a chronological conversation. `role` is `user`, `assistant`,
  or `system`. We only send one user turn here; later examples (12 tool
  calling, 14 streaming agent) will use multi-turn.
- `choices[]` — the API can return multiple candidate completions (`n: 3`
  gives three). We didn't ask for more than one, so we take `choices[0]`. The
  guard on empty `Choices` is defensive.
- `finish_reason` — `"stop"` = finished naturally, `"length"` = hit the
  max-tokens cap and was cut off, `"tool_calls"` = the model wants to call a
  tool (exactly what example 12 will handle). We don't read it yet, but it's
  important to know it exists.

The Go client is the same shape as the embeddings one:

```
build request struct → json.Marshal → bytes.NewReader → http POST
                                                          ↓
return choices[0].Message.Content ← json.Decode ← response body
```

Different endpoint, different field names, identical pattern. The
**OpenAI-compatible HTTP contract** is the stable interface; whether the
server is Kronk, Ollama, vLLM, or OpenAI's cloud, only the URL changes.

---

## 5. Why the prompt template matters

The single line `"Answer using ONLY the context below. If the answer isn't in
the context, say you don't know."` does enormous work. Without it, models
tend to **blend** the injected fact with their parametric guesses — producing
sentences like *"Skiff retries 7 times, though typical defaults are 3–5"* —
which is worse than either pure source, because the user can't tell which
part is grounded.

This is the seed of **prompt engineering as a control surface**: prompt
structure is not phrasing preference, it's an engineering knob that controls
correctness. In production RAG, this exact template becomes the thing the
retrieval pipeline fills in programmatically.

---

## 6. The experiment — observed results

Same model. Same question. Different prompt → completely different answer.

**Question:** *"In FlamApp's Skiff deploy tool, how many times is a failed
upload retried before paging on-call?"*

**Fact (only used in the hot call):** *FlamApp's Skiff retries a failed asset
upload 7 times, waiting 2 seconds between attempts, before paging the on-call
engineer.*

### Cold (no context) — confident hallucination

> Based on common practices in deployment systems (CI/CD pipelines, cloud
> services), **3–5 retries** is a default... If Skiff follows industry
> standards, **3 retries** might be the default before escalating to on-call
> paging. However, this is speculative... **Conclusion**: While the exact
> number isn't publicly confirmed, a typical default might be **3 retries**.

The model:
- Opened with hedges (*"not explicitly documented"*, *"speculative"*).
- Then **anchored on a specific wrong number anyway** (3) — bold-faced.
- Built a structural argument (Common Retry Policies → Skiff-Specific Context
  → Recommendation → Conclusion) that *looks* like research even though no
  new information was retrieved.

A tired engineer skimming this walks away believing **3**. The hedges don't
undo the anchor. *This is what makes hallucination dangerous: it's not
"obviously wrong" — it's confident, structured, and plausible.*

### Hot (with context) — clean and exact

> The failed upload is retried 7 times before paging the on-call engineer.

One sentence. Right number. No padding. No "industry standard" detour. The
constraint instruction kept the parametric guess out.

### The takeaway

> The model didn't get smarter. The model didn't learn anything.
> You wrote the right page into its working memory before asking.

That's the entire mechanism. Every later module — RAG, agents, tools,
multimodal — is variations on **"what should I put in the prompt, and how do
I find it automatically?"**

---

## 7. Two patterns worth noticing for later

**a) Hedging as a hallucination signal.** The cold answer was full of
uncertainty markers (*"may"*, *"might"*, *"speculative"*, *"common
practices"*). The hot answer had none. This is the basis for
**confidence-based gating** — you can sometimes detect "the model is about to
make something up" from its own language. Example 21 (adaptive-retrieval)
uses exactly this idea: a classifier decides *whether to even bother
retrieving*.

**b) The constraint line is doing the heavy lifting.** A weaker prompt
(`"Context: <fact>\n\nQuestion: <q>"` with no instruction) often leaks the
parametric guess back in. The sentence *"Answer using ONLY the context. If
the answer isn't in the context, say you don't know."* is one of the most
leveraged prompts you'll ever write — it's the seed of grounding, citation,
and refusal in production RAG.

---

## 8. Code walkthrough

| Piece | Role |
|-------|------|
| `Message` / `ChatReq` | request structs — one user turn, more roles in later examples |
| `ChatResponse` | minimal response struct — we only read `choices[0].message.content` |
| `chatCompletion(ctx, model, prompt)` | the client: marshal → POST → decode → return content |
| cold call | ask the unknowable question — observe hallucination |
| `template` + `fact` | the injection prompt: instruction + delimited context + question |
| hot call | same model, same question, fact in working memory → correct answer |

A Go gotcha to know: the `template` is a **raw string** (backticks), which
keeps every character literally — including the leading tabs from source
indentation. The model is robust to whitespace noise so the experiment still
works, but in production you'd dedent it or build it with `\n` in a regular
string.

---

## 9. Run it

```sh
go run ./example03-context-injection    # needs the Kronk server on :11435
```

Try:

- **Weaker prompt experiment.** Drop the *"use ONLY the context"* line and
  see if the model blends 7 with "industry standard 3–5". This proves the
  constraint is load-bearing, not decorative.
- **Different fact, same shape.** Make up another unknowable fact (your
  team's deploy schedule, an internal acronym) and watch the same
  cold-hallucinates / hot-grounded pattern repeat. Build the muscle for
  spotting it.
- **Switch the role to `"system"`** for the instruction and `"user"` for the
  question. Many models weight system messages more heavily. See if the cold
  answer admits ignorance more honestly when the constraint sits in a system
  message.

---

## What this seeds

- **Example 04 (chat-streaming)** — same primitive, but the body becomes a
  stream of tokens (SSE) instead of one blob.
- **Example 05 (rag-motivation)** — same with/without contrast, but pointed
  at a corpus that's too big to inject manually → "but how do I know what to
  inject?" births RAG.
- **Module 2 (RAG)** — automate "find the right context" via Module 0's
  cosine similarity over Example 02's embeddings.
- **Example 21 (adaptive-retrieval)** — the hedging signal becomes a
  classifier gate.
- **Example 23 (prompt injection)** — the same attention that reads injected
  context also reads injected *instructions*; the security implications fall
  out of the mechanism itself.
