// Package notify talks to macOS: desktop notifications and the Trash, via osascript.
package notify

import (
	"fmt"
	"os/exec"
	"strings"
)

// AppleScriptString quotes s as an AppleScript string literal (newlines become spaces).
func AppleScriptString(s string) string {
	s = strings.NewReplacer("\r\n", " ", "\n", " ", "\r", " ").Replace(s)
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	return `"` + s + `"`
}

// Send shows a macOS notification. Clicking it cannot open a specific video (osascript limit).
func Send(title, body string) error {
	script := fmt.Sprintf("display notification %s with title %s", AppleScriptString(body), AppleScriptString(title))
	return exec.Command("/usr/bin/osascript", "-e", script).Run()
}

// MoveToTrash moves path to the Finder Trash (recoverable).
func MoveToTrash(path string) error {
	script := fmt.Sprintf("tell application \"Finder\" to delete POSIX file %s", AppleScriptString(path))
	out, err := exec.Command("/usr/bin/osascript", "-e", script).CombinedOutput()
	if err != nil {
		return fmt.Errorf("không chuyển được vào Thùng rác: %v %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}
