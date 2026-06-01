// Example 03 — context injection: writing facts the model doesn't know into its
// working memory (the prompt) so it can answer instead of hallucinating.
// We run the same question twice - cold (no context) and hot (fact injected) —
// and watch the answer change while the model stays the same.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

var (
	// OpenAI-compatible /v1/chat/completions endpoint.
	// Same contract as Ollama (:11434), vLLM, or OpenAI's cloud - only the URL changes.
	ChatUrl   = "http://localhost:11435/v1/chat/completions"
	ModelName = "Qwen3-8B-Q8_0"
)

// Message — one turn in a chat. role is "user", "assistant", or "system".
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatReq — minimal request shape. The API also takes temperature, max_tokens,
// top_p, n, stream, tools, ... - we'll add fields as later examples need them.
type ChatReq struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
}

// ChatResponse — only the field we actually read. The API can return multiple
// candidates (n>1) plus finish_reason and usage; unused fields are dropped by
// Decode, so keeping the struct tight is fine.
type ChatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

// chatCompletion posts a single user prompt and returns the model's reply text.
// Errors come back as errors (network/server failures are part of the world);
func chatCompletion(ctx context.Context, model, prompt string) (string, error) {
	reqBody := ChatReq{
		Model: model,
		Messages: []Message{
			{Role: "user", Content: prompt},
		},
	}

	// Marshal → reader → POST. Same shape as the embeddings client in example 02.
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
		return "", fmt.Errorf("chat: status %d", resp.StatusCode)
	}

	// Decode the response and take the first candidate's content.
	// The empty-Choices guard catches a malformed/empty server response.
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
	// ----- COLD call: ask the unknowable question with no context. -----
	// The "Skiff" fact below was invented for this demo, so the model has never
	// seen it. Expect either a refusal or — more interestingly — a confident
	// hallucination wrapped in industry-standard hedging. That hallucination
	// is the whole reason context injection exists.
	ques := "In FlamApp's Skiff deploy tool, how many times is a failed upload retried before pagin on-call?"
	res, err := chatCompletion(context.Background(), ModelName, ques)
	if err != nil {
		panic(err)
	}
	fmt.Printf("que: %v\n", ques)

	fmt.Println("--- WITHOUT context ---")
	fmt.Printf("res: %v\n", res)

	// ----- HOT call: inject the fact into the prompt. -----
	// The constraint instruction ("use ONLY the context") is load-bearing —
	// without it the model often blends the injected fact with its parametric
	// guesses ("retries 7 times, though typical defaults are 3–5"). With it,
	// the answer stays grounded in the supplied context.
	//
	// Gotcha: this is a raw string (backticks), so the leading tabs from source
	// indentation become part of the prompt verbatim. The model tolerates it;
	// in production you'd dedent or rebuild with explicit \n.
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

	hot, err := chatCompletion(context.Background(), ModelName, prompt)
	if err != nil {
		panic(err)
	}

	fmt.Println("--- WITH context ---")
	fmt.Printf("res: %v\n", hot)
}
