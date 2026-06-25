# Example 06 — Vector DB

> **Running notes.** This file is being written *as the example is being
> built*, capturing insights and observations in real time. Final
> consolidation/polish pass happens after stage 5.

A five-stage progression from "brute-force vector search in pure Go" all the
way down to "Product Quantization compression from scratch". Each stage
holds one variable fixed and changes another, so the trade-offs become
visible one at a time.

| Stage | What | Storage | Index |
|---|---|---|---|
| 1 | brute-force in-memory | Go `[]Doc` | none — linear scan |
| 2 | pgvector brute-force | Postgres | none — sequential scan |
| 3 | pgvector + HNSW | Postgres | graph-based ANN |
| 4 | pgvector + IVFFlat | Postgres | partition-based ANN |
| 5 | Product Quantization from scratch | Go | compressed sub-vectors |

The shared `docs` package holds the FlamApp knowledge base + `Doc` struct
used by every stage. Each stage's `main.go` imports it instead of
re-defining the docs.

---

## Stage 1 — brute-force in-memory

### What was built

Three-line retrieval loop in pure Go:

1. Embed the query → `q ∈ ℝ^768`.
2. For each doc `d`, compute `sim_d = cosine(q, embed(d))`.
3. Sort docs by similarity desc, return top K.

The loop is the entire algorithm. `foundation/vector.CosineSimilarity` from
example 01 does the math; the embedding endpoint from example 02 produces
the vectors; this stage just glues them together with a sort.

### Observed ranking

Question targets the CDN doc:

> *"What is the default TTL on FlamApp's asset CDN for static images, and
> what is the documented propagation time after a manual purge?"*

Output:

```
1. cdn-cache-invalidation    0.8138   ← correct, decisive
2. skiff-deploys             0.6277
3. oncall-rotation           0.5901
4. coffee-bar-policy         0.5662   ← did NOT come last
5. mobile-release-branches   0.5204
```

Three observations:

- **Top-1 was correct and confident.** ~0.19 cosine gap between #1 and #2
  is large in embedding-similarity terms — well above the noise floor.
- **coffee-bar-policy ranked above mobile-release-branches.** The coffee
  doc has heavy **duration vocabulary** ("every 3 weeks", "every 8 weeks",
  "every 14 days", "older than 45 days"). The question is *also* about
  durations ("TTL", "propagation time"). Embeddings cluster things by
  *semantic dimension* — and one of those dimensions is "thing about
  time-windows". So the coffee doc looks more similar to the CDN question
  than a process-and-roles doc like `mobile-release-branches` does. **This
  is the dominant failure mode in production RAG**: a topically-unrelated
  doc shares a semantic dimension with the question and crowds out the
  right answer.
- **The "noise band" is real.** Docs 2–5 sit between 0.52 and 0.63 — that
  is the "ambient similarity" of *any* doc to *any* question in this
  corpus. The signal isn't the absolute top-1 score; it's the *distance
  from the noise floor*.

### What stage 1 proves

Vector search isn't a database technology — it's a math operation
(cosine + sort) over a list of vectors. Everything pgvector / Pinecone /
Weaviate adds in later stages is **storage, indexing, and scale**, not
algorithm.

---

## Stage 2 — pgvector brute-force

### Concept landing points

**Wire format.** pgvector vectors travel over the Postgres wire as **text**:
the literal `"[0.1234,-0.5678,...]"` — bracket, comma-separated floats,
bracket. SQL casts the string into the `VECTOR(N)` type with `::vector`.
Concrete payoff: you can run `psql` and type vector literals by hand to
debug.

**Distance operators.** pgvector adds three:

| Operator | Returns | Use when |
|---|---|---|
| `<=>` | cosine distance = `1 − cos_sim` | direction matters, magnitude doesn't (text embeddings — our case) |
| `<#>` | negative inner product | vectors are pre-normalized; slightly faster than `<=>` |
| `<->` | L2 distance | magnitude carries meaning (some image embeddings) |

**Distance vs similarity.** pgvector returns **distance** (lower = closer)
because every general-purpose sorting algorithm assumes ascending order;
returning similarity would force `DESC` and index inversions everywhere.
Display layer flips back: `1 - (embedding <=> $1::vector) AS similarity`.
Sort by distance (no flip needed because ascending distance =
descending similarity for the same row order).

**Schema.** `embedding VECTOR(768)` — the dimension is **baked into the
column type**. Insert a 1024-dim vector and Postgres rejects it. Switching
embedding models forces a schema migration.

**DROP + CREATE at startup** — only for learning. Production uses
migrations. The pattern keeps every learning run isolated.

### Go syntax depth — `formatVector`

The string-builder version exists because **Go strings are immutable**:
`s = s + x` allocates a new string and copies all existing bytes every
time, producing O(n²) total work. `strings.Builder` uses an internal
`[]byte` buffer that grows amortized O(1) — the same growth strategy as
`append`. For a 768-element vector the difference is ~3 kB of allocations
vs ~295 kB.

The serialization spell: `strconv.FormatFloat(x, 'f', -1, 64)`.

- `'f'` — decimal notation (pgvector accepts this; rejects scientific).
- `-1` — **round-trip-safe precision**: the shortest decimal string that
  parses back to the same float64 bits. Any positive precision either
  loses information or wastes space.
- `64` — source bit-size; tells FormatFloat how much precision `-1`
  should aim for.

The `if i > 0 { sep }` separator pattern is more idiomatic than
`if i < len(v)-1`. `strings.Join` over a `[]string` is cleaner-looking but
allocates ~3× more.

### Debug story — the two-Postgres-on-one-port trap

First run produced: `role "postgres" does not exist`. Diagnosis was that
**two Postgres servers were listening on `localhost:5432` simultaneously**:

- a native Homebrew Postgres (PID 688, set up with the macOS username
  `lakshya` as superuser, no `postgres` role),
- the Docker `ai-pg` container (the pgvector one we wanted).

When two processes both `listen()` on the same port with different bind
scopes (`localhost:5432` vs `*:5432`), the kernel picks the "more
specific" listener — usually the loopback-only one. Native Postgres
won; Go connected to it; auth failed because the `postgres` role
doesn't exist there.

Diagnosis incantation:

```sh
lsof -iTCP:5432 -sTCP:LISTEN
```

Fix path chosen: move Docker to port 5433 (`5433:5432` in `compose.yaml`,
update DSN). This preserves native Postgres for other projects.

Reusable lesson: any "something says it's listening but isn't responding
right" → `lsof -iTCP:PORT -sTCP:LISTEN` (or `ss -tlnp` on Linux). This
generalizes to Redis, Kronk, dev servers, every port collision you'll hit.

### SQL parameter placeholder — `$1`, not `%1`

Mistyping `%1::vector` instead of `$1::vector` is a real trap when you've
been writing `fmt.Printf("%v", ...)` for years. SQL parameter grammar
differs across databases too: Postgres uses `$1`, MySQL uses `?`, SQLite
accepts both. Repetition is the cure, not memorization.

### Debug story — the empty-embedding trap (validation-at-every-layer)

After fixing the port conflict + SQL bugs, inserts failed with:

```
ERROR: vector must have at least 1 dimension (SQLSTATE 22000)
```

Looked like a SQL bug; was actually upstream. Direct curl to Kronk
revealed the embedding endpoint was returning a valid-shaped response
with `"embedding": []` — successfully ran, returned 0 floats:

```json
{
  "data": [{ "object": "embedding", "index": 0, "embedding": [] }],
  "usage": { "prompt_tokens": 3, "total_tokens": 3 }
}
```

The client-side guard checked `len(resp.Data) == 0` but **not**
`len(resp.Data[0].Embedding) == 0` — so the empty vector got forwarded
all the way to pgvector, which surfaced the error at the deepest layer
with the least context.

**Reusable lesson — defensive checks at every boundary.** Each layer
that doesn't validate inputs forwards a broken value; the error then
surfaces at the deepest layer with the least context, which makes
debugging harder. The fix is layered validation:

```
HTTP call → "did the request error?"
            "is the response status OK?"
            "is the outer array non-empty?"
            "is the inner field non-empty?"   ← the one we missed
```

Each check is one line; together they pin the failure to the layer where
it actually originated.

Probable root cause was a Kronk/yzma version drift (yzma↔llama.cpp
ABI). Restart cleared it; a longer-input retry confirmed.

### Observed outcome — identical to stage 1

```
1. cdn-cache-invalidation     0.8138
2. skiff-deploys              0.6277
3. oncall-rotation            0.5901
4. coffee-bar-policy          0.5662
5. mobile-release-branches    0.5204
```

Match to **four decimal places** — no rank order change, no
floating-point drift detectable at this precision. The text → vector →
text serialization is lossless for the embedding magnitudes involved.

**This is the load-bearing proof of the example.** The exact same
ranking means the *algorithm* didn't change; we only swapped the storage
layer from a Go slice to a Postgres table. Anything pgvector adds later
(stages 3–4) is index optimization, not algorithmic change.

---

## Stage 3 — pgvector + HNSW index *(in progress)*

### Why we need an index at all (scale framing)

At 1M vectors, brute-force = 1M cosine computations per query ≈ 30-60ms
per query on CPU. At 1k QPS that's 30-60 CPU-seconds per second — needs
30+ cores doing nothing but math. Doesn't scale.

We want **O(log N) per query**, the way binary search beats linear
search. But binary search needs a 1D sort; vectors are 768D. So we use a
**graph** instead of a sort.

### Worked example — why pure k-NN graphs fail

8 points in 2D, two clusters {A,B,C,D} around (1.5, 1.5) and
{E,F,G,H} around (5.5, 5.5). Query q = (5.5, 6). True answer: F.

Plain k-NN graph (each point connects only to its 2 nearest
neighbors): no edges between clusters. Greedy descent from A goes A → B
→ D and **gets stuck at D** (distance 5.32), unable to reach the F
cluster (distance 0.5). Local minimum.

### The shortcut fix (small-world)

Add ONE random long-range edge: A ↔ G. Now greedy descent from A:

```
A (d=6.73)  →  G (d=1.12, via shortcut)  →  H (d=0.5)   ← done in 3 hops
```

The long-range edge "teleports" across the embedding space; short edges
refine locally. This is the **NSW (Navigable Small World)** structure.

### Why hierarchy makes it elegant (HNSW)

You don't know in advance which edges should be long-range. HNSW builds
**multiple graphs at different scales** and lets long-range emerge from
being at higher layers:

```
LAYER 1 (sparse): only ~few nodes (A, G in our toy example),
                  edges span the whole embedding space
LAYER 0 (dense):  all nodes, edges are local neighbors
```

Search:
1. Start at the entry point at the top layer.
2. Greedy-descend to the closest node in this layer.
3. Drop to the next layer down at that node.
4. Repeat until layer 0; final beam search for the K nearest.

Layer 1 = highway across the space; layer 0 = local refinement. Like
Google Maps zoom levels: navigate cities → streets → houses.

### Why this is O(log N)

- Constant hops per layer (the graph at each layer is navigable).
- O(log N) layers (each layer up has exponentially fewer nodes — geometric
  distribution during construction means ~50% of points only at layer 0,
  ~25% at layer 1, ~12.5% at layer 2, etc.).
- Total: O(log N) hops. At 1M vectors: ~20 hops vs 1M cosine
  computations. 30-60ms → ~1ms.

### The three knobs

- **`m`** (default 16): max edges per node per layer. Higher = denser
  graph, better recall, more memory, slower build.
- **`ef_construction`** (default 64): beam width during build.
- **`ef_search`** (default 40, runtime-tunable per query): beam width
  during query. Higher = better recall, slower query.

### Recall vs latency

HNSW is **approximate** — may return the 4th nearest instead of the 1st.
Recall = fraction of true-top-K returned. Typical production: 95-99%
recall at 10-100× speedup vs brute force.

### pgvector syntax

```sql
CREATE INDEX docs_embedding_hnsw_idx
ON docs USING hnsw (embedding vector_cosine_ops);
```

The `vector_cosine_ops` is critical: it tells pgvector the index is for
the `<=>` operator. Other operator classes: `vector_l2_ops` (`<->`) and
`vector_ip_ops` (`<#>`). **Each index supports only ONE distance
operator.** Query with a different one → index is bypassed.

### Code delta vs stage 2

One new statement in `initSchema`, between `CREATE TABLE` and any inserts:

```sql
CREATE INDEX docs_embedding_hnsw_idx
ON docs USING hnsw (embedding vector_cosine_ops);
```

Order matters: build the index *before* insertions to avoid a full
re-scan when each insert touches the index.

Optional `EXPLAIN ANALYZE` query after retrieval to *see* the index plan
activate.

### Expected outcome at our scale

5 docs is too small for HNSW's approximation to matter. The ranking
should match stages 1+2 exactly. The interesting observation is the
**query plan change** — `Seq Scan` → `Index Scan using
docs_embedding_hnsw_idx`. That's the proof the index activated.

### Observed outcome — index alive, ranking identical, planner-overhead dominates

```
1. cdn-cache-invalidation     0.8138
2. skiff-deploys              0.6277
3. oncall-rotation            0.5901
4. coffee-bar-policy          0.5662
5. mobile-release-branches    0.5204
```

Identical to stages 1 + 2 to 4 decimal places. Confirmed: at 5 docs HNSW
has nothing to approximate.

EXPLAIN plan changed as predicted — from `Seq Scan` to `Index Scan using
docs_embedding_hnsw_idx`. Timing block:

```
Planning Time:  0.241 ms
Execution Time: 0.162 ms
```

**Planning > Execution.** This is the *scale inversion* effect: at small
N the planner's overhead dominates. At 1M docs the relationship flips —
planning stays ~constant; execution without HNSW would be 30-60ms; with
HNSW it stays sub-millisecond. The inversion at N=5 is exactly why
HNSW's benefit is invisible at this scale.

Also visible in the plan: `cost=12.54..68.60 rows=630` — Postgres
estimated 630 rows but there were 5. Statistics get refreshed by
`ANALYZE`; we never ran it on this fresh table, so the planner used
defaults. Doesn't affect correctness; matters for complex multi-table
queries where the wrong row estimate triggers a bad join strategy.

---

## Stage 4 — pgvector + IVFFlat index *(in progress)*

### Concept — partition-based ANN

HNSW thinks in **graphs**: build edges, navigate by greedy descent.
IVFFlat thinks in **regions**: pre-partition the embedding space into K
clusters via K-means, search only the few clusters nearest to the
query.

Build-time pipeline:

1. K-means cluster all N vectors → K centroids + posting lists.
2. Each vector lives in exactly one cluster.

Query-time pipeline:

1. Distance from query to each of K centroids → pick nearest P clusters
   (`probes`).
2. Brute-force compare against vectors in those P clusters only.

If K ≈ √N and P = 1, that's O(√N) per query.

### Trade-off vs HNSW

| | HNSW | IVFFlat |
|---|---|---|
| model | navigate graph | look up in partition |
| build | high | medium (K-means) |
| query | O(log N) | O(√N) typical |
| recall at same speed | usually higher | usually lower |
| memory | higher (edges) | lower (centroids + lists) |
| incremental inserts | natural | retrain when distribution shifts |
| tunable knob | `ef_search` | `probes` |

Why HNSW usually wins on recall: IVFFlat is all-or-nothing. If the right
answer lives in a cluster you didn't probe, you miss it entirely.
HNSW's graph is smoother — long-range edges can pull descent toward the
right region from anywhere.

### The "training" requirement

K-means needs the data to find centroids. Index *before* inserts → trains
on nothing → garbage. Index *after* inserts → trains on real
distribution.

This flips the DDL ordering from stage 3. HNSW didn't care; IVFFlat
does.

### pgvector syntax

```sql
CREATE INDEX docs_embedding_ivf_idx
ON docs USING ivfflat (embedding vector_cosine_ops)
WITH (lists = 2);
```

- `USING ivfflat` — the index type.
- `WITH (lists = K)` — number of clusters. Must be supplied. pgvector
  rule of thumb: `lists ≈ rows / 1000` up to 1M; `lists ≈ √rows`
  beyond.

Runtime knob: `SET ivfflat.probes = N;` — defaults to 1; higher = better
recall, slower query.

### Observed outcome — and an unexpected teaching moment

Two findings, neither what the implementation intended:

**1. The first IVF run still actually had an HNSW index** — copy-paste
artifact, the `USING hnsw` from stage 3 wasn't changed to `USING
ivfflat`. So the test wasn't strictly stage-4. Still revealing:

**2. With the index present and the table containing 5 rows, the
planner chose `Seq Scan` instead of `Index Scan`** — different from
stage 3, which used the same HNSW index on the same data:

```
Stage 3 plan:  Index Scan using docs_embedding_hnsw_idx  (cost=12.54..68.60 rows=630)
Stage 4 plan:  Seq Scan on docs  (cost=0.00..1.06 rows=5)
                ↑ planner now correctly knows N=5
```

The `rows=` estimate is the smoking gun. Stage 3's index was created on
an *empty* table; inserts followed without an `ANALYZE`, so the planner
fell back to a default estimate of ~630 rows. Stage 4's index was
created *after* the inserts, which triggered a stats refresh; the
planner now correctly sees 5 rows. At 5 rows, sequential scan is
cheaper than any index access (no index startup overhead).

Reusable lesson — **the planner is allowed to skip your index, and it
will, when the table is small enough that scanning is cheaper**.
Indexes pay off only when N is large enough to amortize their access
cost. Two ways to verify the index is being used at small N: run
`ANALYZE docs;` to refresh stats and re-check, or force it with
`SET enable_seqscan = off;`.

### Code delta vs stage 3

1. Change `USING hnsw` → `USING ivfflat`, add `WITH (lists = 2)`.
2. Move `CREATE INDEX` to *after* `IngestDocs` (already done) — IVF
   needs data to train.
3. To prove the index is present at our scale, set
   `enable_seqscan = off` before the EXPLAIN ANALYZE.

### Observed outcome — IVF's failure mode caught live at 5 docs

Second IVF run (after fixing the index type) produced:

```
Index Scan using docs_embedding_ivf_idx on docs
   (cost=6.27..16.60 rows=5)  (actual time=0.041..0.048 rows=4 loops=1)
                                                       ↑ FOUR, NOT FIVE
```

Ranking returned:

```
1. cdn-cache-invalidation     0.8138
2. skiff-deploys              0.6277
3. oncall-rotation            0.5901
4. coffee-bar-policy          0.5662
                                       ← mobile-release-branches MISSING
```

**Stages 1, 2, 3 returned 5 docs. IVF returned 4.** This is the
all-or-nothing failure mode arriving without any synthetic scale test
— our 5-doc setup happened to produce it naturally.

Mechanically what happened:

1. `lists = 2` → K-means split the docs into 2 clusters.
2. Likely split: `{cdn, skiff, oncall, coffee}` vs
   `{mobile-release-branches}` — the latter became its own cluster
   because process/release-vocabulary is semantically distinct from the
   time/duration-heavy others.
3. Query cosine to the 2 centroids → cluster 1 wins.
4. `probes = 1` (default) → only cluster 1 gets scanned.
5. `mobile-release-branches` lives in cluster 2 → never compared →
   **missed**.

Recall comparison across all four stages:

| Stage | Returned | Recall |
|---|---|---|
| 1 — brute-force Go | 5/5 | 100% |
| 2 — pgvector Seq Scan | 5/5 | 100% |
| 3 — HNSW | 5/5 | 100% |
| **4 — IVFFlat (probes=1)** | **4/5** | **80%** |

This is *exactly* why the trade-off table places IVFFlat below HNSW on
"recall at same speed". HNSW's graph traversal touches every accessible
neighborhood; IVFFlat's cluster scoping is selective by construction.

**Fix at runtime:** `SET ivfflat.probes = 2;` before the query — both
clusters get scanned, recall returns to 100%, cost 2× the
brute-force-within-clusters work. Production systems measure recall
against ground truth, set `probes` to a chosen recall target.

This finding is the single most valuable result of the example so far
— a textbook ANN failure mode reproduced live, with the exact
diagnostic information the planner provides (`actual rows=4`).

### The deeper miss pattern — boundary geometry, not counting

Can IVF still miss when `probes > 1`? **Yes, as long as `probes <
lists`.** The miss pattern isn't "drops a random doc at random rate" —
it's geometric.

Two query positions, same index, different recall:

- **Interior of a cluster**: the true nearest neighbor is in the same
  cluster as the query. Even `probes = 1` gets 100% recall.
- **Boundary between clusters**: the query is ~equidistant from two
  centroids. The true nearest might be in either cluster. `probes = 1`
  → ~50% chance of missing. `probes = 2` covering both boundary
  clusters → 100%.

At scale (e.g. 1M docs, 1000 lists), typical recall curve:

| probes | recall |
|---|---|
| 1 | ~85% |
| 10 | ~99% |
| 100 | ~99.99% |
| 1000 (= lists) | 100% (degrades to brute force inside index) |

The guarantee: **`probes = lists` always gives 100% recall**, because
scanning all clusters means scanning all docs. So IVF's recall trade-off
exists only when `probes < lists`.

### Partition vs graph as a generalizable distinction

This is why HNSW doesn't show the same failure mode — it has no
boundaries. The graph traversal naturally crosses what would be
boundary regions because greedy descent points toward the query
regardless of partitions.

The distinction generalizes:

- **Partition-based** (IVF, hash tables, B-tree pages, sharded DBs) —
  hard region boundaries; queries near boundaries face "did I pick the
  right region". Fix: probe more regions, at proportional cost.
- **Graph-based** (HNSW, social networks, web links) — no boundaries,
  only edges. Boundary-sensitivity disappears. Cost: more memory + more
  complex construction.

Production systems often combine both — IVF for cheap initial pruning,
then HNSW or rerankers within the probed region for precision.

### Debug story — `SET ivfflat.probes` and the pgxpool connection trap

Attempting to verify probes=2 produced contradictory output:

```
EXPLAIN ANALYZE: actual rows=2 loops=1      ← index returned 2 docs
Final SELECT  :  5 docs in the ranking      ← but 5 came out the other side
```

Two effects compounded:

**1. pgxpool grabs a fresh connection per query.** `SET
ivfflat.probes = 2` is *session-scoped* in Postgres — it only applies
to the connection it ran on. The pattern

```go
pool.Exec(ctx, "SET ivfflat.probes = 2")    // connection A
pool.Query(ctx, "EXPLAIN ANALYZE ...")      // connection B (no probes set)
pool.Query(ctx, "SELECT ...")               // connection C (no probes set)
```

silently sends the SET to one connection and the queries to other
connections. The setting is effectively a no-op for the queries that
needed it. Generalizes to every Postgres GUC
(`maintenance_work_mem`, `statement_timeout`, etc) and is one of the
top "I set this and it didn't work" bugs in pgx code.

**Fix — pin the SET and the query to the same connection:**

```go
// Option A: SET LOCAL inside a transaction (idiomatic)
tx, _ := pool.Begin(ctx)
defer tx.Rollback(ctx)
tx.Exec(ctx, "SET LOCAL ivfflat.probes = 2")
tx.Query(ctx, "EXPLAIN ANALYZE ...")
tx.Query(ctx, "SELECT ...")
tx.Commit(ctx)

// Option B: pool.Acquire to hold one connection
conn, _ := pool.Acquire(ctx)
defer conn.Release()
conn.Exec(ctx, "SET ivfflat.probes = 2")
conn.Query(ctx, "EXPLAIN ANALYZE ...")
conn.Query(ctx, "SELECT ...")
```

**2. K-means is non-deterministic across CREATE INDEX runs.** Each
program run drops the table, re-inserts, and re-creates the index,
which re-runs K-means with random centroid initialization. The cluster
splits change between runs:

- Run 1 split: ~{4 docs, 1 doc} → probes=1 hit the 4-doc cluster
- Run 2 split: ~{2 docs, 3 docs} → probes=1 hit the 2-doc cluster

Same data, same algorithm, different partitioning → different recall
numbers in the EXPLAIN's `actual rows=N`. Production systems either fix
the K-means seed for reproducibility or accept the run-to-run noise and
measure recall over many random queries. This noise is also why
**a single recall measurement isn't trustworthy** — you need
averages across many queries to get a meaningful number.

---

## Stages 5 & 6 — deferred

Stage 5 (Product Quantization from scratch) and stage 6 (HNSW from
scratch) involve substantial from-scratch ML algorithm implementation —
K-means, geometric initialization, graph construction, beam search. The
conceptual foundation for both already lives above (worked examples,
trade-off tables, query mechanics). Implementation deferred for now;
revisit when the curriculum loops back to ML deep dives.

The notes that follow remain as design references for if/when we do
revisit:

---

## Stage 5 — Product Quantization from scratch *(deferred — notes below)*

### Conceptual shift vs stages 3 + 4

HNSW and IVF reduced **how many docs you compare**. PQ reduces **how
expensive each comparison is** AND **how much memory each vector
takes**. Different axis of optimization; combines cleanly with the
other two in production (HNSW-PQ, IVF-PQ).

At 1M vectors of 768 float32 dims: 3 GB of vector data. PQ can crush
that to ~10 MB with measurable but acceptable accuracy loss. This is
the memory wall every large vector DB has to solve.

### The split-and-cluster idea

Naive full-space K-means with K=256 → 1 byte per vector, but
useless approximation (one cluster covers a huge slice of 768-dim
space).

PQ instead splits each vector into M chunks, runs K-means independently
in each chunk's sub-space, stores one cluster ID per chunk. With M=8
and K=256, each vector becomes 8 bytes. The combined "vocabulary" is
`256^8 ≈ 1.8 × 10^19` distinct codewords — astronomically more
expressive than one global 256-cluster space.

### The LUT trick (where the query-time speed comes from)

```
Query Q (full precision, NOT quantized)
  ↓ split into M chunks q_1, ..., q_M
For each chunk m:
  For each centroid k in codebook[m]:
    LUT[m][k] = distance(q_m, codebook[m][k])    ← M × K computations, ONCE

For each doc with codes (c_1, ..., c_M):
  approx_distance(Q, doc) = sum over m of LUT[m][c_m]    ← M lookups + (M-1) adds
```

For 1M docs at D=768, M=8:

- Brute-force: 1M × 768 × 2 ops = 1.5B arithmetic ops per query.
- PQ: 8 × 256 (LUT) + 1M × 8 (lookups) = ~8M ops. **~100× speedup**.

Plus ~384× memory savings (8 bytes vs 3072 bytes per vector).

This is called **Asymmetric Distance Computation (ADC)** — keep the
query full-precision, compare against quantized docs. Standard.

### The three knobs

- **M** — number of chunks. Higher = better recall, less compression.
- **K** — centroids per codebook. Standard 256 (1 byte per code).
- **D/M** — chunk dimensionality. Too small (e.g. 1-2 dims) → K-means
  becomes trivial binning.

Production sweet spots: M=8, K=256 for 768-dim text embeddings.

### N=5 problem and the two-track approach

K-means with K=256 needs ≥256 points to be meaningful. Our 5 FlamApp
docs aren't enough to demonstrate PQ. Two parallel experiments:

1. **Correctness test on 5 docs.** Tiny params (M=4, K=3). Verify the
   PQ-ranked top-1 still matches brute-force top-1
   (`cdn-cache-invalidation`).
2. **Recall test on synthetic data.** Generate 1000 random 768-dim
   vectors with `rand.NormFloat64()`. Realistic params (M=8, K=64).
   Measure recall@10 vs brute-force ground truth.

Both use the *same* PQ code — only the corpus differs.

### Implementation shape

Five functions, design top-down:

- `kmeans(data, k, iters) → centroids` — Lloyd's algorithm. Reused per
  sub-space.
- `PQ.Train(docs)` — for each chunk position m, slice that chunk from
  every doc, run K-means, store as `Codebooks[m]`.
- `PQ.Encode(vec) → []byte` — per chunk, find nearest centroid in the
  corresponding codebook, write ID as byte.
- `PQ.Search(query, codes, k)` — build LUT, score every doc by summing
  LUT entries, sort, return top k.

Each is independently testable. The K-means primitive comes first
because the other three depend on it.

---

## Stage 6 — HNSW from scratch in Go *(deferred — notes below)*

After stage 5, type out HNSW yourself. ~250 lines, two evenings. By
then PQ (stage 5) + IVF (stage 4) + pgvector's HNSW observations (stage
3) will have built enough intuition that implementing it feels like
assembly rather than mystery.

### Construction algorithm (to be implemented)

Insert a new vector `v`:

1. Pick random layer L via geometric distribution.
2. Greedy-descend from entry point through layers > L (find entry to
   layer L).
3. For each layer L down to 0: beam-search to find M closest, connect
   bidirectionally. Beam width = `ef_construction`.
4. If L > current entry-point's layer, promote `v` to entry point.

Search is the same minus the connection step.

### Test plan

Build the index over the same 5 FlamApp docs, query with the CDN
question, compare ranking against stages 1 + 2 (which were brute force,
i.e. ground truth). Then scale-test against synthetic vectors (10k, 100k)
to *feel* the recall-vs-latency trade-off in numbers.

---

## Insights accumulated along the way

(Small, reusable lessons — not stage-specific. Will be folded into the
appropriate sections during the final consolidation pass.)

- **The "duration vocabulary" failure mode** (stage 1) generalizes far
  beyond coffee bars. Real teams discover it as "the HR holidays doc
  keeps surfacing for session-timeout questions". Cures: **rerankers**
  (a second model re-scores top-N with full attention) or **hybrid
  retrieval** (combine dense embedding + sparse BM25 so literal keyword
  presence tips the balance). Examples 21+ touch on these.
- **0.8138 cosine for a correctly retrieved doc is what production RAG
  actually looks like — not 0.95.** Natural language vectors aren't
  near-identical because the question matches only some of the doc's
  many semantic dimensions. The "relevant" band sits ~0.65–0.85; real
  failure is top-1 ~0.45.
- **`strings.Builder` over `s += x` in a loop** is one of the top-three
  silently-O(n²) bugs in Go (the others: `append(nil, x...)` without
  pre-allocating capacity; ranging over a map for ordered output without
  sorting keys). All three look correct at small N; all three explode at
  scale.
- **The `<=>` operator returning *distance* is a database-design choice
  that propagates upward**: "smaller = closer" is the universal sort
  convention, so distance keeps every `ORDER BY` and index ascending.
  Once you start thinking in distance instead of similarity, vector code
  gets cleaner; convert to similarity only at the display layer.
- **The "two processes on the same port" diagnosis pattern**
  (`lsof -iTCP:PORT -sTCP:LISTEN`) is reusable for every port collision
  you'll ever hit — Redis, Postgres, Kronk, dev servers. Make it
  reflexive.

---

## What this seeds (preview — will expand at consolidation)

- **Example 07 (ingestion)** — chunking documents before embedding,
  because whole-doc retrieval is too coarse for many questions.
- **Example 08 (rag-pipeline)** — full retrieve → augment → generate
  loop end-to-end.
- **Example 09 (retrieval-debug)** — tune K, similarity thresholds; deal
  with the duration-vocabulary failure mode head-on.
- **Example 11 (rag-perf)** — parallel/batched embedding calls + caching.
- **Example 21 (adaptive-retrieval)** — gate retrieval on a confidence
  classifier; use hedging signals from the model itself.
