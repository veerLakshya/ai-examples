package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/veerLakshya/ai-examples/foundation/vector"
)

type item struct {
	Name string
	Text string
	Emb  []float64
}

var (
	embeddingUrl = "http://localhost:11435/v1/embeddings"
	modelName    = "embeddinggemma-300m-qat-Q8_0"
)

type ReqBody struct {
	Model       string `json:"model"`
	Input       string `json:"input"`
	Truncate    bool   `json:"truncate"`
	TruncateDir string `json:"truncate_direction,omitempty"`
}

type EmbedResponse struct {
	Data []struct {
		Embedding []float64 `json:"embedding"`
	} `json:"data"`
}

func embedText(ctx context.Context, model, text string) ([]float64, error) {

	reqBody := ReqBody{
		Model:    model,
		Input:    text,
		Truncate: false,
	}

	data, err := json.Marshal(reqBody) // data is []byte of JSON
	if err != nil {
		return nil, fmt.Errorf("marshal: %w", err)
	}

	body := bytes.NewReader(data)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, embeddingUrl, body)
	if err != nil {
		return nil, fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req) // actually sends it
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("embed: status %d", resp.StatusCode)

	}

	var out EmbedResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}
	if len(out.Data) == 0 {
		return nil, fmt.Errorf("embed: empty data")
	}
	return out.Data[0].Embedding, nil
}

func main() {
	vec, err := embedText(context.Background(),
		"embeddinggemma-300m-qat-Q8_0", "hello")
	if err != nil {
		panic(err)
	}
	fmt.Println("len: ", len(vec))
	// fmt.Println("vec: ", vec)

	//====================================================

	var items []item = []item{
		{
			Name: "Go",
			Text: "Go: a compiled, statically typed, garbage-collected language with built-in concurrency",
		},
		{
			Name: "Rust",
			Text: "Rust: a compiled, statically typed systems language with manual memory management",
		},
		{
			Name: "Python",
			Text: "Python: a dynamically typed, interpreted, garbage-collected scripting language",
		},
		{
			Name: "Haskel",
			Text: "Haskell: a compiled, statically typed, purely functional language",
		},
		{
			Name: "chocolate chip cookies",
			Text: "a recipe for chocolate chip cookies",
		},
	}

	for i := range items {
		emb, err := embedText(context.Background(), modelName, items[i].Text)
		if err != nil {
			panic(err)
		}
		items[i].Emb = emb
	}

	for i := range items {
		for j := i + 1; j < len(items); j++ {
			cosineSim := vector.CosineSimilarity(items[i].Emb, items[j].Emb)
			fmt.Printf("%s - %s : %v\n", items[i].Name, items[j].Name, cosineSim)
		}
	}

	fmt.Println()

	for i := range items {
		embA, err := embedText(context.Background(), modelName, items[i].Name)
		if err != nil {
			panic(err)
		}
		for j := i + 1; j < len(items); j++ {
			embB, err := embedText(context.Background(), modelName, items[j].Name)
			if err != nil {
				panic(err)
			}
			cosineSim := vector.CosineSimilarity(embA, embB)
			fmt.Printf("%s - %s : %v\n", items[i].Name, items[j].Name, cosineSim)
		}
	}

}
