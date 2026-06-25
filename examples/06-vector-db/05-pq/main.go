// GOAL: partition N points into k clusters minimizing within-cluster
// squared-distance from each point to its cluster's centroid

/* LOOP:
1. assign each  point to its nearest centroid
2. move each centroid to the mean of its assigned points
3. stop when nothing moves significantly (convergence) or hit max iters.
*/

package pq

import "math/rand"

func sqEuclidean(a, b []float64) float64 {
	var sum float64
	for i := range a {
		d := a[i] - b[i]
		sum += d * d
	}
	return sum
}

func kmeans(data []float64, k, iters int) [][]float64

func initCentroids(data [][]float64, k int) [][]float64 {
	centroids := make([][]float64, k)
	// pick k  random distinct indices into data
	for i, idx := range rand.Perm(len(data))[:k] {
		centroids[i] = make([]float64, len(data[idx]))
		copy(centroids[i], data[idx])
	}
	return centroids
}

func main() {

}
