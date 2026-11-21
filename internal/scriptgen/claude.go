package scriptgen

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"clip-publisher/internal/scriptmodel"
)

// Output is what Claude returns.
type Output struct {
	Script  json.RawMessage   `json:"script"`
	Notes   scriptmodel.Notes `json:"notes"`
	Summary string            `json:"summary"`
}

// ParseEnvelope unwraps `claude -p --output-format json` output. Output also carries stderr
// (Node warnings), so the first JSON object that decodes as an envelope wins and anything around
// it is ignored.
func ParseEnvelope(stdout []byte) (string, error) {
	type envelope struct {
		Type    string  `json:"type"`
		IsError bool    `json:"is_error"`
		Result  *string `json:"result"`
	}
	s := string(stdout)
	var lastErr error
	for i := strings.IndexByte(s, '{'); i >= 0; {
		var env envelope
		err := json.NewDecoder(strings.NewReader(s[i:])).Decode(&env)
		if err == nil && env.Result != nil {
			if env.IsError {
				return "", fmt.Errorf("Claude báo lỗi: %s", *env.Result)
			}
			return *env.Result, nil
		}
		if err != nil {
			lastErr = err
		}
		next := strings.IndexByte(s[i+1:], '{')
		if next < 0 {
			break
		}
		i += next + 1
	}
	if lastErr == nil {
		lastErr = errors.New("không có JSON")
	}
	return "", fmt.Errorf("Claude trả về dữ liệu không đọc được: %w", lastErr)
}

// ExtractOutput finds the JSON object in Claude's reply (plain, fenced, or surrounded by prose).
func ExtractOutput(text string) (*Output, error) {
	t := strings.TrimSpace(text)
	if i := strings.Index(t, "```"); i >= 0 {
		rest := t[i+3:]
		if nl := strings.Index(rest, "\n"); nl >= 0 {
			rest = rest[nl+1:]
		}
		if j := strings.LastIndex(rest, "```"); j >= 0 {
			rest = rest[:j]
		}
		t = strings.TrimSpace(rest)
	}
	start, end := strings.Index(t, "{"), strings.LastIndex(t, "}")
	if start < 0 || end <= start {
		return nil, errors.New("Claude không trả về JSON")
	}
	var o Output
	if err := json.Unmarshal([]byte(t[start:end+1]), &o); err != nil {
		return nil, fmt.Errorf("JSON của Claude không hợp lệ: %w", err)
	}
	if len(o.Script) == 0 || string(o.Script) == "null" {
		return nil, errors.New(`JSON của Claude thiếu "script"`)
	}
	if o.Notes.Slides == nil {
		o.Notes.Slides = map[string]scriptmodel.SlideNote{}
	}
	return &o, nil
}
