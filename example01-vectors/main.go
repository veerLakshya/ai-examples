package main

import (
	"fmt"
	"math"
	"sort"
)

type Language struct {
	Name              string
	StaticTyping      float64
	Compiled          float64
	GarbageCollected  float64
	ObjectOriented    float64
	FunctionalSupport float64
	WebNative         float64
}

type Match struct {
	Lang  Language
	Score float64
}

func (l Language) Vector() []float64 {
	return []float64{l.StaticTyping, l.Compiled, l.GarbageCollected, l.ObjectOriented, l.FunctionalSupport, l.WebNative}
}

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

func main() {

	dataPoints := []Language{C, Cpp, Java, CSharp, Go, Rust, JavaScript, TypeScript, Python, Haskel}

	_ = dataPoints

	for _, X := range dataPoints {
		vec := X.Vector()
		fmt.Printf("Lang - %s : [", X.Name)
		for _, v := range vec {
			fmt.Printf("%v ", v)
		}
		fmt.Printf("]\n")
	}

	// pairwise matrix
	for i, X := range dataPoints {
		for j := i + 1; j < len(dataPoints); j++ {
			Y := dataPoints[j]
			cosine := cosineSimilarity(X.Vector(), Y.Vector())
			fmt.Printf("%s - %s : %v\n", X.Name, Y.Name, cosine)
		}
	}

	top3 := nearest(Python, dataPoints, 3)
	for i, v := range top3 {
		fmt.Println(i+1, v.Lang, v.Score)
	}
}

func cosineSimilarity(x, y []float64) float64 {
	// asuming len x = len y

	var dot float64 = 0
	var sumXSq float64 = 0
	var sumYSq float64 = 0

	for i := range x {
		sumXSq += x[i] * x[i]
		sumYSq += y[i] * y[i]
		dot += x[i] * y[i]
	}

	if sumXSq == 0 || sumYSq == 0 {
		return 0.0
	}

	root := math.Sqrt(sumXSq) * math.Sqrt(sumYSq)

	cosine := dot / root
	return cosine
}

func nearest(query Language, dataset []Language, k int) []Match {
	var total []Match
	for _, X := range dataset {
		if X == query {
			continue
		}
		cosine := cosineSimilarity(query.Vector(), X.Vector())
		total = append(total, Match{
			Lang:  X,
			Score: cosine,
		})
	}
	// descending sort (highest similarity first)
	sort.SliceStable(total, func(i, j int) bool {
		return total[i].Score > total[j].Score
	})

	if k > len(total) {
		k = len(total)
	}

	return total[:k]
}
