package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

var (
	ChatUrl = "http://localhost:11435/v1/chat/completions"
)

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ChatReq struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
}

type ChatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

func chatCompletion(ctx context.Context, model, prompt string) (string, error) {
	reqBody := ChatReq{
		Model: model,
		Messages: []Message{
			{Role: "user",
				Content: prompt},
		},
	}

	data, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	body := bytes.NewReader(data)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, ChatUrl, body)
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("chat: status %d\n", resp.StatusCode)
	}

	var out ChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", fmt.Errorf("decode: %w", err)
	}
	if len(out.Choices) == 0 {
		return "", fmt.Errorf("chat: no choices")
	}

	return out.Choices[0].Message.Content, nil
}

func main() {

	// question without the required context
	ques := "In FlamApp's Skiff deploy tool, how many times is a failed upload retried before pagin on-call?"
	res, err := chatCompletion(context.Background(), "Qwen3-8B-Q8_0", ques)
	if err != nil {
		panic(err)
	}
	fmt.Printf("que: %v\n", ques)
	fmt.Printf("res: %v\n", res)

	template := `You are a precise assistant. Answer using ONLY the context below.
	If the answer isn't in the context, say you don't know.

	Context:
	"""
	%s
	"""

  	Question: %s`

	fact := `FlamApp's internal deploy tool, Skiff, retries a failed asset upload 7 times, waiting 2
  seconds between attempts, before paging the on-call engineer.`

	prompt := fmt.Sprintf(template, fact, ques)

	hot, _ := chatCompletion(context.Background(), "Qwen3-8B-Q*_0", prompt)
	fmt.Println("--- WITH context ---")
	fmt.Println(hot)
}
