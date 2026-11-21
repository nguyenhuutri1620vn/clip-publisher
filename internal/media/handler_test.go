package media

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

type fakeResolver struct{ dir, mp4, thumb string }

func (f fakeResolver) RenderPath(id int64) (string, error) {
	if id == 7 {
		return f.mp4, nil
	}
	return "", errors.New("not found")
}
func (f fakeResolver) VideoDir(id int64) (string, error) {
	if id == 1 {
		return f.dir, nil
	}
	return "", errors.New("not found")
}
func (f fakeResolver) ThumbPath(_ context.Context, id int64) (string, error) {
	if id == 1 {
		return f.thumb, nil
	}
	return "", errors.New("none")
}

func setup(t *testing.T) http.Handler {
	root := t.TempDir()
	dir := filepath.Join(root, "videos", "x")
	os.MkdirAll(filepath.Join(dir, "renders", "preview.v1", "slides"), 0o755)
	os.WriteFile(filepath.Join(dir, "renders", "preview.v1", "slides", "01.png"), []byte("PNGDATA"), 0o644)
	mp4 := filepath.Join(root, "v1.mp4")
	os.WriteFile(mp4, []byte("0123456789"), 0o644)
	thumb := filepath.Join(root, "t.jpg")
	os.WriteFile(thumb, []byte("JPG"), 0o644)
	os.WriteFile(filepath.Join(root, "secret.txt"), []byte("SECRET"), 0o644)
	return Handler(fakeResolver{dir: dir, mp4: mp4, thumb: thumb})
}

func get(h http.Handler, path string, hdr map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest("GET", path, nil)
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestHandlerRange(t *testing.T) {
	rec := get(setup(t), RenderURL(7), map[string]string{"Range": "bytes=0-3"})
	body, _ := io.ReadAll(rec.Body)
	if rec.Code != 206 || string(body) != "0123" {
		t.Fatalf("%d %q", rec.Code, body)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "video/mp4" {
		t.Fatalf("content-type %q", ct)
	}
}

func TestHandlerFileAndThumb(t *testing.T) {
	h := setup(t)
	rec := get(h, FileURL(1, "renders/preview.v1/slides/01.png"), nil)
	if rec.Code != 200 || rec.Body.String() != "PNGDATA" {
		t.Fatalf("file %d %q", rec.Code, rec.Body.String())
	}
	rec = get(h, ThumbURL(1), nil)
	if rec.Code != 200 || rec.Body.String() != "JPG" {
		t.Fatalf("thumb %d", rec.Code)
	}
	if rec := get(h, ThumbURL(2), nil); rec.Code != 404 {
		t.Fatalf("missing thumb %d", rec.Code)
	}
}

func TestHandlerRejectsTraversal(t *testing.T) {
	h := setup(t)
	for _, p := range []string{
		"/media/file/1/..%2F..%2F..%2Fsecret.txt",
		"/media/file/1/../../../secret.txt",
		"/media/file/1/%2Fetc%2Fpasswd",
	} {
		rec := get(h, p, nil)
		if rec.Code != 404 || rec.Body.String() == "SECRET" {
			t.Fatalf("%s → %d %q", p, rec.Code, rec.Body.String())
		}
	}
}

func TestHandlerUnknown(t *testing.T) {
	h := setup(t)
	for _, p := range []string{"/media/render/99", "/media/render/abc", "/media/nope/1", "/media/file/5/a.png"} {
		if rec := get(h, p, nil); rec.Code != 404 {
			t.Fatalf("%s → %d", p, rec.Code)
		}
	}
}
