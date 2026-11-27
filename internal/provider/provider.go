// Package provider defines the model-agnostic interface the engine talks to,
// plus concrete backends (Anthropic, and any OpenAI-compatible API such as
// Ollama, Groq, OpenRouter, Gemini or OpenAI itself).
package provider

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"time"
)

// Message is a single chat turn.
type Message struct {
	Role    string // "user" or "assistant"
	Content string
}

// Request is what the engine sends to a model.
type Request struct {
	System    string
	Messages  []Message
	MaxTokens int
}

// Provider turns a Request into the model's raw text reply.
// Adding a new model backend means implementing this one method.
type Provider interface {
	Name() string
	Complete(ctx context.Context, req Request) (string, error)
}

// Settings configure a provider instance.
type Settings struct {
	Kind    string // "anthropic" or "openai" (OpenAI-compatible wire format)
	BaseURL string
	Model   string
	APIKey  string
	KeyEnv  string // env var the key is normally read from (informational)
	NeedKey bool
}

// Preset is a known provider with sensible defaults.
type Preset struct {
	Settings
	Description string
}

// Presets lists the built-in providers. Any of these can be overridden in the
// config file, and new OpenAI-compatible endpoints can be added there too.
var Presets = map[string]Preset{
	"ollama": {
		Settings:    Settings{Kind: "openai", BaseURL: "http://localhost:11434/v1", Model: "qwen2.5-coder:7b"},
		Description: "Local models via Ollama. Free, private, no key.",
	},
	"anthropic": {
		Settings:    Settings{Kind: "anthropic", BaseURL: "https://api.anthropic.com", Model: "claude-haiku-5-5", KeyEnv: "ANTHROPIC_API_KEY", NeedKey: true},
		Description: "Claude via the Anthropic API (paid credits).",
	},
	"groq": {
		Settings:    Settings{Kind: "openai", BaseURL: "https://api.groq.com/openai/v1", Model: "llama-3.3-70b-versatile", KeyEnv: "GROQ_API_KEY", NeedKey: true},
		Description: "Groq cloud. Free tier with a free account key.",
	},
	"openrouter": {
		Settings:    Settings{Kind: "openai", BaseURL: "https://openrouter.ai/api/v1", Model: "meta-llama/llama-3.3-70b-instruct:free", KeyEnv: "OPENROUTER_API_KEY", NeedKey: true},
		Description: "OpenRouter. Has ':free' models with a free account key.",
	},
	"gemini": {
		Settings:    Settings{Kind: "openai", BaseURL: "https://generativelanguage.googleapis.com/v1beta/openai", Model: "gemini-2.5-flash", KeyEnv: "GEMINI_API_KEY", NeedKey: true},
		Description: "Google Gemini. Free tier with a free AI Studio key.",
	},
	"openai": {
		Settings:    Settings{Kind: "openai", BaseURL: "https://api.openai.com/v1", Model: "gpt-4.1-mini", KeyEnv: "OPENAI_API_KEY", NeedKey: true},
		Description: "OpenAI API (paid).",
	},
}

// PresetNames returns preset names in a stable order.
func PresetNames() []string {
	names := make([]string, 0, len(Presets))
	for n := range Presets {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

var httpClient = &http.Client{Timeout: 60 * time.Second}

// New builds a Provider from settings.
func New(name string, s Settings) (Provider, error) {
	if s.NeedKey && s.APIKey == "" {
		hint := s.KeyEnv
		if hint == "" {
			hint = "an API key"
		}
		return nil, fmt.Errorf("provider %q needs an API key: set %s or run `nlsh key set %s`", name, hint, name)
	}
	switch s.Kind {
	case "anthropic":
		return &Anthropic{name: name, s: s}, nil
	case "openai", "":
		return &OpenAICompat{name: name, s: s}, nil
	default:
		return nil, fmt.Errorf("provider %q: unknown kind %q (use \"anthropic\" or \"openai\")", name, s.Kind)
	}
}
