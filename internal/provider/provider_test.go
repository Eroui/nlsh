package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestOpenAICompat(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Errorf("path %s", r.URL.Path)
		}
		if got := r.Header.Get("authorization"); got != "Bearer k" {
			t.Errorf("auth %q", got)
		}
		var body struct {
			Model    string `json:"model"`
			Messages []struct{ Role, Content string }
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body.Model != "m" || len(body.Messages) != 2 || body.Messages[0].Role != "system" {
			t.Errorf("body %+v", body)
		}
		w.Write([]byte(`{"choices":[{"message":{"content":"ls"}}]}`))
	}))
	defer srv.Close()

	p, err := New("x", Settings{Kind: "openai", BaseURL: srv.URL + "/v1", Model: "m", APIKey: "k"})
	if err != nil {
		t.Fatal(err)
	}
	out, err := p.Complete(context.Background(), Request{System: "s", Messages: []Message{{"user", "q"}}, MaxTokens: 10})
	if err != nil || out != "ls" {
		t.Fatalf("got %q, %v", out, err)
	}
}

func TestAnthropic(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/messages" || r.Header.Get("x-api-key") != "k" || r.Header.Get("anthropic-version") == "" {
			t.Errorf("bad request %s %v", r.URL.Path, r.Header)
		}
		w.Write([]byte(`{"content":[{"type":"text","text":"pwd"}]}`))
	}))
	defer srv.Close()

	p, _ := New("anthropic", Settings{Kind: "anthropic", BaseURL: srv.URL, Model: "m", APIKey: "k", NeedKey: true})
	out, err := p.Complete(context.Background(), Request{System: "s", Messages: []Message{{"user", "q"}}, MaxTokens: 10})
	if err != nil || out != "pwd" {
		t.Fatalf("got %q, %v", out, err)
	}
}

func TestAPIErrorSurfaced(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
		w.Write([]byte(`{"error":{"message":"invalid x-api-key"}}`))
	}))
	defer srv.Close()
	p, _ := New("anthropic", Settings{Kind: "anthropic", BaseURL: srv.URL, APIKey: "bad", NeedKey: true})
	if _, err := p.Complete(context.Background(), Request{}); err == nil || err.Error() != "anthropic: invalid x-api-key (HTTP 401)" {
		t.Fatalf("err = %v", err)
	}
}

func TestMissingKey(t *testing.T) {
	if _, err := New("groq", Presets["groq"].Settings); err == nil {
		t.Fatal("want missing key error")
	}
}
