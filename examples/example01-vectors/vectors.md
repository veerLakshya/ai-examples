# Example 01 — Vectors, from first principles

This document explains *why* everything in `main.go` works, building up from the
most basic question in all of AI. The code implements the math by hand (no
libraries) on purpose — the understanding only lands once you've built the thing
the libraries hide.

---

## 1. The fundamental problem

**Computers only understand numbers. So to make a computer reason about
*meaning*, we must encode meaning as numbers.**

Every clever technique in AI — search, RAG, agents — is a consequence of how we
answer that one question.

### Attempt 1 (naive): one integer per thing

```
Horse = 1   Man = 2   Woman = 3   King = 4   Queen = 5
```

This works for *storage* but not for *meaning*. Is `Man (2)` more like
`Woman (3)` or `Horse (1)`? Both are distance 1 away. The numbers are just
labels in disguise; they carry no information about how things relate. Dead end.

### Attempt 2 (the leap): a *list* of numbers, one per property

Pick a handful of **features** and score each thing on every feature. For
programming languages we chose six:

```
[StaticTyping, Compiled, GarbageCollected, ObjectOriented, FunctionalSupport, WebNative]
```

```
Python = [0, 0, 1, 1, 0.5, 0]
Java   = [1, 1, 1, 1, 0,   0]
Go     = [1, 1, 1, 0, 0.5, 0]
```

Now the numbers *mean something*. Python and Java differ in several slots; the
gaps are measurable. Meaning is **distributed** across many dimensions instead
of crammed into one symbol. This is the foundation of every modern embedding
model — Word2Vec, BERT, GPT, Claude all represent tokens exactly this way.

> The word **"embedding"** literally means we *embedded* discrete symbols into a
> continuous vector space — a geometric space with points, distances, and
> directions.

---

## 2. Why "list of numbers" = "vector" = geometry

A list of N numbers is a **vector**: a point in N-dimensional space. Once your
data lives in space, you get the entire toolbox of linear algebra for free:

- **distance** — how far apart two points are
- **angle** — which direction one point lies relative to another
- **arithmetic** — what adding/subtracting vectors means

That last one is how analogies work (section 4).

---

## 3. Measuring similarity: angle, not distance

Two ways to ask "are these vectors close?":

```
Euclidean distance:  ‖A − B‖                 (how far apart)
Cosine similarity:   (A·B) / (‖A‖·‖B‖)        (the angle between them)
```

We use **cosine** for meaning. Here's why, in one example:

```
A = (3, 4)     length 5
B = (30, 40)   length 50
```

Both point in the *exact same direction* (same angle), differing only in
magnitude. Euclidean distance says "far apart" (~45); cosine says "identical"
(1.0). For meaning, **direction is signal and magnitude is intensity** — a
document strongly about "Go" and one mildly about "Go" should still match.
Cosine throws away magnitude and keeps direction.

Cosine ranges from:

- `+1` → same direction (same meaning)
- ` 0` → perpendicular (unrelated)
- `-1` → opposite (antonyms)

### Where the formula comes from (no magic)

The geometric definition of the dot product is `A·B = ‖A‖·‖B‖·cos(θ)`. Rearrange:

```
cos(θ) = (A·B) / (‖A‖·‖B‖)
```

The dot product is just the sum of pairwise products, `A·B = Σ aᵢbᵢ`. So in one
pass over the slices we compute three sums:

```
dot     = Σ xᵢ·yᵢ      (alignment of components)
‖x‖²    = Σ xᵢ²         (squared length of x)
‖y‖²    = Σ yᵢ²         (squared length of y)
cosine  = dot / (√‖x‖² · √‖y‖²)
```

That is exactly what `cosineSimilarity` does. **Watch the two operations:
multiply *within* a pair, add *across* pairs.** (Conflating them — writing `+`
where a `*` belongs — is the classic bug; cosine can never exceed 1, so any
result `> 1` means the math is wrong.)

---

## 4. Vector arithmetic: analogies become algebra

The famous "King − Man + Woman ≈ Queen" works because features are roughly
**independent** and **linear**. Subtracting two vectors isolates the
*relationship* between them; adding it elsewhere applies that relationship.

Our language version, which lands **exactly**:

```
Java − C++ = [1,1,1,1,0,0] − [1,1,0,1,0,0] = [0,0,1,0,0,0]   ← "added garbage collection"
        + Rust [1,1,0,0,0.5,0]             = [1,1,1,0,0.5,0]  = Go
```

"Java is to C++ as Go is to Rust" — both add garbage collection while keeping
static typing and compilation. The arithmetic captures that, and `nearest`
returns Go at 100%.

### Analogies usually point to a *region*, not a point

A second analogy, "add static typing to Python":

```
TS − JS = [1,0,0,0,0,0]   ("added static typing")
     + Python [0,0,1,1,0.5,0] = [1,0,1,1,0.5,0]
```

This vector matches **no language exactly**. `nearest` returns C# and
TypeScript (tied at 87.45%) — *not* Python. And that's correct: a
statically-typed Python really is more like C#/TypeScript than like dynamic
Python. This is how real analogies behave — you compute a synthesized point and
take the nearest real neighbour (conventionally excluding the input terms).

---

## 5. The bridge to real LLM embeddings

This toy is the real thing with two knobs turned:

| Hand-crafted (this example) | Learned LLM embedding (example 02+) |
|---|---|
| 6 dimensions | ~1024 dimensions |
| Each feature has a human name | Each dimension is uninterpretable alone |
| You wrote the numbers | A neural network produced them from text |
| Features exactly independent/linear | Approximately so |
| **Cosine similarity works** | **Cosine similarity works identically** |

Only the *source* of the numbers changes. The math you built here is unchanged.

---

## 6. What the experiments taught us

Running the dataset surfaced lessons that hold for real embeddings too:

- **The feature set determines everything.** Go and Haskell score *highest*
  (97%) even though programmers consider them opposites — because our features
  don't encode the things (purity, syntax, ecosystem) that distinguish them. An
  embedding sees only the dimensions you give it.
- **Low dimensions are brittle.** JavaScript and TypeScript differ in one
  feature yet drop to 87% similar, because one feature is 1/6 of the meaning. In
  1024 dimensions a single differing feature barely moves the needle — that's
  why real embeddings are high-dimensional.
- **No negative scores here.** Unlike the original Horse/Man example (which used
  a signed ±1 gender axis and produced −0.5 similarities), all our features are
  in `[0, 1]`, so the floor is 0 ("share nothing"). The range of your scores is
  a consequence of how you scaled your features.
- **Ties are real.** C# and TypeScript are exactly equidistant from Python, so
  their order is decided by the (stable) sort, not the math.

---

## 7. Code walkthrough

| Piece | Role |
|---|---|
| `Language` struct | one entity; the six float fields are the features. Field order = vector dimension order. |
| `Vector()` method | turns a `Language` into a fresh `[]float64`. Built new each call so callers can't mutate the source data. |
| `var (...)` dataset | ten languages, hand-scored. Values tuned so `Java − C++ + Rust = Go` exactly. |
| `cosineSimilarity(x, y)` | the angle-based similarity; three sums in one pass, divide-by-zero guard for zero vectors. |
| `nearestLang(query, ...)` | top-K for a **known language**; excludes the query itself ("what else is like X?"). |
| `nearest(query []float64, ...)` | top-K for an **arbitrary point**; excludes nothing. Used for analogy results, which are points that match no language. This mirrors how real vector databases work. |
| `vectorSub` / `vectorAdd` | element-wise `a−b` / `a+b`, returning a **new** slice (inputs never mutated). |
| `Match` struct | a `(Language, Score)` pair — the result type for the nearest functions. |

`nearestLang` vs `nearest` is the key design point: retrieval operates on
*points in space*, and a query point need not be a real entity. The type system
enforces this — an analogy result is a `[]float64`, so it can only go to
`nearest`.

---

## 8. Run it

```sh
go run ./example01-vectors      # from the repo root
```

Things to try:

- Add a 7th feature (e.g. `MemorySafe`) and watch every score shift.
- Re-score a language and see which analogies break.
- Compute `Queen − King + Man` style flips of your own and predict before running.
