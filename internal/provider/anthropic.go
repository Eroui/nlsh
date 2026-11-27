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

// Anthropic talks to the Claude Messages API.
type Anthropic struct {
	name string
	s    Settings
}

func (a *Anthropic) Name() string { return a.name }

func (a *Anthropic) Complete(ctx context.Context, req Request) (string, error) {
	type msg struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}
	body := struct {
		Model     string `json:"model"`
		MaxTokens int    `json:"max_tokens"`
		System    string `json:"system,omitempty"`
		Messages  []msg  `json:"messages"`
	}{Model: a.s.Model, MaxTokens: req.MaxTokens, System: req.System}
	for _, m := range req.Messages {
		body.Messages = append(body.Messages, msg{m.Role, m.Content})
	}
	buf, _ := json.Marshal(body)

	url := strings.TrimRight(a.s.BaseURL, "/") + "/v1/messages"
	hreq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(buf))
	if err != nil {
		return "", err
	}
	hreq.Header.Set("content-type", "application/json")
	hreq.Header.Set("x-api-key", a.s.APIKey)
	hreq.Header.Set("anthropic-version", "2023-06-01")

	resp, err := httpClient.Do(hreq)
	if err != nil {
		return "", fmt.Errorf("anthropic: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)

	var out struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	_ = json.Unmarshal(raw, &out)
	if resp.StatusCode != http.StatusOK {
		if out.Error != nil {
			return "", fmt.Errorf("anthropic: %s (HTTP %d)", out.Error.Message, resp.StatusCode)
		}
		return "", fmt.Errorf("anthropic: HTTP %d", resp.StatusCode)
	}
	var sb strings.Builder
	for _, c := range out.Content {
		if c.Type == "text" {
			sb.WriteString(c.Text)
		}
	}
	return sb.String(), nil
}
