package vector

import "math"

// CosineSimilarity returns the cosine of the angle between x and y vectors
func CosineSimilarity(x, y []float64) float64 {
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

// Subtract subtracts vector b from vector a and returns a new vector
func Subtract(a, b []float64) []float64 {
	// assumes len(a) == len(b)
	res := make([]float64, len(a))

	for i := range a {
		res[i] = a[i] - b[i]
	}
	return res
}

// Add adds vector a and b and returns a new vector
func Add(a, b []float64) []float64 {
	// assumes len(a) == len(b)
	res := make([]float64, len(a))

	for i := range a {
		res[i] = a[i] + b[i]
	}
	return res
}
