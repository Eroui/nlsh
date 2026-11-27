# nlsh: plain English in, shell command out

Type `? <what you want>` at your zsh prompt and press Enter. The line turns
into a suggested command with a one-line explanation and a risk tag.
Press **Enter** to run it, **Esc** to cancel (your request comes back so you
can rephrase), or edit it first. Nothing ever runs without your Enter.

```
READY> ? show the 5 biggest files here
READY> du -sh * | sort -rh | head -5
[safe] Lists sizes of items here, largest first.
Enter: run   Esc: cancel   (or edit it first)
```

Works anywhere zsh runs: iTerm2, Terminal.app, inside tmux.

## Layout

| Path | What |
| --- | --- |
| `internal/engine` | Turns a request + context into a suggestion. No UI or shell knowledge, so any frontend can use it. Includes a local safety net that flags risky commands even if the model doesn't. |
| `internal/provider` | Model interface (`Complete`) and backends: Anthropic, and any OpenAI-compatible API (Ollama, Groq, OpenRouter, Gemini, OpenAI, LM Studio). |
| `internal/config` | Config file, env vars, macOS Keychain. |
| `cmd/nlsh` | The CLI every frontend calls. |
| `shell/nlsh.zsh` | The zsh frontend (the `?` prefix). |

A future tmux popup or standalone app only needs to call
`nlsh suggest --format=json -- <request>`.

## Setup

```bash
go build -o bin/nlsh ./cmd/nlsh
echo 'source ~/Desktop/git/terminal/shell/nlsh.zsh' >> ~/.zshrc
```

### Pick a model

Default is a free local model through [Ollama](https://ollama.com):

```bash
brew install ollama && brew services start ollama
ollama pull qwen2.5-coder:7b
```

Or a free cloud tier (needs a free account key), or Claude once you have credits:

```bash
nlsh providers            # list options
nlsh key set groq         # stores the key in the macOS Keychain
```

Then choose it in `~/.config/nlsh/config.json`:

```json
{ "provider": "groq" }
```

Other options: `"model": "..."`, `"send_dir_listing": false` (stop sending
file names in the current folder), and custom OpenAI-compatible endpoints:

```json
{ "provider": "lmstudio",
  "providers": { "lmstudio": { "kind": "openai", "base_url": "http://localhost:1234/v1", "model": "qwen2.5-7b" } } }
```

Env overrides: `NLSH_PROVIDER`, `NLSH_MODEL`, `NLSH_BASE_URL`, `NLSH_API_KEY`.

## Notes

- What's sent to the model: your request, the current folder path, the names
  of up to 60 entries in it, your OS and shell. Nothing else.
- Esc reacts after zsh's `KEYTIMEOUT` (0.4s by default) so arrow keys and
  Alt shortcuts keep working. `KEYTIMEOUT=10` in `~/.zshrc` makes it snappier.

## Tests

```bash
go test ./...
zsh test/widget_test.zsh   # drives a real zsh with a fake model
```
