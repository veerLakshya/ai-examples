# Example 05 — RAG motivation, from first principles

Example 03 proved that **injection works**: tell the model the fact and it
answers correctly. This example proves that **injection doesn't scale**, and
that's exactly the question whose answer is RAG (retrieval-augmented
generation). After this we'll spend the next several examples building real
retrieval.

---

## 1. Where we are

| Example | What it built | Question it left open |
|---|---|---|
| 01 | hand-crafted vectors + cosine similarity | where do real vectors come from? |
| 02 | text → 768-dim embeddings | what do we *do* with similarity? |
| 03 | inject one fact → grounded answer | what if the fact lives in a knowledge base? |
| 04 | streamed token-by-token | (orthogonal — UX/agent enabler) |
| **05** | **inject a corpus → answer, but at what cost?** | **how do we find just the relevant piece?** |

Example 05 is the **bridge example** — it makes the limits of manual
injection visceral so the motivation for retrieval is obvious.

---

## 2. The two memories (recap)

From example 03: an LLM has exactly two sources of information when it
answers you.

| Parametric memory | Prompt (context window) |
|---|---|
| compressed in the weights at training time | whatever tokens you send *this* request |
| huge, but lossy and frozen | small, but yours to write |
| can't be updated without retraining | every call |

If a fact is in *neither*, the model fills the gap with a
plausible-sounding fabrication — **hallucination**. Injection (example 03)
solves this for one fact you happen to know. But that solution stops
working the moment you don't know which fact to inject.

---

## 3. The motivating question

In example 03, **you** picked the fact to inject. That worked because the
knowledge base was a single sentence you'd typed yourself.

For any realistic system — a company's internal wiki, an engineering
codebase, a textbook, the internet — you can't manually pick. The
knowledge base might be thousands of documents and the question might be
vague.

> **How does the system decide what to inject?**

There are only two answers:

1. **Inject everything** — paste the whole knowledge base into the prompt.
   Simple. Doesn't scale.
2. **Inject just the relevant piece** — measure similarity between the
   question and each chunk, retrieve the closest match, inject that.
   This is **RAG**.

Example 05 demonstrates the limits of (1) so the engineering cost of (2)
is obviously worth paying.

---

## 4. Why "inject everything" doesn't scale — three concrete limits

### a) Context window — the hard wall

LLMs accept a fixed maximum tokens per call. Qwen3-8B sits around 32k
tokens; bigger models reach 128k or 1M. Sounds large until you do the
math:

- 1 token ≈ ¾ word ≈ 4 chars (English).
- 1 page of dense prose ≈ ~500 tokens.
- 32k tokens ≈ ~64 pages.

A company wiki is usually 1000+ pages. A codebase is millions of tokens.
**You cannot fit them.** Once you cross the limit the request errors out.

### b) Cost — every token costs something

Hosted APIs bill per input token. Local inference burns GPU memory and
time per token. Streaming a 30k-token prompt to the model takes seconds
**before the first reply token**. Paying to send 99% of the prompt for
nothing is wasteful even when it fits.

A pointer to the unit-economics: **3012 input tokens to recover a
~100-token fact is a 30:1 noise ratio.** On a hosted API charging
$0.50/M input tokens, ten thousand of these queries cost $15. Rerun the
same workload through a higher-end model at $30/M and you're at $900.
This is the kind of number that makes RAG a board-meeting topic, not an
ML one.

### c) Noise — the subtler killer

Even when the corpus fits, the model attends to **all of it**
simultaneously. Irrelevant chunks act as noise — the model may:

- Pull facts from the wrong section.
- Blend information from multiple sources that shouldn't be mixed.
- Anchor on the most recent document over the most relevant one.
- Produce hedged answers because "the docs say several things."

You'll see this directly in example 09 when we tune similarity
thresholds. Even a 90%-relevant context with 10% noise produces
measurably worse answers than a clean 60%-relevant one. **Quantity of
context is not the same as quality of context.**

---

## 5. The experiment

A small "knowledge base" of 5 fictional FlamApp internal docs, each ~100
words, each covering a distinct topic:

| Doc | Topic | Numeric facts |
|---|---|---|
| `skiff-deploys` | the Skiff deploy tool | 7 retries, 2s backoff |
| `mobile-release-branches` | release process | 2nd Tuesday, 4-day freeze, 2 reviewers |
| `oncall-rotation` | rotation policy | 12-hour shifts, 6-minute ack SLA, 3-level escalation |
| `cdn-cache-invalidation` | asset CDN | **3600s TTL, 90s propagation, 500 paths/purge** |
| `coffee-bar-policy` | (deliberate noise) | 3-week bean rotation, 8-week machine maintenance |

The question targets one specific buried detail:

> *"What is the default TTL on FlamApp's asset CDN for static images, and
> what is the documented propagation time after a manual purge?"*

The right answer lives in exactly one doc (`cdn-cache-invalidation`). The
coffee-bar doc is deliberate noise — totally unrelated semantically, to
test whether the model gets distracted.

### The three runs

| Run | Context | Purpose |
|---|---|---|
| 1. **Cold** | nothing | observe hallucination shape (familiar from ex 03) |
| 2. **Single doc** | `Docs[0]` = skiff-deploys (the wrong doc) | observe what happens when retrieval picks wrong |
| 3. **All docs** | all 5 concatenated | observe correct-but-bloated behavior |

Note that run 2 deliberately injects the *wrong* document — a Skiff doc
when the question is about the CDN. This isn't a bug; it's modelling the
common production failure mode where retrieval returns a
plausibly-relevant but actually-irrelevant document.

---

## 6. Observed results

A single representative run. The model is stochastic, but the
*structural pattern* of the answers is stable across runs.

### Run 1 — Cold (32 input tokens) — confident hallucination in disguise

The model said *"FlamApp is not a widely recognized or documented CDN
service... TTLs typically range from 24 hours (86,400 seconds) to 30 days
(2,592,000 seconds)..."*

Notice the technique:

- Opens with disclaimers (*"not publicly disclosed"*, *"if FlamApp is a
  typo, clarify"*) that read as honesty.
- Then **anchors on industry-default numbers** in a bolded, structured
  section.
- Closes with "check FlamApp's documentation" — looks like research.

The hedges don't undo the anchor. **A tired engineer skimming this walks
away believing 24 hours.** The right answer was 60 minutes. This is
hallucination *dressed as analysis* — the most dangerous failure mode
because it's structurally indistinguishable from a sourced answer.

### Run 2 — Single doc (163 input tokens) — honest refusal

The injected context (Skiff doc) doesn't address the question. The
model's reply:

> *"The information shared focuses on Skiff's retry logic for failed
> uploads, HTTP status code handling, and deploy approval workflows, but
> it does not address CDN-specific configurations like TTL or purge
> propagation times."*

Two things to notice:

- The model **quoted back the topics it read** — proving it actually
  parsed the context.
- It refused to fabricate, instead saying explicitly what's *not*
  there.

This is exactly the behavior you want in production RAG: the system that
retrieves the wrong document *and* refuses to fluff its way through is
**more trustworthy** than one that confabulates from the wrong context.
The "I don't know, here's what I saw" pattern is auditable; the cold
call's hedged-then-anchored pattern is not.

But it's also a **failure**, in the system-level sense — the user got no
useful answer. *"The system retrieved the wrong document"* is the dominant
RAG failure mode in production. Examples 09 (retrieval-debug) and 21
(adaptive-retrieval) exist specifically to address it.

### Run 3 — All docs (3012 input tokens) — correct but expensive

> *"The default TTL on FlamApp's asset CDN for static images (and all
> public media objects) is **3600 seconds (60 minutes)**. After a manual
> purge, the documented propagation time is **90 seconds** across North
> America and Europe. Edge locations in smaller regions may take up to
> **180 seconds**."*

Exact match. Even captured the edge-region nuance that wasn't asked
about.

But: **3012 input tokens to recover ~100 tokens of relevant content.**
The other 2900+ tokens are noise. At 5 docs this is fine. At 50 docs
it's a problem. At 5000 it's impossible.

### The three-state truth table

Reading the three runs together, you get the actual lesson:

| Context | Answer correctness | Auditability | Cost |
|---|---|---|---|
| **None** | wrong, in disguise | low (looks structured) | tiny (32 tokens) |
| **Wrong doc** | refused (honest) | high | small (163 tokens) |
| **All docs** | correct | medium (model digested noise) | huge (3012 tokens) |

The argument for retrieval almost writes itself: **we need a system that
always picks the right doc, not no doc and not all docs.** Right-doc =
correct, terse, cheap, auditable. RAG is the automation of "right doc
selection."

---

## 7. The big takeaway

> The model didn't get smarter between runs. The model didn't learn
> anything. Only the prompt changed.

That sentence was already true in example 03 with a single fact. Example
05 stretches it to the case where the fact lives in a corpus you can't
manually search. The conclusion is the same — **what's in the prompt
determines the answer** — but the practical question changes from
*"what should I type?"* to *"how do I find what to type, at scale?"*

That second question is what the next six examples answer.

---

## 8. Code walkthrough

| Piece | Role |
|-------|------|
| `Doc{Name, Body}` | minimal knowledge-base entry — fields will grow with ex 06+ (IDs, chunks, embeddings) |
| `Docs []Doc` | hardcoded 5-doc knowledge base |
| `Question` | the specific CDN-targeting question — one source of truth for all 3 runs |
| `AskStreaming(ctx, prompt)` | wraps `client.StreamSSE[openai.ChatChunk]` — writes tokens live to stdout, also accumulates the full reply and returns it with an approximate input-token count |
| 3 runs in `main` | cold → wrong-single-doc → all-docs, same question, different context |

The `len(prompt) / 4` token estimate is wrong by ~10-30% (real
tokenization is model-specific) but the *ratios* between the three runs
are accurate, which is what the lesson needs. Proper tokenizer calls
land in Module 3.

Two shared packages this example finally uses together for the first
time: `foundation/client` (for `StreamSSE`) and `foundation/openai`
(for the typed structs). The schema package landed precisely when the
duplication started biting — example 05 would have been the third
file defining the same `Message`/`ChatRequest`/`ChatResponse` structs.
Rule of three.

---

## 9. Run it

```sh
go run ./examples/example05-rag-motivation
```

Try:

- **Switch the single-doc index** from `Docs[0]` (wrong) to `Docs[3]`
  (the CDN doc) and re-run. The answer should become as exact as the
  all-docs version but ~200 tokens instead of 3012. **This is what good
  retrieval would deliver.**
- **Add a 6th doc** that *partially* discusses CDN (e.g., a different
  product's caching policy) and re-run with all-docs. Watch whether the
  model blends facts from both. Production retrieval has to handle this.
- **Move the right doc to the *end*** of the `Docs` slice and re-run with
  all-docs. Some LLMs exhibit "lost-in-the-middle" behavior — accuracy
  degrades when the relevant chunk sits in the middle of a long context.
  Position matters; another reason to retrieve sharply rather than
  inject broadly.

---

## 10. What this seeds

- **Example 06 (vector-db)** — store doc embeddings somewhere queryable;
  replace "pick `Docs[0]`" with "find the most similar doc to the
  question."
- **Example 07 (ingestion)** — chunk documents before embedding, because
  whole docs are still too coarse for many questions.
- **Example 08 (rag-pipeline)** — the full retrieve → augment → generate
  loop end-to-end.
- **Example 09 (retrieval-debug)** — when the retrieved doc is *wrong*
  (which run 2 demonstrated), how do you diagnose it? Tune K, tune
  similarity thresholds, look at the wrong-but-close neighbors.
- **Example 11 (rag-perf)** — parallel/batched embedding calls and
  response caching for the same workload.
- **Example 21 (adaptive-retrieval)** — a classifier that decides *whether
  to even retrieve* based on confidence signals (the hedging patterns
  the cold call exhibited are a real input feature).

The "single doc — wrong" run is the failure mode the next six examples
are built to prevent. Keep this output handy; it's the baseline
everything ahead improves on.
