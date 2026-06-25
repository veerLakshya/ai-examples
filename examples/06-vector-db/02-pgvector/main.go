package main

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/veerLakshya/ai-examples/examples/06-vector-db/docs"
	"github.com/veerLakshya/ai-examples/foundation/client"
)

const (
	DSN        = "postgres://postgres:postgres@localhost:5432/postgres?sslmode=disable"
	EmbedUrl   = "http://localhost:11435/v1/embeddings"
	EmbedModel = "embeddinggemma-300m-qat-Q8_0"
)

type EmbedRequest struct {
	Model string `json:"model"`
	Input string `json:"input"`
}

type EmbedResponse struct {
	Data []struct {
		Embedding []float64 `json:"embedding"`
	} `json:"data"`
}

type Result struct {
	Name       string
	Body       string
	Similarity float64
}

func EmbedText(ctx context.Context, text string) ([]float64, error) {
	req := EmbedRequest{
		Model: EmbedModel,
		Input: text,
	}

	resp, err := client.PostJSON[EmbedResponse](ctx, EmbedUrl, req)
	if err != nil {
		return nil, fmt.Errorf("embed err: %w", err)
	}
	if len(resp.Data) == 0 || len(resp.Data[0].Embedding) == 0 {
		return nil, fmt.Errorf("embed err: empty embedding")
	}
	return resp.Data[0].Embedding, nil
}

func FormatVector(v []float64) string {
	sb := strings.Builder{}
	sb.WriteString("[")

	for i, x := range v {
		if i > 0 {
			sb.WriteString(",")
		}
		sb.WriteString(strconv.FormatFloat(x, 'f', -1, 64))
	}

	sb.WriteString("]")
	return sb.String()
}

func InitSchema(ctx context.Context, pool *pgxpool.Pool) error {
	stmts := []string{
		`CREATE EXTENSION IF NOT EXISTS vector`,
		`DROP TABLE IF EXISTS docs`,
		`CREATE TABLE docs(
		id SERIAL PRIMARY KEY,
		name TEXT NOT NULL,
		body TEXT NOT NULL,
		embedding VECTOR(768) NOT NULL
		)`,
	}

	for _, s := range stmts {
		if _, err := pool.Exec(ctx, s); err != nil {
			return fmt.Errorf("execution err: %w", err)
		}
	}
	return nil
}

func IngestDocs(ctx context.Context, pool *pgxpool.Pool) error {
	for _, d := range docs.Docs {
		emb, err := EmbedText(ctx, d.Body)
		if err != nil {
			return fmt.Errorf("embedding err: %w", err)
		}

		// we pass the embedding as a string and SQL coerces it into the VECTOR(768) type
		_, err = pool.Exec(ctx,
			`INSERT INTO docs (name, body, embedding) VALUES ($1, $2, $3::vector)`,
			d.Name, d.Body, FormatVector(emb))

		if err != nil {
			return fmt.Errorf("insert%q: %w", d.Name, err)
		}
		fmt.Printf(" ingested %s (%d dims)\n", d.Name, len(emb))
	}
	return nil
}

func Retrieve(ctx context.Context, pool *pgxpool.Pool, query string, k int) ([]Result, error) {
	qVec, err := EmbedText(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("embed query: %w", err)
	}

	// 	embedding <=> $1::vector — cosine distance between the stored row and the query.
	// 1 - (...) AS similarity — convert distance back to similarity for display.
	// ORDER BY embedding <=> $1::vector — lowest distance first = most similar first.
	// LIMIT $2 — top K.
	rows, err := pool.Query(ctx,
		`SELECT name, body, 1 - (embedding <=> $1::vector) AS similarity
	FROM docs ORDER BY embedding <=> $1::vector LIMIT $2`, FormatVector(qVec), k)

	if err != nil {
		return nil, fmt.Errorf("query: %w", err)
	}
	defer rows.Close()

	var results []Result
	for rows.Next() {
		var r Result
		err = rows.Scan(&r.Name, &r.Body, &r.Similarity)
		if err != nil {
			return nil, fmt.Errorf("scan: %w", err)
		}
		results = append(results, r)
	}
	return results, rows.Err()
}

func main() {
	ctx := context.Background()

	pool, err := pgxpool.New(ctx, DSN)
	if err != nil {
		panic(err)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		panic(err)
	}

	err = InitSchema(ctx, pool)
	if err != nil {
		panic(err)
	}
	fmt.Println("Schema Initailized")

	fmt.Println("Embedding + inserting docs...")
	err = IngestDocs(ctx, pool)
	if err != nil {
		panic(err)
	}

	fmt.Printf("\nQuestion: %s\n\n", docs.Question)
	fmt.Println("--- Ranked docs by cosine similarity (via pgvector) ---")
	results, err := Retrieve(ctx, pool, docs.Question, 5)
	if err != nil {
		panic(err)
	}
	for i, r := range results {
		fmt.Printf("%d. %-25s  %.4f\n", i+1, r.Name, r.Similarity)
	}

}
