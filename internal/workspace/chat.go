package workspace

import (
	"bufio"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"
)

// ChatMessage is one line of a video's conversation with Claude.
type ChatMessage struct {
	Role string    `json:"role"`
	Text string    `json:"text"`
	At   time.Time `json:"at"`
}

func chatPath(dir string) string { return filepath.Join(dir, "chat.jsonl") }

// AppendChat adds one message to dir/chat.jsonl.
func AppendChat(dir string, m ChatMessage) error {
	line, err := json.Marshal(m)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(chatPath(dir), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	if _, err := f.Write(append(line, '\n')); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

// ReadChat returns all messages in order; a missing file is an empty chat.
func ReadChat(dir string) ([]ChatMessage, error) {
	f, err := os.Open(chatPath(dir))
	if errors.Is(err, os.ErrNotExist) {
		return []ChatMessage{}, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	out := []ChatMessage{}
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64*1024), 4*1024*1024)
	for sc.Scan() {
		if len(sc.Bytes()) == 0 {
			continue
		}
		var m ChatMessage
		if err := json.Unmarshal(sc.Bytes(), &m); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, sc.Err()
}
