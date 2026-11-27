// Package engine turns a plain-English request into a suggested shell command.
// It knows nothing about zsh, terminals or UI: any frontend (shell widget,
// tmux popup, standalone app) calls Suggest and decides how to present it.
package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"runtime"
	"strings"

	"nlsh/internal/provider"
)

// Risk levels, ordered from least to most dangerous.
const (
	RiskSafe    = "safe"
	RiskCaution = "caution"
	RiskDanger  = "danger"
)

// Context describes where the command will run.
type Context struct {
	OS      string
	Shell   string
	Cwd     string
	Entries []string // names in Cwd, optional
}

// Suggestion is the engine's answer.
type Suggestion struct {
	Command     string `json:"command"`
	Explanation string `json:"explanation"`
	Risk        string `json:"risk"`
}

// GatherContext collects the local context for a request.
func GatherContext(sendListing bool) Context {
	c := Context{OS: osName(), Shell: shellName()}
	c.Cwd, _ = os.Getwd()
	if sendListing && c.Cwd != "" {
		if ents, err := os.ReadDir(c.Cwd); err == nil {
			for i, e := range ents {
				if i >= 60 {
					c.Entries = append(c.Entries, fmt.Sprintf("... (%d more)", len(ents)-i))
					break
				}
				n := e.Name()
				if e.IsDir() {
					n += "/"
				}
				c.Entries = append(c.Entries, n)
			}
		}
	}
	return c
}

func osName() string {
	if runtime.GOOS == "darwin" {
		return "macOS (BSD userland, Homebrew available)"
	}
	return runtime.GOOS
}

func shellName() string {
	if s := os.Getenv("NLSH_SHELL"); s != "" {
		return s
	}
	if s := os.Getenv("SHELL"); s != "" {
		parts := strings.Split(s, "/")
		return parts[len(parts)-1]
	}
	return "sh"
}

const systemPrompt = `You translate a plain-English request into ONE shell command.
Reply with ONLY a JSON object, no prose, no code fences:
{"command": "<the command>", "explanation": "<one short sentence>", "risk": "safe|caution|danger"}

Rules:
- Target shell: %s. Target OS: %s. Use flags that exist on this OS.
- Prefer one line; chain with pipes, && or ; when needed.
- Prefer read-only, non-destructive forms when the request is ambiguous.
- risk = "danger" if it deletes/overwrites data, needs sudo, changes system settings, or force-pushes; "caution" if it modifies files or state; otherwise "safe".
- If the request is not something a shell command can do, set "command" to "" and explain why in "explanation".`

// BuildRequest creates the model request for a query.
func BuildRequest(query string, c Context) provider.Request {
	var u strings.Builder
	fmt.Fprintf(&u, "Current directory: %s\n", c.Cwd)
	if len(c.Entries) > 0 {
		fmt.Fprintf(&u, "Directory contents: %s\n", strings.Join(c.Entries, ", "))
	}
	fmt.Fprintf(&u, "Request: %s", query)
	return provider.Request{
		System:    fmt.Sprintf(systemPrompt, c.Shell, c.OS),
		Messages:  []provider.Message{{Role: "user", Content: u.String()}},
		MaxTokens: 400,
	}
}

// Suggest asks the provider for a command and post-processes it.
func Suggest(ctx context.Context, p provider.Provider, query string, c Context) (Suggestion, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return Suggestion{}, fmt.Errorf("empty request")
	}
	raw, err := p.Complete(ctx, BuildRequest(query, c))
	if err != nil {
		return Suggestion{}, err
	}
	s := Parse(raw)
	s.Risk = maxRisk(normalizeRisk(s.Risk), LocalRisk(s.Command))
	return s, nil
}

var fenceRE = regexp.MustCompile("(?s)^```[a-zA-Z]*\\s*(.*?)\\s*```$")

// Parse extracts a Suggestion from a model reply, tolerating fences and
// replies that are just the bare command.
func Parse(raw string) Suggestion {
	t := strings.TrimSpace(raw)
	if m := fenceRE.FindStringSubmatch(t); m != nil {
		t = strings.TrimSpace(m[1])
	}
	if i, j := strings.Index(t, "{"), strings.LastIndex(t, "}"); i >= 0 && j > i {
		var s Suggestion
		if err := json.Unmarshal([]byte(t[i:j+1]), &s); err == nil && (s.Command != "" || s.Explanation != "") {
			s.Command = strings.TrimSpace(s.Command)
			return s
		}
	}
	// Fallback: treat the reply as a bare command.
	t = strings.TrimPrefix(t, "$ ")
	return Suggestion{Command: t}
}

var riskOrder = map[string]int{RiskSafe: 0, RiskCaution: 1, RiskDanger: 2}

func normalizeRisk(r string) string {
	r = strings.ToLower(strings.TrimSpace(r))
	if _, ok := riskOrder[r]; ok {
		return r
	}
	return RiskCaution // unknown means "be careful"
}

func maxRisk(a, b string) string {
	if riskOrder[b] > riskOrder[a] {
		return b
	}
	return a
}

type rule struct {
	re   *regexp.Regexp
	risk string
}

var rules = []rule{
	{regexp.MustCompile(`(^|[;&|]\s*|\s)sudo\s`), RiskDanger},
	{regexp.MustCompile(`\brm\s+(-[a-zA-Z]*[rRf]|--recursive|--force)`), RiskDanger},
	{regexp.MustCompile(`\b(mkfs|fdisk|diskutil\s+(erase|partition|zero))`), RiskDanger},
	{regexp.MustCompile(`\bdd\s+.*\bof=`), RiskDanger},
	{regexp.MustCompile(`>\s*/dev/(disk|sd|nvme)`), RiskDanger},
	{regexp.MustCompile(`\bgit\s+(push\s+.*(--force|-f\b)|reset\s+--hard|clean\s+-[a-zA-Z]*f)`), RiskDanger},
	{regexp.MustCompile(`\b(curl|wget)\b.*\|\s*(sudo\s+)?(ba|z)?sh\b`), RiskDanger},
	{regexp.MustCompile(`\b(shutdown|reboot|halt)\b`), RiskDanger},
	{regexp.MustCompile(`\bchmod\s+-R|\bchown\s+-R`), RiskDanger},
	{regexp.MustCompile(`\bkill(all)?\s`), RiskCaution},
	{regexp.MustCompile(`\b(rm|mv|cp|chmod|chown|ln|truncate)\s`), RiskCaution},
	{regexp.MustCompile(`(^|[^>2&])>\s*[^&\s]`), RiskCaution}, // overwrite redirect
	{regexp.MustCompile(`\b(brew|npm|pip3?|cargo|go)\s+(install|uninstall|remove|upgrade)`), RiskCaution},
	{regexp.MustCompile(`\bgit\s+(push|commit|checkout|rebase|merge|stash\s+drop)`), RiskCaution},
}

// LocalRisk is a model-independent safety net: it never lowers risk.
func LocalRisk(cmd string) string {
	r := RiskSafe
	for _, ru := range rules {
		if ru.re.MatchString(cmd) {
			r = maxRisk(r, ru.risk)
		}
	}
	return r
}
