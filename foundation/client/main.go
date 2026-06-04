// Package client provides generic primitives for talking to OpenAI-compatible
// HTTP APIs. One function per protocol
// shape: PostJSON for a single JSON response, StreamSSE for a stream.

package client

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

func PostJSON[T any](ctx context.Context, url string, body any) (T, error) {
	var zero T

	data, err := json.Marshal(body)
	if err != nil {
		return zero, fmt.Errorf("marshaling error: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return zero, fmt.Errorf("new request error: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return zero, fmt.Errorf("do request error: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return zero, fmt.Errorf("status: %d", resp.StatusCode)
	}

	var out T
	err = json.NewDecoder(resp.Body).Decode(&out)
	if err != nil {
		return zero, fmt.Errorf("decode: %w", err)
	}
	return out, nil
}

func StreamSSE[T any](ctx context.Context, url string, body any, onChunk func(T) error) error {
	data, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("new request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("do request error: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("status %d", resp.StatusCode)
	}

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		payload := strings.TrimPrefix(line, "data: ")
		if payload == "[DONE]" {
			break
		}

		var chunk T
		err = json.Unmarshal([]byte(payload), &chunk)
		if err != nil {
			return fmt.Errorf("decode chunk; %w", err)
		}

		err = onChunk(chunk)
		if err != nil {
			return err
		}
	}

	if err = scanner.Err(); err != nil {
		return fmt.Errorf("scan: %w", err)
	}
	return nil
}
