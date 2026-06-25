# Example 04 — Chat streaming, from first principles

Example 03 sent a prompt and stared at a blank terminal for ten seconds. This
one turns that same primitive into a **live stream of tokens** — the model
appears to "type" into your terminal — and uncovers the protocol underneath
every modern chat product (ChatGPT, Claude, Cursor, every coding agent).

Along the way we also extract our first shared HTTP package and meet **three
distinct Go shapes** for the same underlying problem: how do you hand a
stream of values from a producer to a consumer.

---

## 1. The problem — perceived vs. actual latency

Example 03's `chatCompletion` did this:

```
[POST request]  →  ............ (10 seconds of silence) ............  →  [full reply]
```

The HTTP body sat idle while Kronk generated every token, then the whole
reply arrived at once and we `Decode`d it. In a CLI demo that's fine; in a
chat product it's unusable — users will tab away.

Streaming flips the shape:

```
[POST request]  →  Hello → ! → I'm → here → to → help → ...
                   ↑ visible at ~200ms; the rest trickles in over 10s
```

The **wall-clock** time to the full reply is unchanged. What changes is
**time-to-first-token (TTFT)** — drops from "10 seconds of silence" to
"~200ms then continuous flow". A single metric that turns a homework demo
into a usable product.

There's a deeper payoff too: streaming enables **early termination** (the
caller can bail mid-generation), and **agent loops** (look at tokens as they
arrive — if the model is starting a tool call, intercept). We'll need both
later, so this isn't just UX polish; it's the foundation for everything from
example 12 (tool calling) onward.

---

## 2. What SSE actually is

The protocol underneath is **Server-Sent Events (SSE)**. Despite the name
it's one of the simplest networking protocols you'll ever meet.

- **It's just HTTP.** Normal POST request, normal response. No special
  handshake, no upgrade, no library.
- **The trick is the response stays open.** The server writes a chunk →
  flushes → writes another chunk → flushes → … → closes. The client reads
  as data arrives instead of waiting for EOF.
- **One-way** — server to client. (WebSockets are bidirectional and use a
  separate upgraded protocol; SSE is much simpler when you only need one
  direction, which chat does.)
- **Body is plain text.** Lines separated by `\n\n` (a blank line = end of
  one message).

The wire format Kronk sends, captured live:

```
data: {"choices":[{"delta":{"role":"assistant"}}]}

data: {"choices":[{"delta":{"content":"Alex"}}]}

data: {"choices":[{"delta":{"content":","}}]}

data: {"choices":[{"delta":{"content":" a"}}]}

data: {"choices":[{"delta":{"content":" Go"}}]}

... (many more) ...

data: {"choices":[{"delta":{},"finish_reason":"stop"}]}

data: [DONE]

```

Three rules to read this:

1. Every line either starts with `data: ` (a payload) or is blank
   (separator).
2. The JSON after `data: ` is one chunk of the response.
3. The literal sentinel `data: [DONE]` marks end-of-stream. **It is not
   valid JSON** — must be checked as a string before `json.Unmarshal`.

---

## 3. The schema change — `delta` vs `message`

The streaming endpoint takes one new request field (`"stream": true`) and
returns a *sequence* of chunks instead of one response.

| Non-streaming (example 03) | Streaming (this example) |
|---|---|
| `choices[].message.content` (whole reply) | `choices[].delta.content` (one piece of the reply) |
| One JSON object | Many JSON objects, separated by blank lines |
| `Decode` once | Loop: read line → parse → repeat |

`delta` is named that way because each chunk carries only **what changed**
since the last one — typically a few new tokens of content. The full reply
is reconstructed by **accumulating** `delta.content` strings as they
arrive. The very first chunk usually has `delta.role: "assistant"` and no
content (a "the assistant is starting to speak" marker); skip it.

---

## 4. Three Go shapes for "stream of values"

The same underlying SSE loop can be exposed three ways. Each one teaches a
different Go idiom — and the *choice between them* is a design lesson
worth internalizing because it generalizes far beyond chat.

### a) `io.Writer` — composition through the standard interface

```go
func ChatStreamWriter(ctx, model, prompt string, w io.Writer) (string, error)
```

Pass `os.Stdout` and tokens print as they arrive. Pass any other writer
(a buffer, a file, a `bytes.Buffer`, a TCP connection) and they go there.
Composes with `io.MultiWriter`, `io.Pipe`, `bufio.Writer`, etc. —
everywhere bytes flow in Go.

**Use it when** the consumer wants the stream as bytes flowing to a
destination they already control.

### b) Callback — inversion of control

```go
func ChatStreamCallback(ctx, model, prompt string, onChunk func(string)) (string, error)
```

The function doesn't know what to do with each chunk; the caller decides.
This lets the consumer do *anything* per chunk — count tokens, transform
them, filter on stop words, gate on a stop signal — without writing a
custom `io.Writer`.

**Use it when** the consumer needs more flexibility than "write bytes
somewhere" but doesn't need full concurrency.

### c) Channel — Go-native concurrency

```go
func ChatStreamUsingChan(ctx, model, prompt string) (<-chan string, <-chan error)
```

The function returns *immediately*; the streaming work runs in a goroutine
and emits tokens via a channel. The caller `range`s over the channel and
reads the error after.

**Use it when** the consumer is concurrent — e.g. wants to multiplex this
stream against others (`select`), or needs to do other work while tokens
arrive.

### Why three shapes? The general principle

These map to a hierarchy:

```
callback (synchronous, single-call)   ← lowest level, most general
   ↓ wraps into ↓
io.Writer (synchronous, byte-flow specialization)
   ↓ wraps into ↓
channel (concurrent, value-flow specialization)
```

**The lowest layer should always be the most general primitive.** That's
why `foundation/client.StreamSSE` takes a callback — building a channel or
an io.Writer on top is a 5-line wrap. The reverse (building a callback out
of a channel) requires a goroutine and re-introduces error-propagation
problems. This pattern shows up everywhere in Go: `filepath.WalkDir(fn)`
takes a callback at the base; channel-based walkers are built on top.
`http.HandlerFunc` takes a function; middleware chains compose on top.
**Synchronous primitive at the bottom, convenience shapes on top.**

---

## 5. Why we extracted `foundation/client` here

Three implementations of "POST JSON, decode JSON" had accumulated:

```
example02-embeddings/main.go      → embedText()
example03-context-injection/...   → chatCompletion()
example04-chat-streaming/main.go  → would have been a third
```

That's the **rule of three** firing exactly when it should. The shared
package — `foundation/client/client.go` — exposes two generic primitives:

| | What it returns | Used by |
|---|---|---|
| `PostJSON[T any](ctx, url, body) (T, error)` | one decoded JSON value | non-streaming requests (would replace example 02 & 03 if we retrofit) |
| `StreamSSE[T any](ctx, url, body, onChunk)` | nothing — calls `onChunk` per chunk | this example's three variants |

The package is deliberately small (~70 lines, ~half of Ardan's
`client.go`). We use `http.DefaultClient`, no logger, no functional
options, no sentinel errors. Those will be added when a real problem
demands them — not because mature codebases happen to have them. See
`learn/foundation/client/client.go` for the full implementation.

The Go feature that made this clean: **generics** (Go 1.18+). `PostJSON[T]`
returns a typed value instead of forcing the caller to pass an `&dst` and
hope. Pre-generics, this code would have looked very different — and most
older Go networking libraries still bear those scars.

---

## 6. Code walkthrough

| Piece | Role |
|-------|------|
| `Message` / `ChatReq` | request structs — same as example 03 but with `Stream bool` |
| `ChatChunk` | streaming response shape — `choices[].delta.content` instead of `.message.content` |
| `ChatStreamWriter` | variant 1 — wraps `client.StreamSSE` with a closure that writes to `io.Writer` |
| `ChatStreamCallback` | variant 2 — wraps `client.StreamSSE` and forwards each content delta to the caller's callback |
| `ChatStreamUsingChan` | variant 3 — wraps `client.StreamSSE` inside a goroutine, emits values on a channel |
| `main` | runs the same prompt through all three variants back-to-back |

The three variants are *strikingly* similar — only one line differs between
1 and 2 (`w.Write(...)` vs `onChunk(...)`), and variant 3 just wraps that
in `go func() { ... }()`. That's the lesson: same engine, three steering
wheels.

---

## 7. Observed results

Same prompt — "tell me a short story about a Go programmer who discovers
context injection" — through all three variants in one run:

- **Variant 1 (io.Writer):** ~258 chars of story, streamed to `os.Stdout`.
- **Variant 2 (callback):** **40 chunks**, ~258 chars total. Average **~6
  characters per chunk** → roughly 1–2 tokens per chunk, given English
  averages ~4 chars/token. We are watching Kronk's autoregressive decoder
  tick forward one or two tokens at a time, flush, tick, flush.
- **Variant 3 (channel):** identical streaming behaviour, longer story
  (sampling is stochastic so each call wanders differently). The protocol
  is mechanically identical; only the content varies.

The bug found and fixed during this example: a missing parameter in
variant 1 (`Content: ""` instead of `Content: prompt`) caused the model to
respond *"It seems like your message is empty"*. A textbook copy-paste
artifact between sibling functions. Worth noting: the API *did not* return
an HTTP error — the model gracefully answered an empty message. **In LLM
systems, "the call succeeded" tells you almost nothing.** You must
inspect the content. This is a recurring lesson — same shape will appear
with prompt injection (ex 23), hallucinated tool calls (ex 16), and
malformed RAG context (ex 25).

---

## 8. Run it

```sh
go run ./example04-chat-streaming    # needs the Kronk server on :11435
```

Try:

- **Switch the prompt** to something open-ended ("Explain attention in
  three paragraphs"). Watch the per-chunk pattern change with the model's
  decoding rhythm.
- **Slow your terminal artificially** by adding `time.Sleep(50 * time.Millisecond)`
  inside variant 1's callback. The unbuffered nature of `os.Stdout` plus
  the unbuffered channel in variant 3 will *backpressure* the network read
  — the server will pause sending until you catch up. This is the same
  mechanism that protects production systems from runaway memory growth.
- **Cancel mid-stream** by giving the `ctx` a 1-second timeout
  (`context.WithTimeout`) and watching the loop terminate cleanly partway
  through. This is the "early termination" foundation for agent loops.
- **Print the raw chunks** by adding `fmt.Println(payload)` inside
  `client.StreamSSE` temporarily. See the actual JSON deltas off the wire
  — the `role: "assistant"` marker, the content chunks, the empty final
  delta with `finish_reason: "stop"`.

---

## 9. What this seeds

- **Example 05 (rag-motivation)** — ties this primitive together with
  example 02's embeddings, asks the question that births retrieval.
- **Example 12 (tool calling)** — the streaming chunks can contain
  `tool_calls` deltas too; the agent loop watches the stream for them and
  intercepts.
- **Example 14 (streaming agent)** — the full agent loop on top of
  streamed chat, including the `<think>...</think>` blocks Qwen3 emits
  before its real answer.
- **Example 19 (speculative decoding)** — chunk granularity changes when
  the server uses a draft model to predict multiple tokens at once. The
  same wire protocol; very different chunk sizes.
- **Module 5 (B-layer descent)** — when we build the generation runtime
  ourselves, the loop on *our* side of the wire will mirror the loop on
  *this* side. The flush after each sampled token is the moment that
  produces these chunks.
