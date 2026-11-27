// Command nlsh turns plain English into a shell command suggestion.
//
//	nlsh suggest -- list the 5 biggest files here
//	nlsh providers
//	nlsh config
//	nlsh key set groq
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"nlsh/internal/config"
	"nlsh/internal/engine"
	"nlsh/internal/provider"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	var err error
	switch os.Args[1] {
	case "suggest", "s":
		err = cmdSuggest(os.Args[2:])
	case "providers":
		cmdProviders()
	case "config":
		err = cmdConfig()
	case "key":
		err = cmdKey(os.Args[2:])
	case "-h", "--help", "help":
		usage()
	default:
		// Allow `nlsh list files` as a shortcut for suggest.
		err = cmdSuggest(os.Args[1:])
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "nlsh:", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `nlsh: plain English -> shell command

Usage:
  nlsh suggest [--provider P] [--model M] [--format text|json|zsh] -- <request>
  nlsh providers          list built-in providers
  nlsh config             show effective settings
  nlsh key set <provider> store an API key in the macOS Keychain
  nlsh key delete <provider>

In zsh (after sourcing shell/nlsh.zsh): type "? <request>" and press Enter.
`)
}

func cmdSuggest(args []string) error {
	fs := flag.NewFlagSet("suggest", flag.ContinueOnError)
	prov := fs.String("provider", "", "provider name (default from config, else ollama)")
	model := fs.String("model", "", "model name override")
	format := fs.String("format", "text", "output: text, json or zsh")
	timeout := fs.Duration("timeout", 45*time.Second, "request timeout")
	if err := fs.Parse(args); err != nil {
		return err
	}
	query := strings.Join(fs.Args(), " ")

	cfg, err := config.Load(*prov, *model)
	if err != nil {
		return err
	}
	p, err := provider.New(cfg.ProviderName, cfg.Settings)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	s, err := engine.Suggest(ctx, p, query, engine.GatherContext(cfg.SendDirListing))
	if err != nil {
		return err
	}

	switch *format {
	case "json":
		return json.NewEncoder(os.Stdout).Encode(s)
	case "zsh":
		// NUL-separated so commands containing newlines or quotes survive.
		fmt.Printf("%s\x00%s\x00%s", s.Command, s.Explanation, s.Risk)
	default:
		if s.Command == "" {
			fmt.Fprintln(os.Stderr, s.Explanation)
			return fmt.Errorf("no command suggested")
		}
		fmt.Println(s.Command)
		if s.Explanation != "" {
			fmt.Fprintf(os.Stderr, "# %s [%s]\n", s.Explanation, s.Risk)
		}
	}
	return nil
}

func cmdProviders() {
	cur := ""
	if cfg, err := config.Load("", ""); err == nil {
		cur = cfg.ProviderName
	}
	for _, n := range provider.PresetNames() {
		p := provider.Presets[n]
		mark := " "
		if n == cur {
			mark = "*"
		}
		key := "no key needed"
		if p.NeedKey {
			key = "key: " + p.KeyEnv
		}
		fmt.Printf("%s %-11s %-40s %s\n  %s\n", mark, n, p.Model, key, p.Description)
	}
}

func cmdConfig() error {
	cfg, err := config.Load("", "")
	if err != nil {
		return err
	}
	key := "not set"
	if cfg.Settings.APIKey != "" {
		key = "set"
	}
	fmt.Printf("config file:      %s\nprovider:         %s\nmodel:            %s\nbase url:         %s\napi key:          %s\nsend dir listing: %v\n",
		cfg.Path, cfg.ProviderName, cfg.Settings.Model, cfg.Settings.BaseURL, key, cfg.SendDirListing)
	return nil
}

func cmdKey(args []string) error {
	if len(args) != 2 || (args[0] != "set" && args[0] != "delete") {
		return fmt.Errorf("usage: nlsh key set|delete <provider>")
	}
	var c *exec.Cmd
	if args[0] == "set" {
		// `-w` as the last argument makes `security` prompt for the secret,
		// so it never appears in shell history or the process list.
		fmt.Fprintf(os.Stderr, "Paste the %s API key at the prompt (input is hidden).\n", args[1])
		c = exec.Command("security", "add-generic-password", "-U", "-s", config.KeychainService, "-a", args[1], "-w")
	} else {
		c = exec.Command("security", "delete-generic-password", "-s", config.KeychainService, "-a", args[1])
	}
	c.Stdin, c.Stdout, c.Stderr = os.Stdin, os.Stderr, os.Stderr
	return c.Run()
}
