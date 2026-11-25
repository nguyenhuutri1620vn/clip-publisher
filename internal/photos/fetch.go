package photos

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"

	"clip-publisher/internal/workspace"
)

const maxBytes = 10 << 20

// File is a stored photo, relative to the video folder.
type File struct {
	Rel    string // e.g. "photos/<sha256>.jpg"
	SHA256 string
	Credit string
}

// Ref records which stored photo a slide uses, in photos/index.json.
type Ref struct {
	SHA256  string `json:"sha256"`
	File    string `json:"file"`
	Credit  string `json:"credit"`
	PageURL string `json:"pageUrl"`
}

// Download fetches cand.FullURL into dir/photos/<sha256>.<ext>. It refuses files above 10 MB and
// anything that is not JPEG or PNG; nothing is written in those cases.
func (c *Client) Download(ctx context.Context, cand Candidate, dir string) (File, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, cand.FullURL, nil)
	if err != nil {
		return File{}, err
	}
	req.Header.Set("User-Agent", userAgent)
	resp, err := c.client().Do(req)
	if err != nil {
		return File{}, fmt.Errorf("Không tải được ảnh: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return File{}, fmt.Errorf("Không tải được ảnh (HTTP %d)", resp.StatusCode)
	}

	mt, _, _ := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	var ext string
	switch mt {
	case "image/jpeg":
		ext = ".jpg"
	case "image/png":
		ext = ".png"
	default:
		return File{}, errors.New("Ảnh không hợp lệ: chỉ nhận JPEG hoặc PNG")
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, maxBytes+1))
	if err != nil {
		return File{}, fmt.Errorf("Không tải được ảnh: %w", err)
	}
	if len(data) > maxBytes {
		return File{}, errors.New("Ảnh quá lớn (trên 10 MB)")
	}

	sum := sha256.Sum256(data)
	name := hex.EncodeToString(sum[:])
	rel := "photos/" + name + ext
	full := filepath.Join(dir, rel)
	if _, err := os.Stat(full); os.IsNotExist(err) {
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			return File{}, err
		}
		if err := workspace.WriteFileAtomic(full, data); err != nil {
			return File{}, err
		}
	}
	return File{Rel: rel, SHA256: name, Credit: Credit(cand)}, nil
}

func indexPath(dir string) string { return filepath.Join(dir, "photos", "index.json") }

// Index reads photos/index.json: slide id → photos chosen for it. A missing file is empty.
func Index(dir string) (map[string][]Ref, error) {
	m := map[string][]Ref{}
	b, err := os.ReadFile(indexPath(dir))
	if os.IsNotExist(err) {
		return m, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, fmt.Errorf("photos/index.json không đọc được: %w", err)
	}
	if m == nil {
		m = map[string][]Ref{}
	}
	return m, nil
}

// AddRef records ref for slideID, once per SHA-256, and rewrites index.json atomically.
func AddRef(dir, slideID string, ref Ref) error {
	m, err := Index(dir)
	if err != nil {
		return err
	}
	for _, r := range m[slideID] {
		if r.SHA256 == ref.SHA256 {
			return nil
		}
	}
	m[slideID] = append(m[slideID], ref)
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(indexPath(dir)), 0o755); err != nil {
		return err
	}
	return workspace.WriteFileAtomic(indexPath(dir), b)
}
