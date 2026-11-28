# nlsh

**Describe what you want in plain English. Get the shell command. Press Enter to run it.**

```
% ? show the 5 biggest files here
% du -ah . | sort -rh | head -n 5
[safe] Lists files and folders by size, largest first.
Enter: run   Esc: cancel   (or edit it first)
```

nlsh plugs into zsh, so it works in the terminal you already use: iTerm2,
Terminal.app, inside tmux, over SSH. There's no new app to learn.

## What it does

- Start a line with `?`, describe what you want, and press **Enter**.
- The line is replaced by a suggested command, with a one-line explanation
  and a risk tag (`safe`, `caution`, `DANGER`).
- Press **Enter** to run it, **Esc** to cancel (your request comes back so
  you can rephrase), or edit the command first.
- **Nothing runs without your Enter.** Keys typed while nlsh is thinking are
  discarded, so an impatient extra Enter can't run a command you haven't seen.
- Risky commands (`rm -rf`, `sudo`, force-push, `curl | sh`, ...) are flagged
  by a built-in check even if the model calls them safe. Caution commands show
  in yellow; dangerous ones show in red and need **Enter, then `y`** to run.
- Commands you run go into your normal shell history.
- Works with **any model**: a free local model through Ollama (the default),
  Claude, OpenAI, or any OpenAI-compatible API such as Groq, OpenRouter,
  Gemini or LM Studio.

## Install

Requirements: macOS or Linux, zsh, and [Go](https://go.dev/dl/) 1.24+ to build.

```bash
git clone git@github.com:Eroui/nlsh.git ~/nlsh
cd ~/nlsh
go build -o bin/nlsh ./cmd/nlsh
echo 'source ~/nlsh/shell/nlsh.zsh' >> ~/.zshrc
```

Open a new terminal tab (or run `source ~/.zshrc`).

The zsh script finds `bin/nlsh` next to itself. To put the CLI on your PATH
as well: `ln -s ~/nlsh/bin/nlsh /opt/homebrew/bin/nlsh` (or any PATH folder).

### Pick a model

**Free and local (default):** install [Ollama](https://ollama.com) and pull a model.

```bash
brew install ollama
brew services start ollama
ollama pull qwen2.5-coder:7b
```

That's all. nlsh uses Ollama at `localhost:11434` out of the box. On a Mac
with 16 GB+ of RAM, `qwen2.5-coder:14b` gives noticeably better answers
(set `"model": "qwen2.5-coder:14b"` in the config below).

**Cloud:** get a key from the provider and store it in the macOS Keychain:

```bash
nlsh key set groq        # prompts for the key, input hidden
```

Then select the provider in the config file (next section). Or skip the
Keychain and export the provider's env var (for example `GROQ_API_KEY`).

| Provider | Default model | Key | Cost |
| --- | --- | --- | --- |
| `ollama` | `qwen2.5-coder:7b` | none | free, runs on your machine |
| `groq` | `llama-3.3-70b-versatile` | `GROQ_API_KEY` | free tier |
| `gemini` | `gemini-2.5-flash` | `GEMINI_API_KEY` | free tier |
| `openrouter` | `meta-llama/llama-3.3-70b-instruct:free` | `OPENROUTER_API_KEY` | free models available |
| `anthropic` | `claude-haiku-5-5` | `ANTHROPIC_API_KEY` | paid |
| `openai` | `gpt-4.1-mini` | `OPENAI_API_KEY` | paid |

Run `nlsh providers` to see this list, with the active one starred.

## Configure

Settings live in `~/.config/nlsh/config.json`. Every field is optional.

```json
{
  "provider": "groq",
  "model": "llama-3.3-70b-versatile",
  "send_dir_listing": true
}
```

| Field | Meaning |
| --- | --- |
| `provider` | Which backend to use (default `ollama`). |
| `model` | Override the provider's default model. |
| `send_dir_listing` | Send the names of up to 60 visible entries in the current folder so the model can refer to your files; hidden (dot) entries are never sent and long names are shortened (default `true`). |
| `providers` | Override a built-in provider, or add your own (below). |

**Add any OpenAI-compatible endpoint** (LM Studio, a self-hosted server, etc.):

```json
{
  "provider": "lmstudio",
  "providers": {
    "lmstudio": {
      "kind": "openai",
      "base_url": "http://localhost:1234/v1",
      "model": "qwen2.5-7b-instruct",
      "need_key": false
    }
  }
}
```

`kind` is `openai` (OpenAI wire format) or `anthropic`. For a keyed endpoint,
add `"key_env": "MY_KEY_VAR"` and `"need_key": true`, or store the key with
`nlsh key set <name>`.

**Environment variables** override the file, handy for one-off tries:

| Variable | Effect |
| --- | --- |
| `NLSH_PROVIDER` | provider name |
| `NLSH_MODEL` | model name |
| `NLSH_BASE_URL` | endpoint URL |
| `NLSH_API_KEY` | API key for whichever provider is active |
| `NLSH_CONFIG` | path to a different config file |
| `NLSH_BIN` | path to the `nlsh` binary used by the zsh script |

Check what's in effect with `nlsh config`. Keys are looked up in this
order: `NLSH_API_KEY`, the provider's own env var, then the Keychain
(service `nlsh`, account = provider name). Remove a stored key with
`nlsh key delete <provider>`.

**Snappier Esc:** zsh waits `KEYTIMEOUT` (0.4 s by default) after Esc so
arrow keys and Alt shortcuts keep working. Add `KEYTIMEOUT=10` to
`~/.zshrc` to cut that to 0.1 s.

## Using the CLI directly

The zsh `?` prefix is one frontend. The same engine is available as a CLI:

```bash
nlsh suggest -- find files bigger than 100MB      # prints the command
nlsh suggest --format=json -- list docker containers
nlsh suggest --provider groq --model llama-3.1-8b-instant -- show disk usage
```

## Privacy

Each request sends only: your request text, the current folder path, the
names (not contents) of up to 60 visible entries in it (hidden dot entries
are never sent, and long names are shortened), your OS and your shell.
With Ollama, nothing leaves your machine. Set `"send_dir_listing": false`
to stop sending file names.

## How it's built

```
cmd/nlsh            CLI that every frontend calls
internal/engine     request + context -> suggestion, plus the local risk check
internal/provider   Provider interface; Anthropic and OpenAI-compatible backends
internal/config     config file, env vars, macOS Keychain
shell/nlsh.zsh      zsh frontend (the "?" prefix)
```

The engine doesn't know anything about zsh or terminals, so another
frontend (a tmux popup, a standalone app) only needs to call
`nlsh suggest --format=json`. A new model backend means implementing one
method, `Complete(ctx, Request) (string, error)`, in `internal/provider`.

## Tests

```bash
go test ./...
zsh test/widget_test.zsh    # drives a real interactive zsh against a fake model
```

## Uninstall

Remove the `source .../nlsh.zsh` line from `~/.zshrc`, delete the clone
and `~/.config/nlsh`, and run `nlsh key delete <provider>` for any stored keys.
