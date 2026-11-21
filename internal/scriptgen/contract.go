// Package scriptgen turns the owner's free-form scene text into a history-shorts script.json by
// calling the Claude Code CLI, then repairs it until `history_shorts validate` accepts it.
package scriptgen

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// section returns the text from the line starting with `start` up to (not incl.) the next line
// starting with `end`, or to the end of the document.
func section(doc, start, end string) (string, bool) {
	lines := strings.Split(doc, "\n")
	from := -1
	for i, l := range lines {
		if from < 0 && strings.HasPrefix(l, start) {
			from = i
			continue
		}
		if from >= 0 && strings.HasPrefix(l, end) {
			return strings.Join(lines[from:i], "\n"), true
		}
	}
	if from < 0 {
		return "", false
	}
	return strings.Join(lines[from:], "\n"), true
}

// LoadContract reads the script.json contract (§6 + Appendix A) from make-video-free's spec.
func LoadContract(videoRepo string) (string, error) {
	b, err := os.ReadFile(filepath.Join(videoRepo, "docs", "raw_requirement.md"))
	if err != nil {
		return "", fmt.Errorf("không đọc được hợp đồng script.json: %w", err)
	}
	doc := string(b)
	data, ok1 := section(doc, "## 6. Data Contracts", "## 7.")
	appA, ok2 := section(doc, "## Appendix A", "## Appendix B")
	if !ok1 || !ok2 {
		return "", errors.New("không tìm thấy mục 6 / Phụ lục A trong make-video-free/docs/raw_requirement.md")
	}
	return data + "\n\n" + appA, nil
}

var exampleNames = []string{"hai_ba_trung", "vuong_trieu_nu"}

// LoadExamples returns up to two example scripts from make-video-free/examples.
func LoadExamples(videoRepo string) ([]string, error) {
	var out []string
	for _, n := range exampleNames {
		b, err := os.ReadFile(filepath.Join(videoRepo, "examples", n, "script.json"))
		if err == nil {
			out = append(out, string(b))
		}
	}
	if len(out) == 0 {
		return nil, errors.New("không tìm thấy script mẫu trong make-video-free/examples")
	}
	return out, nil
}
