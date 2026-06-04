package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/veerLakshya/ai-examples/foundation/client"
)

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ChatReq struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
	Stream   bool      `json:"stream"`
}

type ChatChunk struct {
	Choices []struct {
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
	} `json:"choices"`
}

var (
	ChatUrl   = "http://localhost:11435/v1/chat/completions"
	ModelName = "Qwen3-8B-Q8_0"
)

func ChatStreamWriter(ctx context.Context, model, prompt string, w io.Writer) (string, error) {

	req := ChatReq{
		Model:    ModelName,
		Messages: []Message{{Role: "user", Content: prompt}},
		Stream:   true,
	}

	var fullString strings.Builder
	err := client.StreamSSE(ctx, ChatUrl, req, func(cc ChatChunk) error {
		if len(cc.Choices) == 0 {
			return nil
		}
		s := cc.Choices[0].Delta.Content
		if s == "" {
			return nil
		}
		w.Write(([]byte(s)))
		fullString.WriteString(s)
		return nil
	})

	return fullString.String(), err
}

func ChatStreamCallback(ctx context.Context, model, prompt string, onChunk func(string)) (string, error) {
	req := ChatReq{
		Model:    model,
		Messages: []Message{{Role: "user", Content: prompt}},
		Stream:   true,
	}

	var fullString strings.Builder

	err := client.StreamSSE[ChatChunk](ctx, ChatUrl, req, func(cc ChatChunk) error {
		if len(cc.Choices) == 0 {
			return nil
		}

		s := cc.Choices[0].Delta.Content
		if s == "" {
			return nil
		}

		onChunk(s)
		fullString.WriteString(s)
		return nil
	})
	return fullString.String(), err
}

func ChatStreamUsingChan(ctx context.Context, model, prompt string) (<-chan string, <-chan error) {
	out := make(chan string)
	errc := make(chan error, 1)

	go func() {
		defer close(out)
		defer close(errc)

		req := ChatReq{
			Model:    model,
			Messages: []Message{{Role: "user", Content: prompt}},
			Stream:   true,
		}
		err := client.StreamSSE(ctx, ChatUrl, req, func(cc ChatChunk) error {
			if len(cc.Choices) == 0 {
				return nil
			}
			s := cc.Choices[0].Delta.Content
			if s == "" {
				return nil
			}
			out <- s
			return nil
		})
		errc <- err
	}()

	return out, errc
}
func main() {
	ctx := context.Background()
	prompt := "Tell me a short story (2-3 sentences) about a Go programmer who discovers context injection"

	//-----------------------------------------------------
	fmt.Println("====== Variant 1: io.Writer ======")

	full1, err := ChatStreamWriter(ctx, ModelName, prompt, os.Stdout)
	if err != nil {
		panic(err)
	}

	fmt.Printf("\n[full reply was %d chars]\n\n", len(full1))

	//-----------------------------------------------------
	fmt.Println("====== Variant 2: Callback (count chunks) ======")

	chunks := 0
	full2, err := ChatStreamCallback(ctx, ModelName, prompt, func(s string) {
		chunks++
		fmt.Print(s)
	})
	if err != nil {
		panic(err)
	}
	fmt.Printf("\n[saw %d chunks, %d chars]\n\n", chunks, len(full2))

	//-----------------------------------------------------
	fmt.Println("====== Variant 3: channel ======")

	out, errc := ChatStreamUsingChan(ctx, ModelName, prompt)
	for tok := range out {
		fmt.Print(tok)
	}
	err = <-errc
	if err != nil {
		panic(err)
	}
	fmt.Println()
}
