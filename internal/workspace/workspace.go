// Package workspace owns the app's folder layout on disk and safe file writes.
package workspace

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Workspace is the root folder of the app's data (default ~/ClipPublisher).
type Workspace struct {
	Root string
}

// New creates the workspace layout under root.
func New(root string) (*Workspace, error) {
	for _, d := range []string{"videos", "logs", "thumbs"} {
		if err := os.MkdirAll(filepath.Join(root, d), 0o755); err != nil {
			return nil, fmt.Errorf("create workspace: %w", err)
		}
	}
	return &Workspace{Root: root}, nil
}

// VideoDir is the folder of one video.
func (w *Workspace) VideoDir(slug string) string {
	return filepath.Join(w.Root, "videos", slug)
}

// NewVideoDir creates a fresh folder named after hint, adding -2, -3… when taken.
func (w *Workspace) NewVideoDir(hint string) (slug, dir string, err error) {
	base := Slugify(hint)
	for i := 1; ; i++ {
		slug = base
		if i > 1 {
			slug = fmt.Sprintf("%s-%d", base, i)
		}
		dir = w.VideoDir(slug)
		err = os.Mkdir(dir, 0o755)
		if err == nil {
			return slug, dir, nil
		}
		if !os.IsExist(err) {
			return "", "", fmt.Errorf("create video dir: %w", err)
		}
	}
}

func ScriptPath(dir string, n int) string {
	return filepath.Join(dir, fmt.Sprintf("script.v%d.json", n))
}
func NotesPath(dir string, n int) string { return filepath.Join(dir, fmt.Sprintf("notes.v%d.json", n)) }
func RenderMP4Path(dir string, n int) string {
	return filepath.Join(dir, "renders", fmt.Sprintf("v%d.mp4", n))
}
func RenderLogPath(dir string, n int) string {
	return filepath.Join(dir, "renders", fmt.Sprintf("render.v%d.log", n))
}
func PreviewMP4Path(dir string, n int) string {
	return filepath.Join(dir, "renders", fmt.Sprintf("preview.v%d", n), "preview.mp4")
}
func VoiceDir(dir string) string  { return filepath.Join(dir, "voice") }
func InputPath(dir string) string { return filepath.Join(dir, "input.md") }

// WriteFileAtomic writes data to a temp file next to path, then renames it over path.
func WriteFileAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, "."+filepath.Base(path)+".*.tmp")
	if err != nil {
		return err
	}
	tmp := f.Name()
	if _, err := f.Write(data); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
}

const maxSlug = 48

var vnBase = buildVNTable()

func buildVNTable() map[rune]rune {
	groups := map[rune]string{
		'a': "aàáảãạăằắẳẵặâầấẩẫậ",
		'e': "eèéẻẽẹêềếểễệ",
		'i': "iìíỉĩị",
		'o': "oòóỏõọôồốổỗộơờớởỡợ",
		'u': "uùúủũụưừứửữự",
		'y': "yỳýỷỹỵ",
		'd': "dđ",
	}
	m := map[rune]rune{}
	for base, chars := range groups {
		for _, c := range chars {
			m[c] = base
			m[[]rune(strings.ToUpper(string(c)))[0]] = base
		}
	}
	return m
}

// Slugify turns a Vietnamese title into a lowercase ASCII slug (max 48 chars).
func Slugify(s string) string {
	var b strings.Builder
	dash := false
	for _, r := range s {
		if base, ok := vnBase[r]; ok {
			r = base
		}
		if r >= 'A' && r <= 'Z' {
			r += 'a' - 'A'
		}
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			dash = false
		} else if !dash && b.Len() > 0 {
			b.WriteByte('-')
			dash = true
		}
	}
	out := strings.Trim(b.String(), "-")
	if len(out) > maxSlug {
		out = strings.TrimRight(out[:maxSlug], "-")
	}
	if out == "" {
		return "video"
	}
	return out
}

// ThumbPath is the cached thumbnail of a video.
func ThumbPath(root string, videoID int64) string {
	return filepath.Join(root, "thumbs", fmt.Sprintf("%d.jpg", videoID))
}
