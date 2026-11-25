package scriptgen

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"clip-publisher/internal/config"
	"clip-publisher/internal/runner"
	"clip-publisher/internal/scriptmodel"
	"clip-publisher/internal/workspace"
)

func fakeClaude() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..", "testdata", "fakes", "fake_claude.sh")
}

const contractDoc = `# history-shorts
## 5. Functional
nope
## 6. Data Contracts
SCRIPT CONTRACT BODY
## 7. CLI
cli stuff
## Appendix A — v2.1 storyboard
NARRATION RULES
## Appendix B — Era packs
era stuff
`

func TestLoadContract(t *testing.T) {
	repo := t.TempDir()
	os.MkdirAll(filepath.Join(repo, "docs"), 0o755)
	os.WriteFile(filepath.Join(repo, "docs", "raw_requirement.md"), []byte(contractDoc), 0o644)
	c, err := LoadContract(repo)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(c, "SCRIPT CONTRACT BODY") || !strings.Contains(c, "NARRATION RULES") {
		t.Fatalf("missing parts: %q", c)
	}
	if strings.Contains(c, "## 7.") || strings.Contains(c, "era stuff") || strings.Contains(c, "cli stuff") {
		t.Fatalf("extra parts: %q", c)
	}
	os.WriteFile(filepath.Join(repo, "docs", "raw_requirement.md"), []byte("## 6. Data Contracts\nx\n## 7. CLI\n"), 0o644)
	if _, err := LoadContract(repo); err == nil {
		t.Fatal("missing appendix accepted")
	}
}

func TestLoadExamples(t *testing.T) {
	repo := t.TempDir()
	if _, err := LoadExamples(repo); err == nil {
		t.Fatal("no examples accepted")
	}
	os.MkdirAll(filepath.Join(repo, "examples", "vuong_trieu_nu"), 0o755)
	os.WriteFile(filepath.Join(repo, "examples", "vuong_trieu_nu", "script.json"), []byte(`{"x":1}`), 0o644)
	ex, err := LoadExamples(repo)
	if err != nil || len(ex) != 1 || ex[0] != `{"x":1}` {
		t.Fatalf("%v %v", ex, err)
	}
	os.MkdirAll(filepath.Join(repo, "tests", "data"), 0o755)
	os.WriteFile(filepath.Join(repo, "tests", "data", "warrior_script.json"), []byte(`{"role":"viet_general"}`), 0o644)
	ex, err = LoadExamples(repo)
	if err != nil || len(ex) != 2 || ex[1] != `{"role":"viet_general"}` {
		t.Fatalf("male-role example not loaded: %v %v", ex, err)
	}
}

func TestBuildPromptSections(t *testing.T) {
	p := BuildPrompt("CONTRACT", []string{"EX1"}, Request{Input: "CẢNH 1\nLời đọc: xin chào"})
	for _, want := range []string{"CONTRACT", "EX1", "INPUT", "CẢNH 1", "[CẦN KIỂM TRA", "\"script\"", "time_hint", `"2.4"`,
		"viet_general", "northern_soldier", "leader"} {
		if !strings.Contains(p, want) {
			t.Errorf("new prompt lacks %q", want)
		}
	}
	if strings.Contains(p, "CURRENT SCRIPT") {
		t.Error("new prompt has CURRENT SCRIPT")
	}
	var chat []workspace.ChatMessage
	for i := 0; i < 12; i++ {
		chat = append(chat, workspace.ChatMessage{Role: "user", Text: "msg" + string(rune('a'+i))})
	}
	notes := scriptmodel.Notes{Slides: map[string]scriptmodel.SlideNote{"hook": {Note: "cần ảnh thật"}}}
	p = BuildPrompt("C", nil, Request{Current: []byte(`{"title":"x"}`), Notes: &notes, Chat: chat,
		Instruction: "cảnh 2 ngắn lại", Errors: []string{"script.json › slides[0].id: bad"}})
	for _, want := range []string{"CURRENT SCRIPT", `{"title":"x"}`, "cần ảnh thật", "INSTRUCTION", "cảnh 2 ngắn lại",
		"script.json › slides[0].id: bad", "msgl", "msgc"} {
		if !strings.Contains(p, want) {
			t.Errorf("edit prompt lacks %q", want)
		}
	}
	if strings.Contains(p, "msga") || strings.Contains(p, "msgb") {
		t.Error("chat not trimmed to last 10")
	}
}

func TestParseEnvelope(t *testing.T) {
	got, err := ParseEnvelope([]byte(`{"type":"result","is_error":false,"result":"X"}`))
	if err != nil || got != "X" {
		t.Fatalf("%q %v", got, err)
	}
	if _, err := ParseEnvelope([]byte(`{"type":"result","is_error":true,"result":"rate limited"}`)); err == nil ||
		!strings.Contains(err.Error(), "rate limited") {
		t.Fatalf("is_error: %v", err)
	}
	if _, err := ParseEnvelope([]byte(`not json`)); err == nil {
		t.Fatal("garbage accepted")
	}
}

const okOutput = `{"script":{"schema_version":"2.2","title":"T","slides":[]},"notes":{"slides":{"a":{"note":"n"}}},"summary":"S"}`

func TestExtractOutput(t *testing.T) {
	for name, text := range map[string]string{
		"plain":  okOutput,
		"fenced": "```json\n" + okOutput + "\n```",
		"prose":  "Đây là script:\n" + okOutput + "\nChúc vui",
	} {
		o, err := ExtractOutput(text)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if o.Summary != "S" || o.Notes.Slides["a"].Note != "n" || !strings.Contains(string(o.Script), `"title":"T"`) {
			t.Fatalf("%s: %+v", name, o)
		}
	}
	if _, err := ExtractOutput("không có json"); err == nil {
		t.Fatal("no braces accepted")
	}
	if _, err := ExtractOutput(`{"notes":{}}`); err == nil {
		t.Fatal("missing script accepted")
	}
}

func envelope(t *testing.T, dir, name, result string) string {
	t.Helper()
	b, _ := json.Marshal(map[string]any{"type": "result", "is_error": false, "result": result})
	p := filepath.Join(dir, name)
	os.WriteFile(p, b, 0o644)
	return p
}

func newGen(t *testing.T) *Generator {
	return &Generator{
		Runner:     runner.ExecRunner{},
		Settings:   config.Settings{ClaudeBin: fakeClaude()},
		Contract:   "C",
		Timeout:    10 * time.Second,
		MaxRepairs: 3,
	}
}

func TestGenerateRepairs(t *testing.T) {
	dir := t.TempDir()
	bad := `{"script":{"title":"BAD","slides":[]},"notes":{},"summary":"v1"}`
	good := `{"script":{"title":"GOOD","slides":[]},"notes":{},"summary":"v2"}`
	t.Setenv("FAKE_CLAUDE_STATE", filepath.Join(dir, "n"))
	t.Setenv("FAKE_CLAUDE_LOG", filepath.Join(dir, "in"))
	t.Setenv("FAKE_CLAUDE_REPLY_1", envelope(t, dir, "r1", bad))
	t.Setenv("FAKE_CLAUDE_REPLY_2", envelope(t, dir, "r2", good))
	validate := func(_ context.Context, script []byte) ([]string, error) {
		if strings.Contains(string(script), "BAD") {
			return []string{"script.json › slides: must not be empty"}, nil
		}
		return nil, nil
	}
	out, err := newGen(t).Generate(context.Background(), Request{Input: "x"}, validate)
	if err != nil || out.Summary != "v2" {
		t.Fatalf("%+v %v", out, err)
	}
	second, _ := os.ReadFile(filepath.Join(dir, "in.2"))
	if !strings.Contains(string(second), "must not be empty") || !strings.Contains(string(second), "BAD") {
		t.Fatalf("second prompt lacks errors/current: %s", second)
	}
}

func TestGenerateGivesUp(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("FAKE_CLAUDE_STATE", filepath.Join(dir, "n"))
	t.Setenv("FAKE_CLAUDE_REPLY", envelope(t, dir, "r", okOutput))
	validate := func(context.Context, []byte) ([]string, error) { return []string{"e1"}, nil }
	_, err := newGen(t).Generate(context.Background(), Request{Input: "x"}, validate)
	var inv *InvalidScriptError
	if !errors.As(err, &inv) || len(inv.Errors) != 1 || inv.Errors[0] != "e1" || inv.Last == nil {
		t.Fatalf("%v", err)
	}
	calls, _ := os.ReadFile(filepath.Join(dir, "n"))
	if strings.TrimSpace(string(calls)) != "4" {
		t.Fatalf("calls %s", calls)
	}
}

func TestGenerateClaudeFails(t *testing.T) {
	t.Setenv("FAKE_CLAUDE_EXIT", "1")
	_, err := newGen(t).Generate(context.Background(), Request{Input: "x"},
		func(context.Context, []byte) ([]string, error) { return nil, nil })
	if err == nil || !strings.Contains(err.Error(), "Claude") {
		t.Fatalf("%v", err)
	}
}

func TestGenerateTimeout(t *testing.T) {
	t.Setenv("FAKE_CLAUDE_SLEEP", "5")
	g := newGen(t)
	g.Timeout = 300 * time.Millisecond
	_, err := g.Generate(context.Background(), Request{Input: "x"},
		func(context.Context, []byte) ([]string, error) { return nil, nil })
	if err == nil || !strings.Contains(err.Error(), "quá thời gian") {
		t.Fatalf("%v", err)
	}
}

// Review #5: stderr warnings around the JSON envelope must not break parsing.
func TestParseEnvelopeWithNoise(t *testing.T) {
	cases := []string{
		`{"type":"result","is_error":false,"result":"X"}` + "\n(node:123) [DEP0040] DeprecationWarning: The `punycode` module is deprecated.\n",
		"Warning {weird} config\n" + `{"type":"result","is_error":false,"result":"X"}`,
	}
	for _, c := range cases {
		got, err := ParseEnvelope([]byte(c))
		if err != nil || got != "X" {
			t.Fatalf("%q → %q %v", c, got, err)
		}
	}
}
