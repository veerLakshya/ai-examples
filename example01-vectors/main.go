// This example shows what a vector (embedding) is by hand-crafting a set of
// features for ten programming languages, then measuring how similar those
// languages are using cosine similarity. It also demonstrates vector arithmetic where
// "Java - C++ + Rust" lands exactly on Go.

// See vectors.md in this directory for the first-principles explanation.

package main

import (
	"fmt"
	"math"
	"sort"
)

// =============================================================================

// Language is one entity described by six hand-picked features. The ORDER of
// these fields is the order of dimensions produced by Vector and must stay
// consistent across every language, because cosine similarity compares
// dimension i of one vector against dimension i of another.
type Language struct {
	Name              string
	StaticTyping      float64 // These six fields are the "features"; each is a
	Compiled          float64 // number in [0, 1] (0.5 means partial support).
	GarbageCollected  float64
	ObjectOriented    float64
	FunctionalSupport float64
	WebNative         float64
}

// Match pairs a language with its similarity score against some query. It is
// the result type returned by the nearest-neighbour functions below.
type Match struct {
	Lang  Language
	Score float64
}

// Vector converts a Language into its numeric vector representation. The slice
// is built fresh on every call (never cached), so callers can never mutate the
// underlying feature data by accident.
func (l Language) Vector() []float64 {
	return []float64{l.StaticTyping, l.Compiled, l.GarbageCollected, l.ObjectOriented, l.FunctionalSupport, l.WebNative}
}

// =============================================================================

// The dataset: ten languages, hand-scored across the six features. The values
// are tuned so the analogy "Java - C++ + Rust" lands exactly on Go.
var (
	C = Language{Name: "C",
		StaticTyping: 1.0, Compiled: 1.0, GarbageCollected: 0.0, ObjectOriented: 0.0, FunctionalSupport: 0, WebNative: 0}
	Cpp = Language{Name: "C++",
		StaticTyping: 1.0, Compiled: 1.0, GarbageCollected: 0.0, ObjectOriented: 1.0, FunctionalSupport: 0, WebNative: 0}
	Java = Language{Name: "Java",
		StaticTyping: 1.0, Compiled: 1.0, GarbageCollected: 1.0, ObjectOriented: 1.0, FunctionalSupport: 0, WebNative: 0}
	CSharp = Language{Name: "C#",
		StaticTyping: 1.0, Compiled: 1.0, GarbageCollected: 1.0, ObjectOriented: 1.0, FunctionalSupport: 0.5, WebNative: 0}
	Go = Language{Name: "Go",
		StaticTyping: 1.0, Compiled: 1.0, GarbageCollected: 1.0, ObjectOriented: 0.0, FunctionalSupport: 0.5, WebNative: 0}
	Rust = Language{Name: "Rust",
		StaticTyping: 1.0, Compiled: 1.0, GarbageCollected: 0.0, ObjectOriented: 0.0, FunctionalSupport: 0.5, WebNative: 0}
	JavaScript = Language{Name: "Js",
		StaticTyping: 0, Compiled: 0, GarbageCollected: 1.0, ObjectOriented: 1.0, FunctionalSupport: 0.5, WebNative: 1.0}
	TypeScript = Language{Name: "Ts",
		StaticTyping: 1.0, Compiled: 0.0, GarbageCollected: 1.0, ObjectOriented: 1.0, FunctionalSupport: 0.5, WebNative: 1.0}
	Python = Language{Name: "Python",
		StaticTyping: 0.0, Compiled: 0.0, GarbageCollected: 1.0, ObjectOriented: 1.0, FunctionalSupport: 0.5, WebNative: 0}
	Haskel = Language{Name: "Haskel",
		StaticTyping: 1.0, Compiled: 1.0, GarbageCollected: 1.0, ObjectOriented: 0.0, FunctionalSupport: 1.0, WebNative: 0}
)

// =============================================================================

func main() {

	dataPoints := []Language{C, Cpp, Java, CSharp, Go, Rust, JavaScript, TypeScript, Python, Haskel}

	// -------------------------------------------------------------------------
	// Print each language's raw vector so the feature data is visible.
	// region Print vectors

	for _, X := range dataPoints {
		vec := X.Vector()
		fmt.Printf("Lang - %s : [", X.Name)
		for _, v := range vec {
			fmt.Printf("%v ", v)
		}
		fmt.Printf("]\n")
	}

	// endregion

	// -------------------------------------------------------------------------
	// Full pairwise similarity matrix (every language vs every other).
	// Commented out to keep the output short; uncomment to see the whole grid.
	// Only j > i is computed because cosine similarity is symmetric —
	// cos(A,B) == cos(B,A) — so the lower half would be redundant.
	// region Pairwise matrix

	// for i, X := range dataPoints {
	// 	for j := i + 1; j < len(dataPoints); j++ {
	// 		Y := dataPoints[j]
	// 		cosine := cosineSimilarity(X.Vector(), Y.Vector())
	// 		fmt.Printf("%s - %s : %v\n", X.Name, Y.Name, cosine)
	// 	}
	// }

	// endregion

	// -------------------------------------------------------------------------
	// Top-K retrieval: the core operation behind every vector database.
	// region Nearest neighbours

	// nearestLang answers "what else is most similar to this KNOWN language?",
	// so it excludes the query (Python) itself from the results.
	top3 := nearestLang(Python, dataPoints, 3)
	for i, v := range top3 {
		fmt.Println(i+1, v.Lang, v.Score)
	}

	// nearest answers "what is closest to this POINT in space?" — it ranks
	// everything, so the query (Python) appears first at 100%.
	top4 := nearest(Python.Vector(), dataPoints, 4)
	for i, v := range top4 {
		fmt.Println(i+1, v.Lang, v.Score)
	}

	// endregion

	// -------------------------------------------------------------------------
	// Vector arithmetic: analogies become algebra.
	// region Analogies

	// "Java is to C++ as Go is to Rust": (Java - C++) isolates the "added
	// garbage collection" delta; adding it to Rust lands exactly on Go (100%).
	fmt.Println("\nJava - C++ + Rust ≈ ?")
	analogy1 := vectorAdd(vectorSub(Java.Vector(), Cpp.Vector()), Rust.Vector())
	for i, m := range nearest(analogy1, dataPoints, 3) {
		fmt.Printf("  %d. %-8s %.2f%%\n", i+1, m.Lang.Name, m.Score*100)
	}

	// "Add static typing to Python": the result matches no language exactly, so
	// nearest returns the closest neighbourhood (C#/TypeScript), not Python.
	fmt.Println("\nTypeScript - JavaScript + Python ≈ ?")
	analogy2 := vectorAdd(vectorSub(TypeScript.Vector(), JavaScript.Vector()), Python.Vector())
	for i, m := range nearest(analogy2, dataPoints, 3) {
		fmt.Printf("  %d. %-8s %.2f%%\n", i+1, m.Lang.Name, m.Score*100)
	}

	// endregion
}

// =============================================================================

// cosineSimilarity returns the cosine of the angle between vectors x and y: a
// value in [-1, 1] where 1 means identical direction (same meaning), 0 means
// orthogonal (unrelated) and -1 means opposite.
//
//	cos(θ) = (Σ xᵢ·yᵢ) / ( √(Σ xᵢ²) · √(Σ yᵢ²) )
//
// All three sums are accumulated in a single pass. A zero-length vector has no
// direction, so similarity is defined as 0 to avoid dividing by zero.
func cosineSimilarity(x, y []float64) float64 {
	// assumes len(x) == len(y)

	var dot float64 = 0    // Σ xᵢ·yᵢ — how aligned the components are.
	var sumXSq float64 = 0 // Σ xᵢ²   — squared length of x.
	var sumYSq float64 = 0 // Σ yᵢ²   — squared length of y.

	for i := range x {
		sumXSq += x[i] * x[i]
		sumYSq += y[i] * y[i]
		dot += x[i] * y[i]
	}

	if sumXSq == 0 || sumYSq == 0 {
		return 0.0
	}

	root := math.Sqrt(sumXSq) * math.Sqrt(sumYSq) // ‖x‖ · ‖y‖

	cosine := dot / root
	return cosine
}

// =============================================================================

// nearestLang returns the k languages most similar to a KNOWN query language,
// sorted by descending similarity. The query itself is excluded, which makes it
// the right tool for "what else is like X?" questions.
func nearestLang(query Language, dataset []Language, k int) []Match {
	var total []Match
	for _, X := range dataset {
		if X == query { // struct equality skips the query (and any exact duplicate).
			continue
		}
		cosine := cosineSimilarity(query.Vector(), X.Vector())
		total = append(total, Match{
			Lang:  X,
			Score: cosine,
		})
	}

	// Sort descending (highest similarity first). SliceStable keeps tied scores
	// in their original dataset order, so the output is reproducible.
	sort.SliceStable(total, func(i, j int) bool {
		return total[i].Score > total[j].Score
	})

	if k > len(total) { // never slice past the end.
		k = len(total)
	}

	return total[:k]
}

// nearest returns the k languages closest to an arbitrary query VECTOR, sorted
// by descending similarity. Unlike nearestLang it excludes nothing: the query
// is just a point in space and may correspond to no real language — which is
// exactly what an analogy result is. This is the interface real vector
// databases expose.
func nearest(query []float64, dataset []Language, k int) []Match {
	var total []Match

	for _, X := range dataset {
		cosine := cosineSimilarity(query, X.Vector())
		total = append(total, Match{
			Lang:  X,
			Score: cosine,
		})
	}

	// Sort descending; the stable sort makes tied scores deterministic.
	sort.SliceStable(total, func(i, j int) bool {
		return total[i].Score > total[j].Score
	})

	if k > len(total) {
		k = len(total)
	}

	return total[:k]
}

// =============================================================================

func vectorSub(a, b []float64) []float64 {
	// assumes len(a) == len(b)
	res := make([]float64, len(a))

	for i := range a {
		res[i] = a[i] - b[i]
	}
	return res
}

func vectorAdd(a, b []float64) []float64 {
	// assumes len(a) == len(b)
	res := make([]float64, len(a))

	for i := range a {
		res[i] = a[i] + b[i]
	}
	return res
}
