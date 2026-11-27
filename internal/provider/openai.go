package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// OpenAICompat talks to any /chat/completions endpoint in the OpenAI wire
// format. That covers Ollama, LM Studio, Groq, OpenRouter, Gemini and OpenAI.
type OpenAICompat struct {
	name string
	s    Settings
}

func (o *OpenAICompat) Name() string { return o.name }

func (o *OpenAICompat) Complete(ctx context.Context, req Request) (string, error) {
	type msg struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}
	body := struct {
		Model       string  `json:"model"`
		Messages    []msg   `json:"messages"`
		MaxTokens   int     `json:"max_tokens,omitempty"`
		Temperature float64 `json:"temperature"`
	}{Model: o.s.Model, MaxTokens: req.MaxTokens, Temperature: 0}
	if req.System != "" {
		body.Messages = append(body.Messages, msg{"system", req.System})
	}
	for _, m := range req.Messages {
		body.Messages = append(body.Messages, msg{m.Role, m.Content})
	}
	buf, _ := json.Marshal(body)

	url := strings.TrimRight(o.s.BaseURL, "/") + "/chat/completions"
	hreq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(buf))
	if err != nil {
		return "", err
	}
	hreq.Header.Set("content-type", "application/json")
	if o.s.APIKey != "" {
		hreq.Header.Set("authorization", "Bearer "+o.s.APIKey)
	}

	resp, err := httpClient.Do(hreq)
	if err != nil {
		if strings.Contains(err.Error(), "connection refused") && strings.Contains(o.s.BaseURL, "11434") {
			return "", fmt.Errorf("%s: can't reach Ollama at %s (is it running? try `ollama serve`)", o.name, o.s.BaseURL)
		}
		return "", fmt.Errorf("%s: %w", o.name, err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)

	var out struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	_ = json.Unmarshal(raw, &out)
	if resp.StatusCode != http.StatusOK {
		if out.Error != nil && out.Error.Message != "" {
			return "", fmt.Errorf("%s: %s (HTTP %d)", o.name, out.Error.Message, resp.StatusCode)
		}
		return "", fmt.Errorf("%s: HTTP %d", o.name, resp.StatusCode)
	}
	if len(out.Choices) == 0 {
		return "", fmt.Errorf("%s: empty reply", o.name)
	}
	return out.Choices[0].Message.Content, nil
}
