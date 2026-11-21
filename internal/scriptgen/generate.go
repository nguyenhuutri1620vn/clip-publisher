package scriptgen

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"clip-publisher/internal/config"
	"clip-publisher/internal/runner"
)

// Validator checks a script and returns validator errors (nil = valid).
type Validator func(ctx context.Context, script []byte) (errs []string, err error)

// InvalidScriptError means Claude could not produce a valid script within MaxRepairs rounds.
type InvalidScriptError struct {
	Errors []string
	Last   *Output
	Rounds int
}

func (e *InvalidScriptError) Error() string {
	return fmt.Sprintf("Script vẫn lỗi sau %d lần sửa", e.Rounds)
}

// Generator calls Claude to create or edit scripts.
type Generator struct {
	Runner     runner.Runner
	Settings   config.Settings
	Contract   string
	Examples   []string
	Timeout    time.Duration
	MaxRepairs int
}

// Generate asks Claude, validates, and feeds errors back up to MaxRepairs times.
func (g *Generator) Generate(ctx context.Context, r Request, validate Validator) (*Output, error) {
	var last *Output
	var errs []string
	for round := 0; round <= g.MaxRepairs; round++ {
		out, err := g.call(ctx, BuildPrompt(g.Contract, g.Examples, r))
		if err != nil {
			return nil, err
		}
		pretty, err := prettyJSON(out.Script)
		if err != nil {
			return nil, err
		}
		out.Script = pretty
		last = out
		errs, err = validate(ctx, out.Script)
		if err != nil {
			return nil, err
		}
		if len(errs) == 0 {
			return out, nil
		}
		notes := out.Notes
		r.Current, r.Notes, r.Errors = out.Script, &notes, errs
	}
	return nil, &InvalidScriptError{Errors: errs, Last: last, Rounds: g.MaxRepairs}
}

func (g *Generator) call(ctx context.Context, prompt string) (*Output, error) {
	c := runner.Cmd{
		Name:    g.Settings.ClaudeBin,
		Args:    []string{"-p", "--output-format", "json", "--tools", "", "--no-session-persistence"},
		Stdin:   prompt,
		Dir:     os.TempDir(), // neutral cwd: no project CLAUDE.md pulled into the prompt
		Env:     runner.ChildEnv(os.Environ(), filepath.Dir(g.Settings.ClaudeBin)),
		Timeout: g.Timeout,
	}
	res, err := g.Runner.Run(ctx, c, nil)
	switch {
	case errors.Is(err, runner.ErrTimeout):
		return nil, fmt.Errorf("Claude chạy quá thời gian (%s)", g.Timeout)
	case err != nil:
		return nil, fmt.Errorf("không chạy được Claude: %w", err)
	case res.ExitCode != 0:
		return nil, fmt.Errorf("Claude thoát với mã %d: %s", res.ExitCode, lastLine(res.Output))
	}
	text, err := ParseEnvelope([]byte(res.Output))
	if err != nil {
		return nil, err
	}
	return ExtractOutput(text)
}

func prettyJSON(raw json.RawMessage) (json.RawMessage, error) {
	var buf bytes.Buffer
	if err := json.Indent(&buf, raw, "", "  "); err != nil {
		return nil, fmt.Errorf("script của Claude không phải JSON hợp lệ: %w", err)
	}
	buf.WriteByte('\n')
	return buf.Bytes(), nil
}

func lastLine(s string) string {
	lines := bytes.Split(bytes.TrimSpace([]byte(s)), []byte("\n"))
	return string(lines[len(lines)-1])
}
