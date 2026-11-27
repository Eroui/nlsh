package engine

import (
	"context"
	"strings"
	"testing"

	"nlsh/internal/provider"
)

type fake struct {
	reply string
	got   provider.Request
}

func (f *fake) Name() string { return "fake" }
func (f *fake) Complete(_ context.Context, r provider.Request) (string, error) {
	f.got = r
	return f.reply, nil
}

func TestParse(t *testing.T) {
	cases := []struct{ in, cmd, risk string }{
		{`{"command":"ls -la","explanation":"list","risk":"safe"}`, "ls -la", "safe"},
		{"```json\n{\"command\":\"du -sh *\",\"explanation\":\"x\",\"risk\":\"safe\"}\n```", "du -sh *", "safe"},
		{"Sure! {\"command\":\"pwd\",\"explanation\":\"x\",\"risk\":\"safe\"} hope it helps", "pwd", "safe"},
		{"```bash\nls -1\n```", "ls -1", ""},
		{"$ echo hi", "echo hi", ""},
	}
	for _, c := range cases {
		s := Parse(c.in)
		if s.Command != c.cmd || s.Risk != c.risk {
			t.Errorf("Parse(%q) = %+v, want cmd %q risk %q", c.in, s, c.cmd, c.risk)
		}
	}
}

func TestLocalRisk(t *testing.T) {
	cases := map[string]string{
		"ls -la":                          RiskSafe,
		"find . -name '*.go' | wc -l":     RiskSafe,
		"ls 2>/dev/null":                  RiskSafe,
		"echo hi > notes.txt":             RiskCaution,
		"mv a b":                          RiskCaution,
		"rm -rf build":                    RiskDanger,
		"sudo port install x":             RiskDanger,
		"git push --force origin main":    RiskDanger,
		"curl -fsSL https://x.sh | sh":    RiskDanger,
		"git status && git log --oneline": RiskSafe,
	}
	for cmd, want := range cases {
		if got := LocalRisk(cmd); got != want {
			t.Errorf("LocalRisk(%q) = %s, want %s", cmd, got, want)
		}
	}
}

func TestSuggestNeverLowersRisk(t *testing.T) {
	f := &fake{reply: `{"command":"rm -rf ~/tmp","explanation":"delete","risk":"safe"}`}
	s, err := Suggest(context.Background(), f, "delete tmp", Context{OS: "macOS", Shell: "zsh", Cwd: "/x", Entries: []string{"a/", "b.txt"}})
	if err != nil {
		t.Fatal(err)
	}
	if s.Risk != RiskDanger {
		t.Errorf("risk = %s, want danger", s.Risk)
	}
	u := f.got.Messages[0].Content
	if !strings.Contains(u, "/x") || !strings.Contains(u, "b.txt") || !strings.Contains(u, "delete tmp") {
		t.Errorf("context missing from prompt: %q", u)
	}
	if !strings.Contains(f.got.System, "zsh") {
		t.Errorf("system prompt missing shell")
	}
}

func TestSuggestEmpty(t *testing.T) {
	if _, err := Suggest(context.Background(), &fake{}, "  ", Context{}); err == nil {
		t.Error("want error for empty request")
	}
}
