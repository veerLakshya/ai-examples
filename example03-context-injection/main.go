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
	return "", nil
}

func main() {
	res, err := chatCompletion(context.Background(), "Qwen3-8B-Q8_0", "Say hello in one word.")
	if err != nil {
		panic(err)
	}
	fmt.Printf("res: %v\n", res)
}
