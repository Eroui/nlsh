// Package config loads nlsh settings from ~/.config/nlsh/config.json, env
// vars and the macOS Keychain, and resolves them into provider settings.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"nlsh/internal/provider"
)

// ProviderOverride lets the config file tweak a preset or define a new
// OpenAI-compatible endpoint.
type ProviderOverride struct {
	Kind    string `json:"kind,omitempty"`
	BaseURL string `json:"base_url,omitempty"`
	Model   string `json:"model,omitempty"`
	KeyEnv  string `json:"key_env,omitempty"`
	NeedKey *bool  `json:"need_key,omitempty"`
}

// File is the on-disk config.
type File struct {
	Provider       string                      `json:"provider,omitempty"`
	Model          string                      `json:"model,omitempty"`
	SendDirListing *bool                       `json:"send_dir_listing,omitempty"`
	Providers      map[string]ProviderOverride `json:"providers,omitempty"`
}

// Resolved is the effective configuration.
type Resolved struct {
	ProviderName   string
	Settings       provider.Settings
	SendDirListing bool
	Path           string
}

// KeychainService is the Keychain service name keys are stored under.
const KeychainService = "nlsh"

// Path returns the config file path.
func Path() string {
	if p := os.Getenv("NLSH_CONFIG"); p != "" {
		return p
	}
	dir := os.Getenv("XDG_CONFIG_HOME")
	if dir == "" {
		home, _ := os.UserHomeDir()
		dir = filepath.Join(home, ".config")
	}
	return filepath.Join(dir, "nlsh", "config.json")
}

// Load reads the config file (missing is fine) and applies env overrides.
// providerFlag/modelFlag come from the command line and win over everything.
func Load(providerFlag, modelFlag string) (Resolved, error) {
	var f File
	path := Path()
	if b, err := os.ReadFile(path); err == nil {
		if err := json.Unmarshal(b, &f); err != nil {
			return Resolved{}, fmt.Errorf("config %s: %w", path, err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return Resolved{}, err
	}

	name := first(providerFlag, os.Getenv("NLSH_PROVIDER"), f.Provider, "ollama")
	preset, known := provider.Presets[name]
	ov, hasOv := f.Providers[name]
	if !known && !hasOv {
		return Resolved{}, fmt.Errorf("unknown provider %q (known: %s; or define it under \"providers\" in %s)",
			name, strings.Join(provider.PresetNames(), ", "), path)
	}
	s := preset.Settings
	if hasOv {
		s.Kind = first(ov.Kind, s.Kind)
		s.BaseURL = first(ov.BaseURL, s.BaseURL)
		s.Model = first(ov.Model, s.Model)
		s.KeyEnv = first(ov.KeyEnv, s.KeyEnv)
		if ov.NeedKey != nil {
			s.NeedKey = *ov.NeedKey
		}
	}
	s.BaseURL = first(os.Getenv("NLSH_BASE_URL"), s.BaseURL)
	s.Model = first(modelFlag, os.Getenv("NLSH_MODEL"), f.Model, s.Model)
	s.APIKey = lookupKey(name, s.KeyEnv)

	listing := true
	if f.SendDirListing != nil {
		listing = *f.SendDirListing
	}
	return Resolved{ProviderName: name, Settings: s, SendDirListing: listing, Path: path}, nil
}

// lookupKey checks NLSH_API_KEY, the provider's env var, then the Keychain.
func lookupKey(name, keyEnv string) string {
	if k := os.Getenv("NLSH_API_KEY"); k != "" {
		return k
	}
	if keyEnv != "" {
		if k := os.Getenv(keyEnv); k != "" {
			return k
		}
	}
	return KeychainGet(name)
}

// KeychainGet reads a key stored with `nlsh key set <provider>`.
func KeychainGet(name string) string {
	if os.Getenv("NLSH_NO_KEYCHAIN") != "" {
		return ""
	}
	out, err := exec.Command("security", "find-generic-password", "-s", KeychainService, "-a", name, "-w").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func first(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
